// Copyright 2022 Democratized Data Foundation
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package memory

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"

	"github.com/sourcenetwork/corekv"
	"github.com/tidwall/btree"
)

// basicTxn implements corekv.Txn
type basicTxn struct {
	ops *btree.BTreeG[dsItem]
	ds  *Datastore
	// Version of the datastore when the transaction was initiated.
	dsVersion *uint64
	readOnly  bool
	discarded bool

	closed  bool
	closeLk sync.RWMutex
}

var _ corekv.Txn = (*basicTxn)(nil)

func (t *basicTxn) getDSVersion() uint64 {
	return atomic.LoadUint64(t.dsVersion)
}

func (t *basicTxn) getTxnVersion() uint64 {
	return t.getDSVersion() + 1
}

// Delete implements ds.Delete.
func (t *basicTxn) Delete(ctx context.Context, key []byte) error {
	t.closeLk.RLock()
	defer t.closeLk.RUnlock()
	if t.closed {
		return corekv.ErrDBClosed
	}

	if t.discarded {
		return corekv.ErrDiscardedTxn
	}
	if t.readOnly {
		return corekv.ErrReadOnlyTxn
	}
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}

	item := t.get(key)
	if item.key == nil || item.isDeleted {
		// if the key doesn't exist of the item is already deleted, this is a no-op.
		return nil
	}

	t.ops.Set(dsItem{key: key, version: t.getTxnVersion(), isDeleted: true})
	return nil
}

func (t *basicTxn) get(key []byte) dsItem {
	result := dsItem{}
	t.ops.Descend(dsItem{key: key, version: t.getTxnVersion()}, func(item dsItem) bool {
		if bytes.Equal(key, item.key) {
			result = item
		}
		// We only care about the last version so we stop iterating right away by returning false.
		return false
	})
	if result.key == nil {
		result = get(t.ds.values, key, t.getDSVersion())
		result.isGet = true
		t.ops.Set(result)
	}
	return result
}

// Get implements ds.Get.
func (t *basicTxn) Get(ctx context.Context, key []byte) ([]byte, error) {
	t.closeLk.RLock()
	defer t.closeLk.RUnlock()
	if t.closed {
		return nil, corekv.ErrDBClosed
	}
	if t.discarded {
		return nil, corekv.ErrDiscardedTxn
	}
	if len(key) == 0 {
		return nil, corekv.ErrEmptyKey
	}

	result := t.get(key)
	if result.key == nil || result.isDeleted {
		return nil, corekv.ErrNotFound
	}
	return result.val, nil
}

// GetSize implements ds.GetSize.
func (t *basicTxn) GetSize(ctx context.Context, key []byte) (size int, err error) {
	t.closeLk.RLock()
	defer t.closeLk.RUnlock()
	if t.closed {
		return 0, corekv.ErrDBClosed
	}

	if t.discarded {
		return 0, corekv.ErrDiscardedTxn
	}
	if len(key) == 0 {
		return 0, corekv.ErrEmptyKey
	}

	result := t.get(key)
	if result.key == nil || result.isDeleted {
		return 0, corekv.ErrNotFound
	}
	return len(result.val), nil
}

// Has implements ds.Has.
func (t *basicTxn) Has(ctx context.Context, key []byte) (exists bool, err error) {
	t.closeLk.RLock()
	defer t.closeLk.RUnlock()
	if t.closed {
		return false, corekv.ErrDBClosed
	}
	if t.discarded {
		return false, corekv.ErrDiscardedTxn
	}
	if len(key) == 0 {
		return false, corekv.ErrEmptyKey
	}

	result := t.get(key)
	if result.key == nil || result.isDeleted {
		return false, nil
	}
	return true, nil
}

// Set implements ds.Set.
func (t *basicTxn) Set(ctx context.Context, key []byte, value []byte) error {
	t.closeLk.RLock()
	defer t.closeLk.RUnlock()
	if t.closed {
		return corekv.ErrDBClosed
	}
	if t.discarded {
		return corekv.ErrDiscardedTxn
	}
	if t.readOnly {
		return corekv.ErrReadOnlyTxn
	}
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	t.ops.Set(dsItem{key: key, version: t.getTxnVersion(), val: value})

	return nil
}

func (t *basicTxn) Iterator(ctx context.Context, opts corekv.IterOptions) (corekv.Iterator, error) {
	t.closeLk.RLock()
	defer t.closeLk.RUnlock()
	if t.closed {
		return nil, corekv.ErrDBClosed
	}

	if opts.Prefix != nil {
		return &txnIterator{
			opts:      opts,
			txnIter:   newPrefixIter(t.ds, t.ops, opts.Prefix, opts.Reverse, t.getTxnVersion()),
			storeIter: newPrefixIter(t.ds, t.ds.values, opts.Prefix, opts.Reverse, t.getDSVersion()),
		}, nil
	}

	return &txnIterator{
		opts:      opts,
		txnIter:   newRangeIter(t.ds, t.ops, opts.Start, opts.End, opts.Reverse, t.getTxnVersion()),
		storeIter: newRangeIter(t.ds, t.ds.values, opts.Start, opts.End, opts.Reverse, t.getDSVersion()),
	}, nil
}

type txnIterator struct {
	opts corekv.IterOptions

	txnIter   *iterator
	storeIter *iterator

	pendingTxnKey   []byte
	pendingStoreKey []byte

	activeIter *iterator
}

var _ corekv.Iterator = (*txnIterator)(nil)

func (iter *txnIterator) Next() (bool, error) {
	txnHasNext := len(iter.pendingTxnKey) != 0
	if !txnHasNext {
		var err error
		txnHasNext, err = iter.txnIter.Next()
		if err != nil {
			return false, err
		}
	}

	storeHasNext := len(iter.pendingStoreKey) != 0
	if !storeHasNext {
		var err error
		storeHasNext, err = iter.storeIter.Next()
		if err != nil {
			return false, err
		}
	}

	return iter.selectIterator(txnHasNext, storeHasNext), nil
}

func (iter *txnIterator) Key() []byte {
	if iter.activeIter == nil {
		return nil
	}
	return iter.activeIter.Key()
}

func (iter *txnIterator) Value() ([]byte, error) {
	if iter.activeIter == nil {
		return nil, nil
	}
	return iter.activeIter.Value()
}

func (iter *txnIterator) Seek(key []byte) (bool, error) {
	txnSeekOk, err := iter.txnIter.Seek(key)
	if err != nil {
		return false, err
	}

	storeSeekOk, err := iter.storeIter.Seek(key)
	if err != nil {
		return false, err
	}

	return iter.selectIterator(txnSeekOk, storeSeekOk), nil
}

// selectIterator selects the iterator to be used by the next `Key` or `Value` function calls
// by mutating the internal `txnIterator` state.
//
// It returns false if there are no values to yield from the iterator.
func (iter *txnIterator) selectIterator(hasPendingTxnKey bool, hasPendingStoreKey bool) bool {
	if !hasPendingTxnKey && !hasPendingStoreKey {
		return false
	}

	if !hasPendingTxnKey {
		iter.activeIter = iter.storeIter
		// Discard the pending key that we are progressing the iterator to.
		// It is no longer pending, and it is very cheap to re-get if the consumer requests it.
		iter.pendingStoreKey = nil
		return true
	}

	if !hasPendingStoreKey {
		iter.activeIter = iter.txnIter
		// Discard the pending key that we are progressing the iterator to.
		// It is no longer pending, and it is very cheap to re-get if the consumer requests it.
		iter.pendingTxnKey = nil
		return true
	}

	iter.pendingTxnKey = iter.txnIter.Key()
	iter.pendingStoreKey = iter.storeIter.Key()

	if (iter.opts.Reverse && gte(iter.pendingTxnKey, iter.pendingStoreKey)) ||
		(!iter.opts.Reverse && lt(iter.pendingTxnKey, iter.pendingStoreKey)) {
		iter.activeIter = iter.txnIter
		// Discard the pending key that we are progressing the iterator to.
		// It is no longer pending, and it is very cheap to re-get if the consumer requests it.
		iter.pendingTxnKey = nil
	} else {
		iter.activeIter = iter.storeIter
		// Discard the pending key that we are progressing the iterator to.
		// It is no longer pending, and it is very cheap to re-get if the consumer requests it.
		iter.pendingStoreKey = nil
	}

	return true
}

func (iter *txnIterator) Reset() {
	iter.activeIter = nil
	iter.pendingTxnKey = nil
	iter.pendingStoreKey = nil
	iter.txnIter.Reset()
	iter.storeIter.Reset()
}

func (iter *txnIterator) Close() error {
	err := iter.txnIter.Close()
	if err != nil {
		return err
	}

	return iter.storeIter.Close()
}

func (t *basicTxn) Close() error {
	return t.Discard()
}

// Discard removes all the operations added to the transaction.
func (t *basicTxn) Discard() error {
	if t.discarded {
		return nil
	}

	t.ops.Clear()
	t.clearInFlightTxn()
	t.discarded = true

	return nil
}

// Commit saves the operations to the underlying datastore.
func (t *basicTxn) Commit() error {
	t.closeLk.RLock()
	defer t.closeLk.RUnlock()
	if t.closed {
		return corekv.ErrDBClosed
	}

	if t.discarded {
		return corekv.ErrDiscardedTxn
	}
	defer t.Discard()

	if !t.readOnly {
		return t.ds.commit(t)
	}

	return nil
}

func (t *basicTxn) checkForConflicts() error {
	if t.getDSVersion() == t.ds.getVersion() {
		return nil
	}
	iter := t.ops.Iter()
	defer iter.Release()
	for iter.Next() {
		expectedItem := get(t.ds.values, iter.Item().key, t.getDSVersion())
		latestItem := get(t.ds.values, iter.Item().key, t.ds.getVersion())
		if latestItem.version != expectedItem.version {
			return corekv.ErrTxnConflict
		}
	}
	return nil
}

func (t *basicTxn) clearInFlightTxn() {
	t.ds.inFlightTxn.Delete(
		dsTxn{
			dsVersion:  t.getDSVersion(),
			txnVersion: t.getTxnVersion(),
		},
	)
	t.ds.clearOldInFlightTxn()
}

func (t *basicTxn) close() {
	t.closeLk.Lock()
	defer t.closeLk.Unlock()
	t.closed = true
}

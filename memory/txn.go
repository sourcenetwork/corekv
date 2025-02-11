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

// var _ ds.Txn = (*basicTxn)(nil)

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
		return ErrClosed
	}

	if t.discarded {
		return ErrTxnDiscarded
	}
	if t.readOnly {
		return ErrReadOnlyTxn
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
		result = t.ds.get(key, t.getDSVersion())
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
		return nil, ErrClosed
	}
	if t.discarded {
		return nil, ErrTxnDiscarded
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
		return 0, ErrClosed
	}

	if t.discarded {
		return 0, ErrTxnDiscarded
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
		return false, ErrClosed
	}
	if t.discarded {
		return false, ErrTxnDiscarded
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
		return ErrClosed
	}
	if t.discarded {
		return ErrTxnDiscarded
	}
	if t.readOnly {
		return ErrReadOnlyTxn
	}
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	t.ops.Set(dsItem{key: key, version: t.getTxnVersion(), val: value})

	return nil
}

// Query implements ds.Query.
// func (t *basicTxn) Query(ctx context.Context, q dsq.Query) (dsq.Results, error) {
// 	t.closeLk.RLock()
// 	defer t.closeLk.RUnlock()
// 	if t.closed {
// 		return nil, ErrClosed
// 	}

// 	if t.discarded {
// 		return nil, ErrTxnDiscarded
// 	}
// 	// best effort allocation
// 	re := make([]dsq.Entry, 0, t.ds.values.Height()+t.ops.Height())
// 	iter := t.ds.values.Iter()
// 	iterOps := t.ops.Iter()
// 	iterOpsHasValue := iterOps.Next()
// 	// iterate over the underlying store and ensure that ops with keys smaller than or equal to
// 	// the key of the underlying store are added with priority.
// 	for iter.Next() {
// 		// fast forward to last inserted version
// 		item := iter.Item()
// 		for iter.Next() {
// 			if item.key == iter.Item().key {
// 				item = iter.Item()
// 				continue
// 			}
// 			iter.Prev()
// 			break
// 		}

// 		// handle all ops that come before the current item's key or equal to the current item's key
// 		for iterOpsHasValue && iterOps.Item().key <= item.key {
// 			if iterOps.Item().key == item.key {
// 				item = iterOps.Item()
// 			} else if !iterOps.Item().isDeleted && !iterOps.Item().isGet {
// 				re = append(re, setEntry(iterOps.Item().key, iterOps.Item().val, q))
// 			}
// 			iterOpsHasValue = iterOps.Next()
// 		}

// 		if item.isDeleted {
// 			continue
// 		}

// 		re = append(re, setEntry(item.key, item.val, q))
// 	}

// 	iter.Release()

// 	// add the remaining ops
// 	for iterOpsHasValue {
// 		if !iterOps.Item().isDeleted && !iterOps.Item().isGet {
// 			re = append(re, setEntry(iterOps.Item().key, iterOps.Item().val, q))
// 		}
// 		iterOpsHasValue = iterOps.Next()
// 	}

// 	iterOps.Release()

// 	r := dsq.ResultsWithEntries(q, re)
// 	r = dsq.NaiveQueryApply(q, r)
// 	return r, nil
// }

// func setEntry(key string, value []byte, q dsq.Query) dsq.Entry {
// 	e := dsq.Entry{
// 		Key:  key,
// 		Size: len(value),
// 	}
// 	if !q.KeysOnly {
// 		e.Value = value
// 	}
// 	return e
// }

func (t *basicTxn) Close() error {
	t.Discard()
	return nil
}

// Discard removes all the operations added to the transaction.
func (t *basicTxn) Discard() {
	if t.discarded {
		return
	}
	t.ops.Clear()
	t.clearInFlightTxn()
	t.discarded = true
}

// Commit saves the operations to the underlying datastore.
func (t *basicTxn) Commit() error {
	t.closeLk.RLock()
	defer t.closeLk.RUnlock()
	if t.closed {
		return ErrClosed
	}

	if t.discarded {
		return ErrTxnDiscarded
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
		expectedItem := t.ds.get(iter.Item().key, t.getDSVersion())
		latestItem := t.ds.get(iter.Item().key, t.ds.getVersion())
		if latestItem.version != expectedItem.version {
			return ErrTxnConflict
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

package memory

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sourcenetwork/corekv"

	"github.com/tidwall/btree"
)

type dsTxn struct {
	dsVersion  uint64
	txnVersion uint64
	expiresAt  time.Time
	txn        *basicTxn
}

func byDSVersion(a, b dsTxn) bool {
	switch {
	case a.dsVersion < b.dsVersion:
		return true
	case a.dsVersion == b.dsVersion && a.txnVersion < b.txnVersion:
		return true
	default:
		return false
	}
}

type dsItem struct {
	key       []byte
	version   uint64
	val       []byte
	isDeleted bool
	isGet     bool
}

func byKeys(a, b dsItem) bool {
	// fmt.Println("compare:", string(a.key), string(b.key))
	cmp := bytes.Compare(a.key, b.key)
	if cmp == -1 { // a < b
		return true
	} else if cmp == 1 { // a > b
		return false
	}
	return byVersion(a, b)
}

func byVersion(a, b dsItem) bool {
	return a.version < b.version
}

// Datastore uses a btree for internal storage.
type Datastore struct {
	// Latest committed version.
	version     *uint64
	values      *btree.BTreeG[dsItem]
	inFlightTxn *btree.BTreeG[dsTxn]

	closing  chan struct{}
	closed   bool
	closeLk  sync.RWMutex
	commitLk sync.Mutex
}

var _ corekv.Store = (*Datastore)(nil)

// var _ corekv.Batchable = (*Datastore)(nil)
// var _ corekv.TxnStore = (*Datastore)(nil)

// NewDatastore constructs an empty Datastore.
func NewDatastore(ctx context.Context) *Datastore {
	v := uint64(0)
	d := &Datastore{
		version:     &v,
		values:      btree.NewBTreeG(byKeys),
		inFlightTxn: btree.NewBTreeG(byDSVersion),
		closing:     make(chan struct{}),
	}
	go d.purgeOldVersions(ctx)
	go d.handleContextDone(ctx)
	return d
}

func (d *Datastore) getVersion() uint64 {
	return atomic.LoadUint64(d.version)
}

func (d *Datastore) nextVersion() uint64 {
	return atomic.AddUint64(d.version, 1)
}

// Batch return a corekv.Batch datastore based on Datastore.
// func (d *Datastore) Batch(ctx context.Context) (corekv.Batch, error) {
// 	return d.newBasicBatch(), nil
// }

// // newBasicBatch returns a corekv.Batch datastore.
// func (d *Datastore) newBasicBatch() corekv.Batch {
// 	return &basicBatch{
// 		ops: make(map[[]byte]op),
// 		ds:  d,
// 	}
// }

func (d *Datastore) Close() error {
	d.closeLk.Lock()
	defer d.closeLk.Unlock()
	if d.closed {
		return nil
	}

	d.closed = true
	close(d.closing)

	iter := d.inFlightTxn.Iter()

	for iter.Next() {
		iter.Item().txn.close()
	}
	iter.Release()

	return nil
}

// Delete implements corekv.Store
func (d *Datastore) Delete(ctx context.Context, key []byte) error {
	d.closeLk.RLock()
	defer d.closeLk.RUnlock()
	if d.closed {
		return ErrClosed
	}
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	tx := d.newTransaction(false)

	err := tx.Delete(ctx, key)
	cErr := tx.Commit(ctx)

	return errors.Join(err, cErr)
}

func (d *Datastore) get(key []byte, version uint64) dsItem {
	result := dsItem{}
	d.values.Descend(dsItem{key: key, version: version}, func(item dsItem) bool {
		if bytes.Equal(key, item.key) {
			result = item
		}
		// We only care about the last version so we stop iterating right away by returning false.
		return false
	})
	return result
}

// Get implements corekv.Store.
func (d *Datastore) Get(ctx context.Context, key []byte) (value []byte, err error) {
	d.closeLk.RLock()
	defer d.closeLk.RUnlock()
	if d.closed {
		return nil, ErrClosed
	}
	if len(key) == 0 {
		return nil, corekv.ErrEmptyKey
	}
	result := d.get(key, d.getVersion())
	if result.key == nil || result.isDeleted {
		return nil, corekv.ErrNotFound
	}

	return result.val, nil
}

// Has implements corekv.Store.
func (d *Datastore) Has(ctx context.Context, key []byte) (exists bool, err error) {
	d.closeLk.RLock()
	defer d.closeLk.RUnlock()
	if d.closed {
		return false, ErrClosed
	}
	if len(key) == 0 {
		return false, corekv.ErrEmptyKey
	}
	result := d.get(key, d.getVersion())
	return result.key != nil && !result.isDeleted, nil
}

// NewTransaction return a corekv.Txn datastore based on Datastore.
// func (d *Datastore) NewTxn(ctx context.Context, readOnly bool) (corekv.Txn, error) {
// 	d.closeLk.RLock()
// 	defer d.closeLk.RUnlock()
// 	if d.closed {
// 		return nil, ErrClosed
// 	}
// 	return d.newTransaction(readOnly), nil
// }

// newTransaction returns a corekv.Txn datastore.
//
// isInternal should be set to true if this transaction is created from within the
// datastore and is already protected by stuff like locks.  Failure to correctly set
// this to true may result in deadlocks.  Failure to correctly set it to false may lead
// to other concurrency issues.
func (d *Datastore) newTransaction(readOnly bool) *basicTxn {
	v := d.getVersion()
	txn := &basicTxn{
		ops:       btree.NewBTreeG(byKeys),
		ds:        d,
		readOnly:  readOnly,
		dsVersion: &v,
	}

	d.inFlightTxn.Set(dsTxn{v, v + 1, time.Now().Add(1 * time.Hour), txn})
	return txn
}

// Put implements corekv.Store.
func (d *Datastore) Set(ctx context.Context, key []byte, value []byte) (err error) {
	d.closeLk.RLock()
	defer d.closeLk.RUnlock()
	if d.closed {
		return ErrClosed
	}
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	tx := d.newTransaction(false)

	err = tx.Set(ctx, key, value)
	if err != nil {
		tx.Discard(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (d *Datastore) Iterator(ctx context.Context, opts corekv.IterOptions) corekv.Iterator {
	if opts.Prefix != nil {
		return newPrefixIter(d, opts.Prefix, opts.Reverse, d.getVersion())
	}
	return newRangeIter(d, opts.Start, opts.End, opts.Reverse, d.getVersion())
}

// purgeOldVersions will execute the purge once a day or when explicitly requested.
func (d *Datastore) purgeOldVersions(ctx context.Context) {
	dbStartTime := time.Now()
	nextCompression := time.Date(dbStartTime.Year(), dbStartTime.Month(), dbStartTime.Day()+1,
		0, 0, 0, 0, dbStartTime.Location())

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.closing:
			return
		case <-time.After(time.Until(nextCompression)):
			d.executePurge()
			now := time.Now()
			nextCompression = time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		}
	}
}

func (d *Datastore) executePurge() {
	// purging bellow this version
	v := d.getVersion()
	if dsTxn, hasMin := d.inFlightTxn.Min(); hasMin {
		v = dsTxn.dsVersion
	}

	for {
		itemsToDelete := []dsItem{}
		iter := d.values.Iter()
		iter.Next()
		item := iter.Item()
		// fast forward to last inserted version and delete versions before it
		total := 0
		for iter.Next() {
			if iter.Item().version > v {
				continue
			}
			if bytes.Equal(item.key, iter.Item().key) {
				itemsToDelete = append(itemsToDelete, item)
				total++
			}
			item = iter.Item()
			// we don't want to delete more than 1000 items at a time
			// to prevent loading too much into memory
			if total >= 1000 {
				break
			}
		}
		iter.Release()

		if total == 0 {
			return
		}

		for _, i := range itemsToDelete {
			d.values.Delete(i)
		}
	}
}

func (d *Datastore) handleContextDone(ctx context.Context) {
	<-ctx.Done()
	d.Close()
}

// commit commits the given transaction to the datastore.
//
// WARNING: This is a notable bottleneck, as commits can only be commited one at a time (handled internally).
// This is to ensure correct, threadsafe, mututation of the datastore version.
func (d *Datastore) commit(t *basicTxn) error {
	d.commitLk.Lock()
	defer d.commitLk.Unlock()

	// The commitLk scope must include checkForConflicts, and it must be a write lock. The datastore version
	// cannot be allowed to change between here and the release of the iterator, else the check for conflicts
	// will be stale and potentially out of date.
	err := t.checkForConflicts()
	if err != nil {
		return err
	}

	iter := t.ops.Iter()
	v := t.ds.nextVersion()
	for iter.Next() {
		if iter.Item().isGet {
			continue
		}
		item := iter.Item()
		item.version = v
		t.ds.values.Set(item)
	}
	iter.Release()
	return nil
}

func (d *Datastore) clearOldInFlightTxn() {
	if d.inFlightTxn.Height() == 0 {
		return
	}

	now := time.Now()
	for {
		itemsToDelete := []dsTxn{}
		iter := d.inFlightTxn.Iter()

		total := 0
		for iter.Next() {
			if now.After(iter.Item().expiresAt) {
				itemsToDelete = append(itemsToDelete, iter.Item())
				total++
			}
		}
		iter.Release()

		if total == 0 {
			return
		}

		for _, i := range itemsToDelete {
			d.inFlightTxn.Delete(i)
		}
	}
}

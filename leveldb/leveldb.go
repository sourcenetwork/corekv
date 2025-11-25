// This package implements a datastore backed by goleveldb. It has some important limitations to consider around transactions:
//
// Only one transaction can be opened at a time. Subsequent call to Write and OpenTransaction will be blocked until in-flight transaction is committed or discarded. The returned transaction handle is safe for concurrent use.
//
// Transaction is expensive and can overwhelm compaction, especially if transaction size is small. Use with caution.
//
// The transaction must be closed once done, either by committing or discarding the transaction. Closing the DB will discard open transaction.
package leveldb

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/sourcenetwork/goleveldb/leveldb"
	"github.com/sourcenetwork/goleveldb/leveldb/opt"
	"github.com/sourcenetwork/goleveldb/leveldb/util"

	"github.com/sourcenetwork/corekv"
)

type Datastore struct {
	db     *leveldb.DB
	closed atomic.Bool
}

var _ corekv.TxnStore = (*Datastore)(nil)

func NewDatastore(path string, opts *opt.Options) (*Datastore, error) {
	db, err := leveldb.OpenFile(path, opts)
	if err != nil {
		return nil, err
	}
	return &Datastore{db: db}, nil
}

func NewDatastoreFrom(db *leveldb.DB) *Datastore {
	return &Datastore{db: db}
}

func (l *Datastore) Get(ctx context.Context, key []byte) ([]byte, error) {
	value, err := l.db.Get(key, nil)
	if err != nil {
		return nil, levelErrToKVErr(err)
	}
	return value, nil
}

func (l *Datastore) Has(ctx context.Context, key []byte) (bool, error) {
	exists, err := l.db.Has(key, nil)
	if err != nil {
		return false, levelErrToKVErr(err)
	}
	return exists, nil
}

func (l *Datastore) Set(ctx context.Context, key []byte, value []byte) error {
	err := l.db.Put(key, value, nil)
	return levelErrToKVErr(err)

}

func (l *Datastore) Delete(ctx context.Context, key []byte) error {
	err := l.db.Delete(key, nil)
	return levelErrToKVErr(err)
}

func (l *Datastore) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return nil
	}
	err := l.db.Close()
	return levelErrToKVErr(err)
}

func (l *Datastore) Iterator(ctx context.Context, iterOpts corekv.IterOptions) (corekv.Iterator, error) {
	if l.closed.Load() {
		return nil, corekv.ErrDBClosed
	}
	var slice *util.Range
	if iterOpts.Prefix != nil {
		slice = util.BytesPrefix(iterOpts.Prefix)
	} else {
		slice = &util.Range{Start: iterOpts.Start, Limit: iterOpts.End}
	}
	return &iterator{
		d:        l,
		i:        l.db.NewIterator(slice, nil),
		start:    slice.Start,
		end:      slice.Limit,
		reverse:  iterOpts.Reverse,
		keysOnly: iterOpts.KeysOnly,
		reset:    true,
	}, nil
}

func (l *Datastore) NewTxn(_ bool) corekv.Txn {
	// This error is only returned when the db is closed.
	// We store it for later and return it from all functions
	// to satisfy the transaction interface.
	t, err := l.db.OpenTransaction()
	return &lTxn{
		t:   t,
		d:   l,
		err: err,
	}
}

type lTxn struct {
	t   *leveldb.Transaction
	d   *Datastore
	err error
}

func (txn *lTxn) Get(ctx context.Context, key []byte) ([]byte, error) {
	if txn.err != nil {
		return nil, levelErrToKVErr(txn.err)
	}
	if txn.d.closed.Load() {
		return nil, corekv.ErrDBClosed
	}
	value, err := txn.t.Get(key, nil)
	if err != nil {
		return nil, levelErrToKVErr(err)
	}
	return value, nil
}

func (txn *lTxn) Has(ctx context.Context, key []byte) (bool, error) {
	if txn.err != nil {
		return false, levelErrToKVErr(txn.err)
	}
	if txn.d.closed.Load() {
		return false, corekv.ErrDBClosed
	}
	exists, err := txn.t.Has(key, nil)
	if err != nil {
		return false, levelErrToKVErr(err)
	}
	return exists, nil
}

func (txn *lTxn) Iterator(ctx context.Context, iterOpts corekv.IterOptions) (corekv.Iterator, error) {
	if txn.err != nil {
		return nil, levelErrToKVErr(txn.err)
	}
	if txn.d.closed.Load() {
		return nil, corekv.ErrDBClosed
	}
	var slice *util.Range
	if iterOpts.Prefix != nil {
		slice = util.BytesPrefix(iterOpts.Prefix)
	} else {
		slice = &util.Range{Start: iterOpts.Start, Limit: iterOpts.End}
	}
	return &iterator{
		d:        txn.d,
		i:        txn.t.NewIterator(slice, nil),
		start:    slice.Start,
		end:      slice.Limit,
		reverse:  iterOpts.Reverse,
		keysOnly: iterOpts.KeysOnly,
		reset:    true,
	}, nil
}

func (txn *lTxn) Set(ctx context.Context, key []byte, value []byte) error {
	if txn.err != nil {
		return levelErrToKVErr(txn.err)
	}
	if txn.d.closed.Load() {
		return corekv.ErrDBClosed
	}
	err := txn.t.Put(key, value, nil)
	return levelErrToKVErr(err)
}

func (txn *lTxn) Delete(ctx context.Context, key []byte) error {
	if txn.err != nil {
		return levelErrToKVErr(txn.err)
	}
	if txn.d.closed.Load() {
		return corekv.ErrDBClosed
	}
	err := txn.t.Delete(key, nil)
	return levelErrToKVErr(err)
}

func (txn *lTxn) Commit() error {
	if txn.err != nil {
		return levelErrToKVErr(txn.err)
	}
	if txn.d.closed.Load() {
		return corekv.ErrDBClosed
	}
	err := txn.t.Commit()
	return levelErrToKVErr(err)
}

func (txn *lTxn) Discard() {
	txn.t.Discard()
}

var levelErrToKVErrMap = map[error]error{
	leveldb.ErrNotFound: corekv.ErrNotFound,
	leveldb.ErrClosed:   corekv.ErrDBClosed,
}

func levelErrToKVErr(err error) error {
	if err == nil {
		return nil
	}
	mappedErr, ok := levelErrToKVErrMap[err]
	if ok {
		return mappedErr
	}
	for k, v := range levelErrToKVErrMap {
		if errors.Is(k, err) {
			return v
		}
	}
	return err
}

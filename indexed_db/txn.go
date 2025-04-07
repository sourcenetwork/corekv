//go:build js

package indexed_db

import (
	"context"
	"syscall/js"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/goji"
	"github.com/sourcenetwork/goji/indexed_db"
)

var _ corekv.Txn = (*txn)(nil)

type txn struct {
	complete    bool
	transaction indexed_db.TransactionValue
}

func newTxn(db indexed_db.DatabaseValue, readOnly bool) corekv.Txn {
	var transaction indexed_db.TransactionValue
	if readOnly {
		transaction = db.Transaction(objectStoreName, indexed_db.TransactionModeReadOnly)
	} else {
		transaction = db.Transaction(objectStoreName, indexed_db.TransactionModeReadWrite)
	}
	t := &txn{transaction: transaction}

	var abortCallback js.Func
	var completeCallback js.Func
	abortCallback = goji.EventListener(func(event goji.EventValue) {
		t.complete = true
		abortCallback.Release()
		completeCallback.Release()
	})
	completeCallback = goji.EventListener(func(event goji.EventValue) {
		t.complete = true
		abortCallback.Release()
		completeCallback.Release()
	})

	goji.EventTargetValue(transaction).AddEventListener("abort", abortCallback.Value)
	goji.EventTargetValue(transaction).AddEventListener("complete", completeCallback.Value)
	return t
}

func (t *txn) Set(ctx context.Context, key, value []byte) error {
	if js.Value(t.transaction).IsUndefined() {
		return corekv.ErrDBClosed
	}
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	var val js.Value
	if value == nil {
		val = js.Null()
	} else {
		val = toBytesValue(value)
	}
	store := t.transaction.ObjectStore(objectStoreName)
	keyVal := toBytesValue(key)
	_, err := indexed_db.Await(store.Put(val, keyVal))
	return err
}

func (t *txn) Delete(ctx context.Context, key []byte) error {
	if js.Value(t.transaction).IsUndefined() {
		return corekv.ErrDBClosed
	}
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	store := t.transaction.ObjectStore(objectStoreName)
	keyVal := toBytesValue(key)
	_, err := indexed_db.Await(store.Delete(keyVal))
	return err
}

func (t *txn) Get(ctx context.Context, key []byte) ([]byte, error) {
	if js.Value(t.transaction).IsUndefined() {
		return []byte(nil), corekv.ErrDBClosed
	}
	if len(key) == 0 {
		return []byte(nil), corekv.ErrEmptyKey
	}
	store := t.transaction.ObjectStore(objectStoreName)
	keyVal := toBytesValue(key)
	val, err := indexed_db.Await(store.Get(keyVal))
	if err != nil {
		return []byte(nil), err
	}
	if val.IsUndefined() {
		return []byte(nil), corekv.ErrNotFound
	}
	return fromBytesValue(val), nil
}

func (t *txn) Has(ctx context.Context, key []byte) (bool, error) {
	if js.Value(t.transaction).IsUndefined() {
		return false, corekv.ErrDBClosed
	}
	if len(key) == 0 {
		return false, corekv.ErrEmptyKey
	}
	store := t.transaction.ObjectStore(objectStoreName)
	keyVal := toBytesValue(key)
	req, err := indexed_db.Await(store.OpenCursor(keyVal))
	if err != nil {
		return false, nil
	}
	return js.Value(req).Truthy(), nil
}

func (t *txn) Iterator(ctx context.Context, opts corekv.IterOptions) (corekv.Iterator, error) {
	if js.Value(t.transaction).IsUndefined() {
		return nil, corekv.ErrDBClosed
	}
	store := t.transaction.ObjectStore(objectStoreName)
	return newIterator(store, opts), nil
}

func (t *txn) Commit() error {
	if t.complete {
		return nil
	}
	if js.Value(t.transaction).IsUndefined() {
		return corekv.ErrDBClosed
	}
	t.complete = true
	t.transaction.Commit()
	return nil
}

func (t *txn) Discard() {
	if t.complete || js.Value(t.transaction).IsUndefined() {
		return
	}
	t.complete = true
	t.transaction.Abort()
}

func (t *txn) Close() error {
	t.Discard()
	return nil
}

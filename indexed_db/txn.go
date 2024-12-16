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
	transaction indexed_db.TransactionValue
}

func (t *txn) Set(ctx context.Context, key, value []byte) error {
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
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	store := t.transaction.ObjectStore(objectStoreName)
	keyVal := toBytesValue(key)
	_, err := indexed_db.Await(store.Delete(keyVal))
	return err
}

func (t *txn) Get(ctx context.Context, key []byte) ([]byte, error) {
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
	if len(key) == 0 {
		return false, corekv.ErrEmptyKey
	}
	store := t.transaction.ObjectStore(objectStoreName)
	keyVal := js.Value(goji.Uint8ArrayFromBytes(key))
	req, err := indexed_db.Await(store.OpenCursor(keyVal))
	if err != nil {
		return false, nil
	}
	return js.Value(req).Truthy(), nil
}

func (t *txn) Iterator(ctx context.Context, opts corekv.IterOptions) corekv.Iterator {
	store := t.transaction.ObjectStore(objectStoreName)
	return newIterator(store, opts)
}

func (t *txn) Commit(ctx context.Context) error {
	t.transaction.Commit()
	return nil
}

func (t *txn) Discard(ctx context.Context) error {
	t.transaction.Abort()
	return nil
}

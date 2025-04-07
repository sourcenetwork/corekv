//go:build js

package indexed_db

import (
	"context"

	"github.com/sourcenetwork/corekv"

	"github.com/sourcenetwork/goji"
	"github.com/sourcenetwork/goji/indexed_db"
)

const (
	// databaseVersion is the current database version
	databaseVersion = uint(1)
	// objectStoreName is the name of the object store used to store key values
	objectStoreName = "values"
)

var _ corekv.TxnStore = (*Datastore)(nil)

type Datastore struct {
	db     indexed_db.DatabaseValue
	closed bool
}

// NewDatastore returns a new indexed db backed datastore.
func NewDatastore(name string) (*Datastore, error) {
	var req indexed_db.RequestValue[indexed_db.DatabaseValue]
	upgradeNeeded := goji.EventListener(func(event goji.EventValue) {
		req.Result().CreateObjectStore(objectStoreName)
	})
	defer upgradeNeeded.Release()

	req = indexed_db.Open(name, databaseVersion)
	req.EventTarget().AddEventListener(indexed_db.UpgradeNeededEvent, upgradeNeeded.Value)

	db, err := indexed_db.Await(req)
	if err != nil {
		return nil, err
	}
	return &Datastore{db: db}, nil
}

func (d *Datastore) NewTxn(readOnly bool) corekv.Txn {
	// return an empty transaction if already closed
	if d.closed {
		return &txn{}
	}
	return newTxn(d.db, readOnly)
}

func (d *Datastore) Set(ctx context.Context, key, value []byte) error {
	if d.closed {
		return corekv.ErrDBClosed
	}

	txn := d.NewTxn(false)
	defer txn.Discard()

	err := txn.Set(ctx, key, value)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (d *Datastore) Delete(ctx context.Context, key []byte) error {
	if d.closed {
		return corekv.ErrDBClosed
	}

	txn := d.NewTxn(false)
	defer txn.Discard()

	err := txn.Delete(ctx, key)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (d *Datastore) Get(ctx context.Context, key []byte) ([]byte, error) {
	if d.closed {
		return nil, corekv.ErrDBClosed
	}

	txn := d.NewTxn(true)
	defer txn.Discard()

	val, err := txn.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	return val, txn.Commit()
}

func (d *Datastore) Has(ctx context.Context, key []byte) (bool, error) {
	if d.closed {
		return false, corekv.ErrDBClosed
	}

	txn := d.NewTxn(true)
	defer txn.Discard()

	val, err := txn.Has(ctx, key)
	if err != nil {
		return false, err
	}
	return val, txn.Commit()
}

func (d *Datastore) Iterator(ctx context.Context, opts corekv.IterOptions) (corekv.Iterator, error) {
	txn := d.NewTxn(true)
	return txn.Iterator(ctx, opts)
}

func (d *Datastore) Close() error {
	if d.closed {
		return nil
	}
	d.closed = true
	d.db.Close()
	return nil
}

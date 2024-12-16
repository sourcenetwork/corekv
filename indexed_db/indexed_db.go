//go:build js

package indexed_db

import (
	"context"
	"errors"

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
	db indexed_db.DatabaseValue
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
	return &Datastore{db}, nil
}

func (d *Datastore) NewTxn(readOnly bool) corekv.Txn {
	var transaction indexed_db.TransactionValue
	if readOnly {
		transaction = d.db.Transaction(objectStoreName, indexed_db.TransactionModeReadOnly)
	} else {
		transaction = d.db.Transaction(objectStoreName, indexed_db.TransactionModeReadWrite)
	}
	return &txn{transaction}
}

func (d *Datastore) Set(ctx context.Context, key, value []byte) error {
	txn := d.NewTxn(false)
	err := txn.Set(ctx, key, value)
	if err != nil {
		return errors.Join(err, txn.Discard(ctx))
	}
	return txn.Commit(ctx)
}

func (d *Datastore) Delete(ctx context.Context, key []byte) error {
	txn := d.NewTxn(false)
	err := txn.Delete(ctx, key)
	if err != nil {
		return errors.Join(err, txn.Discard(ctx))
	}
	return txn.Commit(ctx)
}

func (d *Datastore) Get(ctx context.Context, key []byte) ([]byte, error) {
	txn := d.NewTxn(true)
	val, err := txn.Get(ctx, key)
	if err != nil {
		return nil, errors.Join(err, txn.Discard(ctx))
	}
	return val, txn.Commit(ctx)
}

func (d *Datastore) Has(ctx context.Context, key []byte) (bool, error) {
	txn := d.NewTxn(true)
	val, err := txn.Has(ctx, key)
	if err != nil {
		return false, errors.Join(err, txn.Discard(ctx))
	}
	return val, txn.Commit(ctx)
}

func (d *Datastore) Iterator(ctx context.Context, opts corekv.IterOptions) corekv.Iterator {
	txn := d.NewTxn(true)
	return txn.Iterator(ctx, opts)
}

func (d *Datastore) Close() {
	d.db.Close()
}

package namespace

import (
	"github.com/sourcenetwork/corekv"
)

type Txn struct {
	Datastore

	txn corekv.Txn
}

var _ corekv.Txn = (*Txn)(nil)

// WrapTxn lets you namespace a transaction with a given prefix.
func WrapTxn(txn corekv.Txn, prefix []byte) *Txn {
	return &Txn{
		Datastore: *Wrap(txn, prefix),
		txn:       txn,
	}
}

func (ntxn *Txn) Commit() error {
	return ntxn.txn.Commit()
}

func (ntxn *Txn) Discard() {
	ntxn.txn.Discard()
}

type TxnStore struct {
	Datastore

	store corekv.TxnStore
}

var _ corekv.TxnStore = (*TxnStore)(nil)

func (ntxn *TxnStore) NewTxn(readonly bool) corekv.Txn {
	txn := ntxn.store.NewTxn(readonly)
	return WrapTxn(txn, ntxn.namespace)
}

func (ntxn *TxnStore) Close() error {
	return ntxn.store.Close()
}

// WrapTS lets you namespace a transaction store with a given prefix.
func WrapTS(store corekv.TxnStore, prefix []byte) *TxnStore {
	return &TxnStore{
		Datastore: *Wrap(store, prefix),
		store:     store,
	}
}

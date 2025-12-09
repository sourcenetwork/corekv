package namespace

import (
	"github.com/sourcenetwork/corekv"
)

type Txn struct {
	Datastore
	corekv.TxnCore
}

var _ corekv.Txn = (*Txn)(nil)

// WrapTxn lets you namespace a transaction with a given prefix.
func WrapTxn(txn corekv.Txn, prefix []byte) *Txn {
	return &Txn{
		Datastore: *Wrap(txn, prefix),
		TxnCore:   txn,
	}
}

type TxnStore struct {
	Datastore

	store corekv.TxnReaderWriter
}

var _ corekv.TxnReaderWriter = (*TxnStore)(nil)

func (ntxn *TxnStore) NewTxn(readonly bool) corekv.Txn {
	txn := ntxn.store.NewTxn(readonly)
	return WrapTxn(txn, ntxn.namespace)
}

// WrapTS lets you namespace a transaction store with a given prefix.
func WrapTS(store corekv.TxnReaderWriter, prefix []byte) *TxnStore {
	return &TxnStore{
		Datastore: *Wrap(store, prefix),
		store:     store,
	}
}

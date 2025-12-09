package chunk

import (
	"context"

	"github.com/sourcenetwork/corekv"
)

type Txn struct {
	*Chunkstore

	corekv.TxnCore
}

var _ corekv.Txn = (*Txn)(nil)

// NewTxn wraps a given corekv transaction with a Chunkstore.
//
// The keyLen of the store will be determined dynamically, as per [New].
func NewTxn(ctx context.Context, txn corekv.Txn, chunkSize int) (*Txn, error) {
	chunkstore, err := New(ctx, txn, chunkSize)
	if err != nil {
		return nil, err
	}

	return &Txn{
		Chunkstore: chunkstore,
		TxnCore:    txn,
	}, nil
}

type TxnStore struct {
	*Chunkstore

	store corekv.TxnReaderWriter
}

var _ corekv.TxnReaderWriter = (*TxnStore)(nil)

// NewTxn returns a new transaction.
func (ntxn *TxnStore) NewTxn(readonly bool) corekv.Txn {
	txn := ntxn.store.NewTxn(readonly)

	return &Txn{
		Chunkstore: NewSized(txn, ntxn.chunkSize, ntxn.keyLen),
		TxnCore:    txn,
	}
}

// NewTxn wraps a given corekv TxnReaderWriter with a Chunkstore.
//
// The keyLen of the store will be determined dynamically, as per [New].
func NewTS(ctx context.Context, store corekv.TxnReaderWriter, chunkSize int) (*TxnStore, error) {
	chunkstore, err := New(ctx, store, chunkSize)
	if err != nil {
		return nil, err
	}

	return &TxnStore{
		Chunkstore: chunkstore,
		store:      store,
	}, err
}

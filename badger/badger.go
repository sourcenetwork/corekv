package badger

import (
	"context"
	"errors"
	"strings"

	"github.com/dgraph-io/badger/v4"

	"github.com/sourcenetwork/corekv"
)

type bDB struct {
	db *badger.DB
}

func NewDatastore(path string, opts badger.Options) (corekv.Store, error) {
	return newDatastore(path, opts)
}

func newDatastore(path string, opts badger.Options) (*bDB, error) {
	opts.Dir = path
	opts.ValueDir = path
	opts.Logger = nil // badger is too chatty
	store, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return newDatastoreFrom(store), nil
}

func NewDatastoreFrom(db *badger.DB) corekv.Store {
	return newDatastoreFrom(db)
}

func newDatastoreFrom(db *badger.DB) *bDB {
	return &bDB{
		db: db,
	}
}

func (b *bDB) Get(ctx context.Context, key []byte) ([]byte, error) {
	txn := b.newTxn(true)

	result, err := txn.Get(ctx, key)
	dErr := txn.Discard()

	return result, errors.Join(err, dErr)
}

func (b *bDB) Has(ctx context.Context, key []byte) (bool, error) {
	txn := b.newTxn(true)

	result, err := txn.Has(ctx, key)
	dErr := txn.Discard()

	return result, errors.Join(err, dErr)
}

func (b *bDB) Set(ctx context.Context, key []byte, value []byte) error {
	txn := b.newTxn(false)

	err := txn.Set(ctx, key, value)
	if err != nil {
		dErr := txn.Discard()
		return errors.Join(err, dErr)
	}

	return txn.Commit()
}

func (b *bDB) Delete(ctx context.Context, key []byte) error {
	txn := b.newTxn(false)

	err := txn.Delete(ctx, key)
	if err != nil {
		dErr := txn.Discard()
		return errors.Join(err, dErr)
	}

	return txn.Commit()
}

func (b *bDB) Close() error {
	return b.db.Close()
}

func (b *bDB) Iterator(ctx context.Context, iterOpts corekv.IterOptions) corekv.Iterator {
	txn := b.newTxn(true)
	it := txn.iterator(iterOpts)

	// closer for discarding implicit txn
	// so that the txn is discarded when the
	// iterator is closed
	it.withCloser(func() error {
		return txn.Discard()
	})
	return it

}

func (b *bDB) NewTxn(readonly bool) corekv.Txn {
	return b.newTxn(readonly)
}

func (b *bDB) newTxn(readonly bool) *bTxn {
	return &bTxn{b.db.NewTransaction(!readonly)}
}

type bTxn struct {
	t *badger.Txn
}

func (txn *bTxn) Get(ctx context.Context, key []byte) ([]byte, error) {
	item, err := txn.t.Get(key)
	if err != nil {
		return nil, badgerErrToKVErr(err)
	}

	return item.ValueCopy(nil)
}

func (txn *bTxn) Has(ctx context.Context, key []byte) (bool, error) {
	_, err := txn.t.Get(key)
	switch {
	case errors.Is(err, badger.ErrKeyNotFound):
		return false, nil
	case err == nil:
		return true, nil
	default:
		return false, badgerErrToKVErr(err)
	}
}

func (txn *bTxn) Iterator(ctx context.Context, iterOpts corekv.IterOptions) corekv.Iterator {
	return txn.iterator(iterOpts)
}

func (txn *bTxn) iterator(iopts corekv.IterOptions) iteratorCloser {
	if iopts.Prefix != nil {
		return newPrefixIterator(txn, iopts.Prefix, iopts.Reverse, iopts.KeysOnly)
	}
	return newRangeIterator(txn, iopts.Start, iopts.End, iopts.Reverse, iopts.KeysOnly)
}

func (txn *bTxn) Set(ctx context.Context, key []byte, value []byte) error {
	err := txn.t.Set(key, value)
	return badgerErrToKVErr(err)
}

func (txn *bTxn) Delete(ctx context.Context, key []byte) error {
	err := txn.t.Delete(key)
	return badgerErrToKVErr(err)
}

func (txn *bTxn) Commit() error {
	err := txn.t.Commit()
	return badgerErrToKVErr(err)
}

func (txn *bTxn) Discard() error {
	txn.t.Discard()
	return nil
}

var badgerErrToKVErrMap = map[error]error{
	badger.ErrEmptyKey:     corekv.ErrEmptyKey,
	badger.ErrKeyNotFound:  corekv.ErrNotFound,
	badger.ErrDiscardedTxn: corekv.ErrDiscardedTxn,
	badger.ErrDBClosed:     corekv.ErrDBClosed,
}

func badgerErrToKVErr(err error) error {
	if err == nil {
		return nil
	}

	// first we try the optimistic lookup, assuming
	// there is no preexisting wrapping
	mappedErr := badgerErrToKVErrMap[err]

	// then we check the wrapped error chain
	if mappedErr == nil {
		switch {
		case errors.Is(err, badger.ErrEmptyKey):
			mappedErr = corekv.ErrEmptyKey
		case errors.Is(err, badger.ErrKeyNotFound):
			mappedErr = corekv.ErrNotFound
		case errors.Is(err, badger.ErrDBClosed):
			mappedErr = corekv.ErrDBClosed
		// Annoyingly, badger's error wrapping seems to break `errors.Is`, so we have to
		// check the string
		case strings.Contains(err.Error(), badger.ErrDBClosed.Error()):
			mappedErr = corekv.ErrDBClosed
		default:
			return err // no mapping needed atm
		}
	}

	return errors.Join(mappedErr, err)

}

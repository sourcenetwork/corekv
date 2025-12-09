package badger

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/dgraph-io/badger/v4"

	"github.com/sourcenetwork/corekv"
)

type Datastore struct {
	db *badger.DB

	// Badger panics when creating a new iterator if the store has closed, instead of returning an error
	// like it does everywhere else.  We would much rather error than panic, so we have to track closed-ness
	// ourself on top of the badger tracking.
	closed  bool
	closeLk sync.RWMutex
}

var _ corekv.TxnStore = (*Datastore)(nil)
var _ corekv.Dropable = (*Datastore)(nil)

func NewDatastore(path string, opts badger.Options) (*Datastore, error) {
	opts.Dir = path
	opts.ValueDir = path
	opts.Logger = nil // badger is too chatty
	store, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return NewDatastoreFrom(store), nil
}

func NewDatastoreFrom(db *badger.DB) *Datastore {
	return &Datastore{
		db: db,
	}
}

func (b *Datastore) Get(ctx context.Context, key []byte) ([]byte, error) {
	txn, ok := corekv.TryGetCtxTxnG[*bTxn](ctx)
	if ok {
		return txn.Get(ctx, key)
	}

	txn = b.newTxn(true)
	defer txn.Discard()

	return txn.Get(ctx, key)
}

func (b *Datastore) Has(ctx context.Context, key []byte) (bool, error) {
	txn, ok := corekv.TryGetCtxTxnG[*bTxn](ctx)
	if ok {
		return txn.Has(ctx, key)
	}

	txn = b.newTxn(true)
	defer txn.Discard()

	return txn.Has(ctx, key)
}

func (b *Datastore) Set(ctx context.Context, key []byte, value []byte) error {
	txn, ok := corekv.TryGetCtxTxnG[*bTxn](ctx)
	if ok {
		return txn.Set(ctx, key, value)
	}

	txn = b.newTxn(false)
	defer txn.Discard()

	err := txn.Set(ctx, key, value)
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (b *Datastore) Delete(ctx context.Context, key []byte) error {
	txn, ok := corekv.TryGetCtxTxnG[*bTxn](ctx)
	if ok {
		return txn.Delete(ctx, key)
	}

	txn = b.newTxn(false)
	defer txn.Discard()

	err := txn.Delete(ctx, key)
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (b *Datastore) Close() error {
	b.closeLk.Lock()
	defer b.closeLk.Unlock()
	b.closed = true

	return b.db.Close()
}

func (b *Datastore) Iterator(ctx context.Context, iterOpts corekv.IterOptions) (corekv.Iterator, error) {
	txn, ok := corekv.TryGetCtxTxnG[*bTxn](ctx)
	if ok {
		return txn.iterator(iterOpts)
	}
	txn = b.newTxn(true)

	it, err := txn.iterator(iterOpts)
	if err != nil {
		return nil, err
	}

	// closer for discarding implicit txn
	// so that the txn is discarded when the
	// iterator is closed
	it.withCloser(func() error {
		txn.Discard()
		return nil
	})

	return it, nil
}

func (d *Datastore) DropAll() error {
	return d.db.DropAll()
}

func (b *Datastore) NewTxn(readonly bool) corekv.Txn {
	return b.newTxn(readonly)
}

func (b *Datastore) newTxn(readonly bool) *bTxn {
	state, onSuccess, onError, onDiscard := corekv.NewTxnState()
	return &bTxn{
		TxnState:  state,
		onSuccess: onSuccess,
		onError:   onError,
		onDiscard: onDiscard,
		t:         b.db.NewTransaction(!readonly),
		d:         b,
	}
}

type bTxn struct {
	corekv.TxnState

	onSuccess func()
	onError   func()
	onDiscard func()

	t *badger.Txn
	d *Datastore
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

func (txn *bTxn) Iterator(ctx context.Context, iterOpts corekv.IterOptions) (corekv.Iterator, error) {
	return txn.iterator(iterOpts)
}

func (txn *bTxn) iterator(iopts corekv.IterOptions) (iteratorCloser, error) {
	txn.d.closeLk.RLock()
	defer txn.d.closeLk.RUnlock()
	if txn.d.closed {
		return nil, corekv.ErrDBClosed
	}

	if iopts.Prefix != nil {
		return newPrefixIterator(txn, iopts.Prefix, iopts.Reverse, iopts.KeysOnly), nil
	}
	return newRangeIterator(txn, iopts.Start, iopts.End, iopts.Reverse, iopts.KeysOnly), nil
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
	txn.d.closeLk.RLock()
	defer txn.d.closeLk.RUnlock()

	var err error
	if txn.d.closed {
		err = corekv.ErrDBClosed
	} else {
		err = txn.t.Commit()
	}

	if err != nil {
		txn.onError()
	} else {
		txn.onSuccess()
	}
	return badgerErrToKVErr(err)
}

func (txn *bTxn) Discard() {
	txn.t.Discard()
	txn.onDiscard()
}

var badgerErrToKVErrMap = map[error]error{
	badger.ErrEmptyKey:     corekv.ErrEmptyKey,
	badger.ErrKeyNotFound:  corekv.ErrNotFound,
	badger.ErrDiscardedTxn: corekv.ErrDiscardedTxn,
	badger.ErrDBClosed:     corekv.ErrDBClosed,
	badger.ErrConflict:     corekv.ErrTxnConflict,
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
		case strings.Contains(err.Error(), badger.ErrConflict.Error()):
			mappedErr = corekv.ErrTxnConflict
		default:
			return err // no mapping needed atm
		}
	}

	return mappedErr
}

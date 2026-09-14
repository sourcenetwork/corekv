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
var _ corekv.BatchWriter = (*Datastore)(nil)

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

// WriteBatch implements [corekv.BatchWriter] over badger's own WriteBatch.
//
// It is NOT atomic as a whole.  badger caps how large the transaction underneath the
// batch may grow, and commits that transaction and opens a new one as soon as the next
// operation would cross the cap (badger's `WriteBatch.handleEntry`, on `ErrTxnTooBig`).
// A batch bigger than one badger transaction therefore lands as several writes, and a
// failure partway through leaves everything that was already committed in the store.
// Callers that need all-or-nothing want a [corekv.Txn].
//
// Like [Datastore.DropAll] it applies to the store and ignores any transaction held in
// ctx.
//
// A store opened in badger's managed mode cannot use this: badger's NewWriteBatch
// panics there.  [NewDatastore] never opens one that way; a DB handed to
// [NewDatastoreFrom] must not be in managed mode.
//
// A closed store is checked explicitly, the same way [bTxn.iterator] checks it:
// badger's own close detection is not enough here.  Flush's internal commit opens a
// replacement transaction, which waits on a watermark that a closed store's background
// goroutines will never advance again, so on a closed store Flush hangs forever instead
// of returning the error it returns from a plain Set.  Checking closed-ness ourselves
// before ever calling into badger avoids that hang.
func (b *Datastore) WriteBatch(ctx context.Context, ops []corekv.BatchOp) error {
	if len(ops) == 0 {
		return nil
	}

	b.closeLk.RLock()
	defer b.closeLk.RUnlock()
	if b.closed {
		return corekv.ErrDBClosed
	}

	wb := b.db.NewWriteBatch()
	// badger's release path when Flush is not reached.  After a successful Flush it is
	// a no-op: Flush has already finished the throttle and discarded the transaction.
	defer wb.Cancel()

	for _, op := range ops {
		var err error
		if op.Delete {
			err = wb.Delete(op.Key)
		} else {
			err = wb.Set(op.Key, op.Value)
		}
		if err != nil {
			return badgerErrToKVErr(err)
		}
	}

	return badgerErrToKVErr(wb.Flush())
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
	return &bTxn{
		t: b.db.NewTransaction(!readonly),
		d: b,
	}
}

type bTxn struct {
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
	err := txn.t.Commit()
	return badgerErrToKVErr(err)
}

func (txn *bTxn) Discard() {
	txn.t.Discard()
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

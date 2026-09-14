// This package implements a datastore backed by regolith, an embedded key-value
// engine written in Rust.  It is a thin adapter over
// [github.com/sourcenetwork/go-regolith], which owns the cgo binding and the
// Rust staticlib behind it.
//
// Because of that binding, `make ffi` must have been run in the go-regolith
// checkout before `go build` here, otherwise the link will fail with a missing
// `ffi/target/release/libregolith_ffi.a`.  See the `replace` in `go.mod`.
//
// Transactions are optimistic, so a commit that lost a validation race returns
// [corekv.ErrTxnConflict] and should be retried from a new transaction.  This
// matches the badger store's behaviour.
//
// They run at snapshot isolation unless the store was opened with another level.
// The level is a store-level setting - the `Isolation` field of
// [github.com/sourcenetwork/go-regolith.Options] - because [Datastore.NewTxn]
// leaves no room for a per-transaction one.  Snapshot isolation aborts on a
// write-write overlap and admits write skew; badger validates a transaction's
// read set as well, which is what go-regolith's `IsolationSerializable`
// asks for.
//
// Some important limitations to consider:
//
// - Iterators and transactions must be closed/discarded before [Datastore.Close],
// and an iterator created from a transaction must be closed before that
// transaction is committed or discarded.  The same ordering that badger requires.
//
// - Only a small subset of regolith's engine `Options` crosses the FFI so far,
// so only that subset can be passed to [NewDatastore].  See
// [github.com/sourcenetwork/go-regolith.Options].
package regolith

import (
	"context"
	"sync"

	"github.com/sourcenetwork/go-regolith"

	"github.com/sourcenetwork/corekv"
)

type Datastore struct {
	db *regolith.DB

	// batches holds the packed buffers of batches that have been written, so a store
	// fed batches of a similar shape stops allocating after the first few: a
	// [github.com/sourcenetwork/go-regolith.WriteBatch] keeps its buffer across
	// `Reset`.  They are pooled rather than kept in one field because a WriteBatch is
	// not safe for concurrent use, and `WriteBatch` may be called from several
	// goroutines at once.
	batches sync.Pool
}

var _ corekv.TxnStore = (*Datastore)(nil)
var _ corekv.Dropable = (*Datastore)(nil)
var _ corekv.BatchWriter = (*Datastore)(nil)

// NewDatastore opens (or creates) a regolith store at the given path with the
// given engine options, following the same shape as the leveldb store's
// constructor: the options are the engine's own type, and a nil `opts` means the
// engine defaults.
//
// A zero [github.com/sourcenetwork/go-regolith.Options] means the same thing, as
// only the fields explicitly set on it are applied.  Note that for several of
// those fields zero is a real setting rather than an absence of one, which is
// why the numeric ones are pointers.
func NewDatastore(path string, opts *regolith.Options) (*Datastore, error) {
	if opts == nil {
		opts = &regolith.Options{}
	}
	db, err := regolith.OpenWith(path, *opts)
	if err != nil {
		return nil, regolithErrToKVErr(err)
	}
	return &Datastore{
		db:      db,
		batches: sync.Pool{New: func() any { return regolith.NewWriteBatch(0) }},
	}, nil
}

func (d *Datastore) Get(ctx context.Context, key []byte) ([]byte, error) {
	txn, ok := corekv.TryGetCtxTxnG[*rTxn](ctx)
	if ok {
		return txn.Get(ctx, key)
	}
	value, err := d.db.Get(key)
	if err != nil {
		return nil, regolithErrToKVErr(err)
	}
	return value, nil
}

func (d *Datastore) Has(ctx context.Context, key []byte) (bool, error) {
	txn, ok := corekv.TryGetCtxTxnG[*rTxn](ctx)
	if ok {
		return txn.Has(ctx, key)
	}
	exists, err := d.db.Has(key)
	if err != nil {
		return false, regolithErrToKVErr(err)
	}
	return exists, nil
}

func (d *Datastore) Set(ctx context.Context, key []byte, value []byte) error {
	txn, ok := corekv.TryGetCtxTxnG[*rTxn](ctx)
	if ok {
		return txn.Set(ctx, key, value)
	}
	err := d.db.Set(key, value)
	return regolithErrToKVErr(err)
}

func (d *Datastore) Delete(ctx context.Context, key []byte) error {
	txn, ok := corekv.TryGetCtxTxnG[*rTxn](ctx)
	if ok {
		return txn.Delete(ctx, key)
	}
	err := d.db.Delete(key)
	return regolithErrToKVErr(err)
}

// WriteBatch implements [corekv.BatchWriter], applying every operation in one crossing
// of the FFI boundary instead of one crossing per operation.
//
// The whole batch lands or none of it does: the engine writes it as a single
// write-ahead log record.  It takes no snapshot and validates nothing against
// concurrent writers, and it ignores any transaction held in ctx, exactly as
// [Datastore.DropAll] does.
//
// The engine refuses a batch whose log record would be larger than it can replay,
// returning [github.com/sourcenetwork/go-regolith.ErrInvalidArgument] with the limit in
// its detail and writing nothing.  The limit is on bytes, not on the number of
// operations, so a caller with unbounded input has to split by size.
func (d *Datastore) WriteBatch(ctx context.Context, ops []corekv.BatchOp) error {
	if len(ops) == 0 {
		return nil
	}

	batch := d.batches.Get().(*regolith.WriteBatch) //nolint:forcetypeassert
	defer func() {
		batch.Reset()
		d.batches.Put(batch)
	}()

	for _, op := range ops {
		if op.Delete {
			batch.Delete(op.Key)
			continue
		}
		batch.Set(op.Key, op.Value)
	}

	return regolithErrToKVErr(d.db.Write(batch))
}

func (d *Datastore) Iterator(ctx context.Context, iterOpts corekv.IterOptions) (corekv.Iterator, error) {
	txn, ok := corekv.TryGetCtxTxnG[*rTxn](ctx)
	if ok {
		return txn.Iterator(ctx, iterOpts)
	}
	// Unlike badger, no implicit transaction is needed here: the store-level
	// iterator reads from a snapshot taken by the engine, so there is nothing
	// for the iterator to close besides itself.
	i, err := d.db.NewIter(toIterOptions(iterOpts))
	if err != nil {
		return nil, regolithErrToKVErr(err)
	}
	return &iterator{i: i}, nil
}

func (d *Datastore) DropAll() error {
	err := d.db.DropAll()
	return regolithErrToKVErr(err)
}

// Close closes the store.  Closing an already closed store is a no-op.
func (d *Datastore) Close() error {
	err := d.db.Close()
	return regolithErrToKVErr(err)
}

// NewTxn begins a transaction at the isolation level the store was opened with.
func (d *Datastore) NewTxn(readonly bool) corekv.Txn {
	// This error is only returned when the db is closed.
	// We store it for later and return it from all functions
	// to satisfy the transaction interface.
	t, err := d.db.NewTxn(readonly)
	return &rTxn{
		t:   t,
		err: err,
	}
}

type rTxn struct {
	t   *regolith.Txn
	err error
}

var _ corekv.Txn = (*rTxn)(nil)

func (txn *rTxn) Get(ctx context.Context, key []byte) ([]byte, error) {
	if txn.err != nil {
		return nil, regolithErrToKVErr(txn.err)
	}
	value, err := txn.t.Get(key)
	if err != nil {
		return nil, regolithErrToKVErr(err)
	}
	return value, nil
}

func (txn *rTxn) Has(ctx context.Context, key []byte) (bool, error) {
	if txn.err != nil {
		return false, regolithErrToKVErr(txn.err)
	}
	exists, err := txn.t.Has(key)
	if err != nil {
		return false, regolithErrToKVErr(err)
	}
	return exists, nil
}

func (txn *rTxn) Set(ctx context.Context, key []byte, value []byte) error {
	if txn.err != nil {
		return regolithErrToKVErr(txn.err)
	}
	err := txn.t.Set(key, value)
	return regolithErrToKVErr(err)
}

func (txn *rTxn) Delete(ctx context.Context, key []byte) error {
	if txn.err != nil {
		return regolithErrToKVErr(txn.err)
	}
	err := txn.t.Delete(key)
	return regolithErrToKVErr(err)
}

func (txn *rTxn) Iterator(ctx context.Context, iterOpts corekv.IterOptions) (corekv.Iterator, error) {
	if txn.err != nil {
		return nil, regolithErrToKVErr(txn.err)
	}
	i, err := txn.t.NewIter(toIterOptions(iterOpts))
	if err != nil {
		return nil, regolithErrToKVErr(err)
	}
	return &iterator{i: i}, nil
}

func (txn *rTxn) Commit() error {
	if txn.err != nil {
		return regolithErrToKVErr(txn.err)
	}
	err := txn.t.Commit()
	return regolithErrToKVErr(err)
}

func (txn *rTxn) Discard() {
	// the transaction might be nil if the db was closed prior to opening it
	if txn.t != nil {
		txn.t.Discard()
	}
}

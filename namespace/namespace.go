package namespace

import (
	"context"

	"github.com/sourcenetwork/corekv"
)

// Datastore wraps a namespace of another database as a logical database.
type Datastore struct {
	namespace []byte
	store     corekv.ReaderWriter
}

var _ corekv.ReaderWriter = (*Datastore)(nil)

// Wrap lets you namespace a store with a given prefix.
func Wrap(store corekv.ReaderWriter, prefix []byte) *Datastore {
	return &Datastore{
		namespace: prefix,
		store:     store,
	}
}

func (nstore *Datastore) Get(ctx context.Context, key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, corekv.ErrEmptyKey
	}

	pkey := nstore.prefixed(key)
	value, err := nstore.store.Get(ctx, pkey)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (nstore *Datastore) Has(ctx context.Context, key []byte) (bool, error) {
	if len(key) == 0 {
		return false, corekv.ErrEmptyKey
	}
	pkey := nstore.prefixed(key)

	has, err := nstore.store.Has(ctx, pkey)
	if err != nil {
		return false, err
	}
	return has, nil
}

func (nstore *Datastore) Set(ctx context.Context, key []byte, value []byte) error {
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	pkey := nstore.prefixed(key)

	return nstore.store.Set(ctx, pkey, value)
}

func (nstore *Datastore) Delete(ctx context.Context, key []byte) error {
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	pkey := nstore.prefixed(key)

	return nstore.store.Delete(ctx, pkey)
}

func (nstore *Datastore) prefixed(key []byte) []byte {
	return prefixed(nstore.namespace, key)
}

func prefixed(prefix, key []byte) []byte {
	return append(cp(prefix), key...)
}

// Iterator creates a new iterator instance
func (nstore *Datastore) Iterator(ctx context.Context, opts corekv.IterOptions) (corekv.Iterator, error) {
	if opts.Prefix != nil {
		opts.Prefix = nstore.prefixed(opts.Prefix)
	} else if opts.Start != nil || opts.End != nil {
		opts.Start = nstore.prefixed(opts.Start)

		if opts.End != nil {
			opts.End = nstore.prefixed(opts.End)
		} else {
			// End is exclusive, and if it is nil, it still needs limiting to the namespace, so we
			// set it to the namespace plus one
			opts.End = bytesPrefixEnd(nstore.namespace)
		}
	} else {
		// If all the scoping options are nil, we still need to scope the iterator
		// to the prefix.
		opts.Prefix = nstore.prefixed(opts.Prefix)
	}

	iterator, err := nstore.store.Iterator(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &namespaceIterator{
		namespace: nstore.namespace,
		it:        iterator,
	}, nil
}

type namespaceIterator struct {
	namespace []byte
	it        corekv.Iterator
}

var (
	_ corekv.Iterator      = (*namespaceIterator)(nil)
	_ corekv.ValueAppender = (*namespaceIterator)(nil)
	_ corekv.ValueBorrower = (*namespaceIterator)(nil)
)

func (nIter *namespaceIterator) Reset() {
	nIter.it.Reset()
}

func (nIter *namespaceIterator) Next() (bool, error) {
	return nIter.it.Next()
}

func (nIter *namespaceIterator) Key() []byte {
	key := nIter.it.Key()
	return key[len(nIter.namespace):] // strip namespace
}

func (nIter *namespaceIterator) Value() ([]byte, error) {
	return nIter.it.Value()
}

// AppendValue implements [corekv.ValueAppender].
//
// Namespacing rewrites keys only, values pass through it untouched, so this forwards
// to the underlying iterator where it implements the interface, and falls back to
// `Value` where it does not.  The fallback keeps the wrapper correct over any
// underlying store, at the cost of it being no faster than `Value` for those stores.
//
// Note that because the method is always defined, a namespaced iterator always
// satisfies [corekv.ValueAppender], even when the store beneath it has nothing to gain
// from it - a caller's type assertion will succeed, it just may not save them an
// allocation.  Correctness over any underlying store is preferred here over preserving
// that (fairly weak) signal.
func (nIter *namespaceIterator) AppendValue(dst []byte) ([]byte, error) {
	if appender, ok := nIter.it.(corekv.ValueAppender); ok {
		return appender.AppendValue(dst)
	}

	value, err := nIter.it.Value()
	if err != nil {
		return nil, err
	}

	return append(dst, value...), nil
}

// BorrowValue implements [corekv.ValueBorrower].
//
// Like `AppendValue` it forwards to the underlying iterator where it implements the
// interface, and falls back to `Value` where it does not.  Falling back yields a slice
// that outlives `fn`, which is permitted - the contract bounds how long the caller may
// use the bytes for, not how long they remain valid.
//
// Any error returned by `fn` is returned unchanged on both paths.
func (nIter *namespaceIterator) BorrowValue(fn func(value []byte) error) error {
	if borrower, ok := nIter.it.(corekv.ValueBorrower); ok {
		return borrower.BorrowValue(fn)
	}

	value, err := nIter.it.Value()
	if err != nil {
		return err
	}

	return fn(value)
}

func (nIter *namespaceIterator) Seek(key []byte) (bool, error) {
	pKey := prefixed(nIter.namespace, key)
	return nIter.it.Seek(pKey)
}

func (nIter *namespaceIterator) Close() error {
	return nIter.it.Close()
}

func cp(bz []byte) (ret []byte) {
	ret = make([]byte, len(bz))
	copy(ret, bz)
	return ret
}

func bytesPrefixEnd(b []byte) []byte {
	end := make([]byte, len(b))
	copy(end, b)
	for i := len(end) - 1; i >= 0; i-- {
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	// This statement will only be reached if the key is already a
	// maximal byte string (i.e. already \xff...).
	return b
}

package namespace

import (
	"context"

	"github.com/sourcenetwork/corekv"
)

// Datastore wraps a namespace of another database as a logical database.
type Datastore struct {
	namespace []byte
	store     corekv.Store
}

var _ corekv.Store = (*Datastore)(nil)

type TxnStore struct {
	Datastore

	store corekv.TxnStore
}

var _ corekv.TxnStore = (*TxnStore)(nil)

type Txn struct {
	corekv.Store

	txn corekv.Txn
}

var _ corekv.Txn = (*Txn)(nil)

// Wrap lets you namespace a store with a given prefix.
func Wrap(store corekv.Store, prefix []byte) *Datastore {
	return &Datastore{
		namespace: prefix,
		store:     store,
	}
}

// WrapTS lets you namespace a transaction store with a given prefix.
func WrapTS(store corekv.TxnStore, prefix []byte) *TxnStore {
	return &TxnStore{
		Datastore: *Wrap(store, prefix),
		store:     store,
	}
}

// WrapTxn lets you namespace a transaction with a given prefix.
func WrapTxn(txn corekv.Txn, prefix []byte) *Txn {
	return &Txn{
		Store: Wrap(txn, prefix),
		txn:   txn,
	}
}

func (nstore *TxnStore) NewTxn(readonly bool) corekv.Txn {
	txn := nstore.store.NewTxn(readonly)
	return WrapTxn(txn, nstore.namespace)
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

func (nstore *Datastore) Close() error {
	return nstore.store.Close()
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

func (nIter *namespaceIterator) Seek(key []byte) (bool, error) {
	pKey := prefixed(nIter.namespace, key)
	return nIter.it.Seek(pKey)
}

func (nIter *namespaceIterator) Close() error {
	return nIter.it.Close()
}

func (txn *Txn) Commit() error {
	return txn.txn.Commit()
}

func (txn *Txn) Discard() {
	txn.txn.Discard()
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

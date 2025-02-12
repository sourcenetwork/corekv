package namespace

import (
	"context"

	"github.com/sourcenetwork/corekv"
)

// namespaceStore wraps a namespace of another database as a logical database.
type namespaceStore struct {
	namespace []byte
	store     corekv.Store
}

var _ corekv.TxnStore = (*namespaceStore)(nil)

type namespacedTxn struct {
	corekv.Store

	txn corekv.Txn
}

var _ corekv.Txn = (*namespacedTxn)(nil)

// Wrap lets you namespace a store with a given prefix.
func Wrap(store corekv.Store, prefix []byte) corekv.Store {
	return &namespaceStore{
		namespace: prefix,
		store:     store,
	}
}

// WrapTS lets you namespace a transaction store with a given prefix.
func WrapTS(store corekv.TxnStore, prefix []byte) corekv.TxnStore {
	return &namespaceStore{
		namespace: prefix,
		store:     store,
	}
}

// WrapTxn lets you namespace a transaction with a given prefix.
func WrapTxn(txn corekv.Txn, prefix []byte) corekv.Txn {
	return &namespacedTxn{
		Store: Wrap(txn, prefix),
		txn:   txn,
	}
}

func (nstore *namespaceStore) NewTxn(readonly bool) corekv.Txn {
	// Here we assume the cast is safe, as this function is only ever publically
	// exposed by values returned from `WrapTS`, in which case the type is always
	// `TxnStore`.
	txn := nstore.store.(corekv.TxnStore).NewTxn(readonly)
	return WrapTxn(txn, nstore.namespace)
}

func (nstore *namespaceStore) Get(ctx context.Context, key []byte) ([]byte, error) {
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

func (nstore *namespaceStore) Has(ctx context.Context, key []byte) (bool, error) {
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

func (nstore *namespaceStore) Set(ctx context.Context, key []byte, value []byte) error {
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	pkey := nstore.prefixed(key)

	return nstore.store.Set(ctx, pkey, value)
}

func (nstore *namespaceStore) Delete(ctx context.Context, key []byte) error {
	if len(key) == 0 {
		return corekv.ErrEmptyKey
	}
	pkey := nstore.prefixed(key)

	return nstore.store.Delete(ctx, pkey)
}

func (nstore *namespaceStore) Close() error {
	return nstore.store.Close()
}

func (nstore *namespaceStore) prefixed(key []byte) []byte {
	return prefixed(nstore.namespace, key)
}

func prefixed(prefix, key []byte) []byte {
	return append(cp(prefix), key...)
}

// Iterator creates a new iterator instance
func (nstore *namespaceStore) Iterator(ctx context.Context, opts corekv.IterOptions) corekv.Iterator {
	if opts.Prefix != nil {
		opts.Prefix = nstore.prefixed(opts.Prefix)
	} else {
		if opts.Start != nil {
			opts.Start = nstore.prefixed(opts.Start)
		}
		if opts.End != nil {
			opts.End = nstore.prefixed(opts.End)
		}
	}

	return &namespaceIterator{
		namespace: nstore.namespace,
		it:        nstore.store.Iterator(ctx, opts),
	}
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

func (txn *namespacedTxn) Commit() error {
	return txn.txn.Commit()
}

func (txn *namespacedTxn) Discard() error {
	return txn.txn.Discard()
}

func cp(bz []byte) (ret []byte) {
	ret = make([]byte, len(bz))
	copy(ret, bz)
	return ret
}

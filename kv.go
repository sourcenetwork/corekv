package corekv

import "context"

// DefaultIterOptions is exactly the default zero value
// for the IterOptions stuct. It is however recomended
// to use this if you want the "default" behavior, as
// these values may change in some way in the future
// and the old "default" as zero values may produce
// different behavior.
var DefaultIterOptions = IterOptions{}

// IterOptions contains the full set of available iterator options,
// it can be provided when creating an [Iterator] from a [Store].
type IterOptions struct {
	// Prefix iteration, only keys beginning with the designated prefix
	// with the given prefix will be yielded.
	//
	// Keys exactly matching the provided `Prefix` value will not be
	// yielded.
	//
	// Providing a Prefix value should cause the Start and End options
	// to be ignored, although this is currently untested:
	// https://github.com/sourcenetwork/corekv/issues/35
	Prefix []byte

	// If Prefix is nil, and Start is provided, the iterator will
	// only yield items with a key lexographically greater than or
	// equal to this value.
	//
	// Providing an `End` value equal to or smaller than this value
	// will result in undefined behaviour:
	// https://github.com/sourcenetwork/corekv/issues/32
	Start []byte

	// If Prefix is nil, and End is provided, the iterator will
	// only yield items with a key lexographically smaller than this
	// value.
	//
	// Providing an End value equal to or smaller than Start
	// will result in undefined behaviour:
	// https://github.com/sourcenetwork/corekv/issues/32
	End []byte

	// Reverse the direction of the iteration, returning items in
	// lexographically descending order of their keys.
	Reverse bool

	// Only iterate through keys. Calling Value on the
	// iterator will return nil and no error.
	//
	// This option is currently untested:
	// https://github.com/sourcenetwork/corekv/issues/34
	//
	// It is very likely ignored for the memory store iteration:
	// https://github.com/sourcenetwork/corekv/issues/33
	KeysOnly bool
}

// Reader contains read-only functions for interacting with a store.
type Reader interface {
	// Get returns the value at the given key.
	//
	// If no item with the given key is found, nil, and an [ErrNotFound]
	// error will be returned.
	Get(ctx context.Context, key []byte) ([]byte, error)

	// Has returns true if an item at the given key is found, otherwise
	// will return false.
	Has(ctx context.Context, key []byte) (bool, error)

	// Iterator returns a read-only iterator using the given options.
	Iterator(ctx context.Context, opts IterOptions) (Iterator, error)
}

// Writer contains functions for mutating values within a store.
type Writer interface {
	// Set sets the value stored against the given key.
	//
	// If an item already exists at the given key it will be overwritten.
	Set(ctx context.Context, key, value []byte) error

	// Delete removes the value at the given key.
	//
	// If no matching key is found the behaviour is undefined:
	// https://github.com/sourcenetwork/corekv/issues/36
	Delete(ctx context.Context, key []byte) error
}

// ReaderWriter contains the functions for reading and writing values within the store.
type ReaderWriter interface {
	Reader
	Writer
}

// Iterator is a read-only iterator that allows iteration over the underlying
// store (or a part of it).
//
// Iterator implements the [Enumerable](https://github.com/sourcenetwork/immutable/blob/main/enumerable/enumerable.go)
// allowing instances of this type to make use of the various utilities within that package.
type Iterator interface {
	// Next attempts to move the iterator forward, it will return `true` if it was successful,
	// otherwise `false`.
	Next() (bool, error)

	// Key returns the key at the current iterator location.
	//
	// If the iterator is currently at an invalid location it's behaviour is undefined:
	// https://github.com/sourcenetwork/corekv/issues/37
	Key() []byte

	// Value returns the value at the current iterator location.
	//
	// If the iterator is currently at an invalid location it's behaviour is undefined:
	// https://github.com/sourcenetwork/corekv/issues/37
	Value() ([]byte, error)

	// Seek moves the iterator to the given key, if an exact match is not found, the
	// iterator will progress to the next valid value (depending on the `Reverse` option).
	//
	// Seek will return `true` if it found a valid item, otherwise `false`.
	//
	// Seek will not seek to values outside of the constraints provided in [IterOptions].
	Seek([]byte) (bool, error)

	// Reset resets the iterator, allowing for re-iteration.
	Reset()

	// Close releases the iterator.
	Close() error
}

// Dropable is an optional interface implemented by some Stores.
//
// It provides a convenient and cheap way of deleting all the data in an existing store.
type Dropable interface {
	// DropAll deletes all the data stored in the store.
	//
	// Concurrent writes will be blocked, however depending on the underlying store implementation,
	// concurrent reads may not be.  Badger does not block concurrent reads, but Memory store does.
	DropAll() error
}

// Store contains all the functions required for interacting with a store.
type Store interface {
	ReaderWriter

	// Close disposes of any resources directly held by the store.
	//
	// WARNING: Some implmentations close transactions and iterators, others do not:
	// https://github.com/sourcenetwork/corekv/issues/39
	Close() error
}

// TxnStore represents a [Store] that supports transactions.
type TxnStore interface {
	Store

	// NewTxn returns a new transaction.
	NewTxn(readonly bool) Txn
}

// Txn isolates changes made to the underlying store from this object,
// and isolates changes made via this object from the underlying store
// until `Commit` is called.
type Txn interface {
	ReaderWriter

	// Commit applies all changes made via this [Txn] to the underlying
	// [Store].
	Commit() error

	// Discard discards all changes made via this object so far, returning
	// it to the state it was at at time of construction.
	Discard()
}

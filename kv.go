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

// ValueAppender is an optional interface implemented by some Iterators.
//
// It allows a caller to supply the buffer that the current value is written into, so
// that a single buffer may be reused across an entire iteration instead of a new one
// being allocated for every value.
//
// It is the interface to reach for if the caller needs to retain the value beyond the
// current iteration step; if the value is consumed immediately, [ValueBorrower] avoids
// the copy as well.
//
// Implementations that can hand out a value without allocating at all (for example by
// returning a sub-slice of their own storage) have nothing to gain from this and are
// not expected to implement it.  Callers should type-assert for it and fall back to
// [Iterator.Value].
type ValueAppender interface {
	// AppendValue appends the value at the current iterator location to dst
	// and returns the extended slice, following the convention of the
	// stdlib's Append* functions.
	//
	// Note that this saves the allocation of the destination buffer, it does not
	// save the copy - the value bytes are still copied into dst.
	//
	// If the iterator was created with the [IterOptions.KeysOnly] option, dst is
	// returned unchanged and no error, mirroring [Iterator.Value].
	//
	// If the iterator is currently at an invalid location it's behaviour is undefined:
	// https://github.com/sourcenetwork/corekv/issues/37
	//
	// The returned slice is only valid until the next call to AppendValue.
	AppendValue(dst []byte) ([]byte, error)
}

// ValueBorrower is an optional interface implemented by some Iterators.
//
// It allows a caller to read the value at the current iterator location without the
// store copying it, for stores that are able to expose the bytes that they already
// hold.  It is the interface to reach for if the caller consumes the value immediately;
// callers that need to retain it must copy it, and may find [ValueAppender] more
// convenient.
//
// The lifetime of the borrowed bytes is bound lexically, by the callback, rather than
// by a handle that the caller must remember to release.
type ValueBorrower interface {
	// BorrowValue calls fn with the value at the current iterator location.
	//
	// The slice passed to fn is only valid for the duration of the call, and must not
	// be retained or mutated.  Callers that need the value afterwards must copy it, or
	// use [Iterator.Value] instead.
	//
	// If the iterator was created with the [IterOptions.KeysOnly] option, fn is called
	// with nil, mirroring [Iterator.Value] returning nil.  fn is always called exactly
	// once unless an error prevents the value from being read.
	//
	// If the iterator is currently at an invalid location it's behaviour is undefined:
	// https://github.com/sourcenetwork/corekv/issues/37
	//
	// Any error returned by fn is returned by BorrowValue unchanged and unwrapped -
	// implementations must not add context to it.
	//
	// A caller therefore cannot tell a store-side failure from their own callback's
	// failure by inspecting the returned error alone.  Callers that need to distinguish
	// the two should use their own sentinel error, or record the failure in a variable
	// captured by the closure.
	BorrowValue(fn func(value []byte) error) error
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

// BatchOp is a single operation within a batch handed to [BatchWriter.WriteBatch].
type BatchOp struct {
	// Key is the key to write to, or to delete.
	Key []byte

	// Value is the value to store against Key.  It is ignored when Delete is true.
	Value []byte

	// Delete makes this operation remove the item at Key, mirroring [Writer.Delete],
	// instead of writing to it, mirroring [Writer.Set].
	Delete bool
}

// BatchWriter is an optional interface implemented by some Stores.
//
// It allows a caller that already knows every key it wants to change to hand the whole
// set to the store at once, for stores that can apply it in a single write instead of
// one write per operation.  Callers should type-assert for it and fall back to
// [Writer.Set] and [Writer.Delete].
//
// It is not a transaction.  Nothing is read, nothing is validated against concurrent
// writers, no snapshot is taken, no conflict can be reported, and nothing can be rolled
// back once written.  Callers that need to read what they wrote, or to abandon the
// writes, want a [Txn].
type BatchWriter interface {
	// WriteBatch applies the given operations to the store.
	//
	// Operations apply in the order they are given, so the last operation on a key is
	// the one that survives.  Deleting a key that holds no item is not an error, as in
	// [Writer.Delete].  An empty or nil `ops` writes nothing and returns nil.
	//
	// Atomicity is the implementation's own property, and differs between stores.  The
	// regolith store applies the whole batch or none of it.  The badger store does not:
	// badger caps the size of the transaction underneath its batch and starts a new one
	// when the cap is reached, so a large batch lands as several writes and an error
	// partway through leaves the operations that already landed in the store.  A caller
	// that needs all-or-nothing must read the implementation it is holding, or use a
	// [Txn].
	//
	// The batch applies to the store, not to any transaction carried in ctx.  Like
	// [Dropable.DropAll], an implementation ignores a context transaction; a caller that
	// wants these writes inside a transaction must use [Writer.Set] and [Writer.Delete]
	// on it.
	//
	// A store may refuse a batch that is too large to apply in one write.  What it
	// bounds is bytes, not the number of operations, so a caller building a batch from
	// unbounded input must accumulate key and value bytes up to a budget of its own,
	// write, and start the next batch.
	//
	// The store may read Key and Value until WriteBatch returns and does not retain
	// either afterwards.  A caller must not modify them while the call is running.
	WriteBatch(ctx context.Context, ops []BatchOp) error
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

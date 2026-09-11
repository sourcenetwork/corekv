package regolith

import (
	"github.com/sourcenetwork/go-regolith"

	"github.com/sourcenetwork/corekv"
)

// iterator adapts a regolith iterator to [corekv.Iterator].
//
// All of corekv's fussier iteration semantics - exclusive `End`, `Prefix`
// overriding `Start` and `End`, seek clamping, and the "pending reset" start
// position - are implemented by the engine, so there is nothing to reimplement
// here.
//
// Iterator handles are not thread-safe, so neither is this type.  The store
// must not be closed while one is alive.
type iterator struct {
	i *regolith.Iter
}

var (
	_ corekv.Iterator      = (*iterator)(nil)
	_ corekv.ValueAppender = (*iterator)(nil)
	_ corekv.ValueBorrower = (*iterator)(nil)
)

// toIterOptions translates corekv's iterator options into regolith's, which
// have the same fields and the same meanings.
func toIterOptions(iterOpts corekv.IterOptions) regolith.IterOptions {
	return regolith.IterOptions{
		Prefix:   iterOpts.Prefix,
		Start:    iterOpts.Start,
		End:      iterOpts.End,
		Reverse:  iterOpts.Reverse,
		KeysOnly: iterOpts.KeysOnly,
	}
}

func (it *iterator) Next() (bool, error) {
	hasNext, err := it.i.Next()
	if err != nil {
		return false, regolithErrToKVErr(err)
	}
	return hasNext, nil
}

func (it *iterator) Seek(key []byte) (bool, error) {
	hasValue, err := it.i.Seek(key)
	if err != nil {
		return false, regolithErrToKVErr(err)
	}
	return hasValue, nil
}

func (it *iterator) Reset() {
	it.i.Reset()
}

// Key returns the key at the current iterator location, or nil if the iterator
// is not at a valid location.
//
// The store being closed does not affect this, as the entry is held by the
// iterator and not read back out of the store - matching the other stores,
// which also keep yielding the current entry after a close.
func (it *iterator) Key() []byte {
	return it.i.Key()
}

// Value returns the value at the current iterator location, or nil if the
// iterator is not at a valid location or was created with `KeysOnly`.
func (it *iterator) Value() ([]byte, error) {
	value, err := it.i.Value()
	if err != nil {
		return nil, regolithErrToKVErr(err)
	}
	return value, nil
}

// AppendValue implements [corekv.ValueAppender], appending the value at the
// current iterator location to the caller's own buffer.
//
// The engine hands the value over as a borrowed pointer into its own memory, so
// this is one copy straight into dst and no allocation when dst has the capacity
// - where `Value` both copies and allocates.
func (it *iterator) AppendValue(dst []byte) ([]byte, error) {
	dst, err := it.i.AppendValue(dst)
	if err != nil {
		return nil, regolithErrToKVErr(err)
	}
	return dst, nil
}

// BorrowValue implements [corekv.ValueBorrower], yielding the engine's own value
// bytes to the given function instead of copying them.
//
// The slice given to `fn` points into memory the engine owns, and is valid only
// for the duration of the call: it must not be retained or mutated, and `fn` must
// not call back into the iterator, as advancing, seeking, resetting or closing it
// all invalidate those bytes.  See `regolith.Iter.BorrowValue` for the full
// lifetime argument.
//
// [corekv.ValueBorrower] requires `fn`'s error back unchanged and unwrapped, and
// `regolithErrToKVErr` would not honour that for an `fn` that happened to return
// something matching a regolith sentinel.  The two origins are therefore told
// apart directly: whether `fn` ran at all decides whose error is being returned,
// as the only failure that can reach here without running `fn` is the store's.
func (it *iterator) BorrowValue(fn func(value []byte) error) error {
	called := false
	err := it.i.BorrowValue(func(value []byte) error {
		called = true
		return fn(value)
	})
	if called {
		return err
	}
	return regolithErrToKVErr(err)
}

// Close releases the iterator.  Closing an already closed iterator is a no-op,
// and it remains safe to call after the store has been closed.
func (it *iterator) Close() error {
	err := it.i.Close()
	return regolithErrToKVErr(err)
}

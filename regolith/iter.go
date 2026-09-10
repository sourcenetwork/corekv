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

var _ corekv.Iterator = (*iterator)(nil)

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

// Close releases the iterator.  Closing an already closed iterator is a no-op,
// and it remains safe to call after the store has been closed.
func (it *iterator) Close() error {
	err := it.i.Close()
	return regolithErrToKVErr(err)
}

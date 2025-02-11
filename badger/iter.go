package badger

import (
	"bytes"

	"github.com/dgraph-io/badger/v4"

	"github.com/sourcenetwork/corekv"
)

type iteratorCloser interface {
	corekv.Iterator
	withCloser(func() error)
}

type iterator struct {
	i        *badger.Iterator
	start    []byte
	end      []byte
	reverse  bool
	keysOnly bool
	// reset is a mutatuble property that indicates whether the iterator should be
	// returned to the beginning on the next [Next] call.
	reset  bool
	closer func() error
}

func newPrefixIterator(txn *bTxn, prefix []byte, reverse, keysOnly bool) *iterator {
	opt := badger.DefaultIteratorOptions
	opt.Reverse = reverse
	opt.Prefix = prefix
	opt.PrefetchValues = !keysOnly

	return &iterator{
		i:        txn.t.NewIterator(opt),
		start:    prefix,
		end:      bytesPrefixEnd(prefix),
		reverse:  reverse,
		keysOnly: keysOnly,
		reset:    true,
	}
}

func newRangeIterator(txn *bTxn, start, end []byte, reverse, keysOnky bool) *iterator {
	opt := badger.DefaultIteratorOptions
	opt.Reverse = reverse
	opt.PrefetchValues = !keysOnky

	return &iterator{
		i:        txn.t.NewIterator(opt),
		start:    start,
		end:      end,
		reverse:  reverse,
		keysOnly: keysOnky,
		reset:    true,
	}
}

func (it *iterator) Reset() {
	it.reset = true
}

// restart returns the iterator back to it's initial location at time of construction,
// allowing re-iteration of the underlying data.
func (it *iterator) restart() (bool, error) {
	it.reset = false

	if it.reverse {
		return it.Seek(it.end)
	} else {
		return it.Seek(it.start)
	}
}

func (it *iterator) valid() bool {
	if !it.i.Valid() {
		return false
	}

	if len(it.start) > 0 && lt(it.i.Item().Key(), it.start) {
		return false
	}

	if len(it.end) > 0 && gte(it.i.Item().Key(), it.end) {
		return false
	}

	return true
}

func (it *iterator) Next() (bool, error) {
	if it.reset {
		return it.restart()
	}

	if !it.i.Valid() {
		// `it.i.Next()` will panic if we attempt to call it having already reached
		// the iterator end, so we must return before trying to make that call if we can.
		return false, nil
	}

	it.i.Next()
	return it.valid(), nil
}

func (it *iterator) Key() []byte {
	return it.i.Item().KeyCopy(nil)
}

func (it *iterator) Value() ([]byte, error) {
	if it.keysOnly {
		return nil, nil
	}

	return it.i.Item().ValueCopy(nil)
}

func (it *iterator) Seek(key []byte) (bool, error) {
	// Clear the reset property, else if Next was call following Seek,
	// Next may incorrectly return to the beginning.
	it.reset = false

	var target []byte
	if it.reverse {
		if it.end != nil && lt(it.end, key) {
			// We should not yield keys greater/equal to the `end`, so if the given seek-key
			// is greater than `end`, we should instead seek to `end`.
			target = it.end
		} else {
			target = key
		}
	} else {
		if it.start != nil && lt(key, it.start) {
			// We should not yield keys smaller than `start`, so if the given seek-key
			// is smaller than `start`, we should instead seek to `start`.
			target = it.start
		} else {
			target = key
		}
	}

	it.i.Seek(target)

	if !it.valid() {
		return it.Next()
	}

	return true, nil
}

func (it *iterator) Close() error {
	it.i.Close()
	if it.closer != nil {
		return it.closer()
	}
	return nil
}

func (it *iterator) withCloser(closer func() error) {
	it.closer = closer
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

// less than (a < b)
func lt(a, b []byte) bool {
	return bytes.Compare(a, b) == -1
}

// greater than or equal to (a >= b)
func gte(a, b []byte) bool {
	return bytes.Compare(a, b) > -1
}

package leveldb

import (
	"bytes"

	"github.com/sourcenetwork/corekv"
	iter "github.com/sourcenetwork/goleveldb/leveldb/iterator"
)

type iterator struct {
	d        *Datastore
	i        iter.Iterator
	start    []byte
	end      []byte
	reverse  bool
	keysOnly bool
	// reset is a mutatuble property that indicates whether the iterator should be
	// returned to the beginning on the next [Next] call.
	reset  bool
	closer func() error
}

func (it *iterator) Reset() {
	it.reset = true
}

// restart returns the iterator back to it's initial location at time of construction,
// allowing re-iteration of the underlying data.
func (it *iterator) restart() (bool, error) {
	it.reset = false

	if it.reverse {
		return it.i.Last(), nil
	} else {
		return it.i.First(), nil
	}
}

func (it *iterator) Next() (bool, error) {
	if it.d.closed.Load() {
		return false, corekv.ErrDBClosed
	}
	if it.reset {
		return it.restart()
	}
	if it.reverse {
		return it.i.Prev(), nil
	}
	return it.i.Next(), nil
}

func (it *iterator) Key() []byte {
	return bytes.Clone(it.i.Key())
}

func (it *iterator) Value() ([]byte, error) {
	if it.keysOnly {
		return nil, nil
	}
	if len(it.i.Value()) == 0 {
		return nil, nil
	}
	return bytes.Clone(it.i.Value()), nil
}

func (it *iterator) Seek(key []byte) (bool, error) {
	it.reset = false
	if it.d.closed.Load() {
		return false, corekv.ErrDBClosed
	}
	var target []byte
	if it.reverse {
		if it.end != nil && bytes.Compare(it.end, key) < 0 {
			// We should not yield keys greater/equal to the `end`, so if the given seek-key
			// is greater than `end`, we should instead seek to `end`.
			target = it.end
		} else {
			target = key
		}
	} else {
		if it.start != nil && bytes.Compare(key, it.start) >= 0 {
			// We should not yield keys smaller than `start`, so if the given seek-key
			// is smaller than `start`, we should instead seek to `start`.
			target = it.start
		} else {
			target = key
		}
	}
	it.i.Seek(target)
	if !it.i.Valid() {
		return it.Next()
	}
	return true, nil
}

func (it *iterator) Close() error {
	it.i.Release()
	if it.closer != nil {
		return it.closer()
	}
	return nil
}

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
	reset bool
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
	value := it.i.Value()
	if len(value) == 0 {
		return nil, nil
	}
	return bytes.Clone(value), nil
}

func (it *iterator) Seek(key []byte) (bool, error) {
	it.reset = false
	if it.d.closed.Load() {
		return false, corekv.ErrDBClosed
	}
	if it.reverse && it.end != nil && bytes.Compare(it.end, key) < 0 {
		return it.i.Last(), nil
	}
	if !it.reverse && it.start != nil && bytes.Compare(key, it.start) < 0 {
		return it.i.First(), nil
	}
	if !it.i.Seek(key) {
		return false, nil
	}
	// seek finds a key with a greater than or equal value
	// when reversed we want a less than or equal value
	if it.reverse && bytes.Compare(it.i.Key(), key) > 0 {
		return it.i.Prev(), nil
	}
	return it.i.Valid(), nil
}

func (it *iterator) Close() error {
	it.i.Release()
	return nil
}

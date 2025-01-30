package memory

import (
	"bytes"
	"context"

	"github.com/sourcenetwork/corekv"

	"github.com/tidwall/btree"
)

type iterator struct {
	db      *Datastore
	version uint64
	it      btree.IterG[dsItem]

	// The key at which this iterator begins.
	//
	// This is inclusive only if `mayExactlyMatchStart` is true.
	start []byte

	// The key at which this iterator ends, inclusive.
	end []byte

	// If this is true, `start` is inclusive, else `start` is exclusive.
	isStartInclusive bool

	// If true, the iterator will iterate in reverse order, from the largest
	// key to the smallest.
	reverse bool

	// reset is a mutatuble property that indicates whether the iterator should be
	// returned to the beginning on the next [Next] call.
	reset bool
}

var _ corekv.Iterator = (*iterator)(nil)

func newPrefixIter(db *Datastore, prefix []byte, reverse bool, version uint64) *iterator {
	return &iterator{
		db:      db,
		version: version,
		it:      db.values.Iter(),
		start:   prefix,
		end:     bytesPrefixEnd(prefix),
		// A prefix iterator must not return a key exactly matching itself.
		isStartInclusive: false,
		reverse:          reverse,
		reset:            true,
	}
}

func newRangeIter(db *Datastore, start, end []byte, reverse bool, version uint64) *iterator {
	return &iterator{
		db:               db,
		version:          version,
		it:               db.values.Iter(),
		start:            start,
		end:              end,
		isStartInclusive: true,
		reverse:          reverse,
		reset:            true,
	}
}

func (iter *iterator) Reset() {
	iter.reset = true
}

// restart returns the iterator back to it's initial location at time of construction,
// allowing re-iteration of the underlying data.
func (iter *iterator) restart() (bool, error) {
	iter.reset = false

	if len(iter.end) > 0 && iter.reverse {
		return iter.Seek(iter.end)
	} else if len(iter.start) > 0 && !iter.reverse {
		return iter.Seek(iter.start)
	} else {
		var hasItem bool
		if iter.reverse {
			hasItem = iter.it.Last()
			// We don't need to bother loading the latest item in reverse, as the Last item
			// will be of the latest version anyway.
		} else {
			hasItem = iter.it.First()
			iter.loadLatestItem()
		}

		if !hasItem {
			return false, nil
		}

		if !iter.valid() {
			return iter.Next()
		}

		return true, nil
	}

}

func (iter *iterator) valid() bool {
	if len(iter.it.Item().key) == 0 {
		return false
	}

	if iter.it.Item().isDeleted {
		return false
	}

	if !iter.isStartInclusive && (!iter.reverse && bytes.Equal(iter.it.Item().key, iter.start) ||
		iter.reverse && bytes.Equal(iter.it.Item().key, iter.end)) {
		return false
	}

	if len(iter.end) > 0 && !lt(iter.it.Item().key, iter.end) {
		return false
	}

	return gte(iter.it.Item().key, iter.start)
}

func (iter *iterator) Next() (bool, error) {
	if iter.reset {
		return iter.restart()
	}

	previousItem := iter.it.Item()
	var hasItem bool
	for iter.next() {
		// Scan through until we reach the next key.
		// It doesn't matter if it is deleted or not.
		if !bytes.Equal(previousItem.key, iter.it.Item().key) {
			hasItem = true
			break
		}
	}

	if !hasItem {
		return false, nil
	}

	iter.loadLatestItem()

	if iter.it.Item().isDeleted {
		return iter.Next()
	}

	return iter.valid(), nil
}

func (iter *iterator) next() bool {
	if iter.reverse {
		// WARNING: There is a bug in `Prev` that can cause unexpected behaviour
		// when attempting to iterate beyond the end of the iterator.
		//
		// This is documented by the test `TestBTreePrevBug`, and our current
		// interface/implementation should prevent it from surfacing, but be careful
		// with this call.
		return iter.it.Prev()
	}
	return iter.it.Next()
}

func (iter *iterator) Key() []byte {
	return iter.it.Item().key
}

func (iter *iterator) Value() ([]byte, error) {
	return iter.it.Item().val, nil
}

func (iter *iterator) Seek(key []byte) (bool, error) {
	// Clear the reset property, else if Next was call following Seek,
	// Next may incorrectly return to the beginning.
	iter.reset = false

	// get the correct initial version for the seek
	// if there exists an exact match in keys, use the latest version
	// of that key, otherwise, use the provided DB version
	// TODO this could use some "peek" mechanic instead of a full lookup
	version := iter.version
	result := iter.db.get(key, version)
	if result.key != nil && !result.isDeleted {
		version = result.version
	}

	var hasItem bool
	if iter.reverse {
		// Unfortunately the BTree iterator doesn't provide a reversed seek, so we have to
		// do a bit of work ourselves here if iterating in reverse.

		var target []byte
		if iter.end != nil && lt(iter.end, key) {
			// We should not yield keys greater/equal to the `end`, so if the given seek-key
			// is greater than `end`, we should instead seek to `end`.
			target = iter.end
		} else {
			target = key
		}

		hasItem = iter.it.Seek(dsItem{key: target, version: version})
		if hasItem {
			if !bytes.Equal(target, iter.it.Item().key) {
				// If the BTree iterator `Seek` finds an item, it must be equal or greater than
				// our upper bound.  The previous item key must then be less than our upper bound
				// so if it is not equal we must look back once.
				hasItem = iter.it.Prev()
			}
		}

		if !hasItem {
			// If no items were found above or on the upper bound, we can move to the end of the
			// BTree.
			hasItem = iter.it.Last()
		}

		if !hasItem {
			// If there are no items found at this point, it means the store is empty.
			return false, nil
		}
	} else {
		var target []byte
		if iter.start != nil && lt(key, iter.start) {
			// We should not yield keys smaller than `start`, so if the given seek-key
			// is smaller than `start`, we should instead seek to `start`.
			target = iter.start
		} else {
			target = key
		}

		hasItem = iter.it.Seek(dsItem{key: target, version: version})
	}

	if !hasItem {
		return false, nil
	}

	iter.loadLatestItem()

	if !iter.valid() {
		return iter.Next()
	}

	return true, nil
}

func (iter *iterator) Close(ctx context.Context) error {
	iter.it.Release()
	return nil
}

func (iter *iterator) loadLatestItem() {
	previousItem := iter.it.Item()
	for iter.it.Next() {
		// Scan through until we reach the next key.
		// It doesn't matter if it is deleted or not.
		if !bytes.Equal(previousItem.key, iter.it.Item().key) {
			iter.it.Prev()
			break
		}
	}
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

// greater than or equal to (a >= b)
func gte(a, b []byte) bool {
	return bytes.Compare(a, b) > -1
}

// less than (a < b)
func lt(a, b []byte) bool {
	return bytes.Compare(a, b) == -1
}

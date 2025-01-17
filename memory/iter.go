package memory

import (
	"bytes"
	"context"

	"github.com/tidwall/btree"
)

type dsPrefixIter struct {
	ctx     context.Context
	db      *Datastore
	version uint64
	it      *baseIterator
	prefix  []byte
	reverse bool
	curItem *dsItem
}

type dsRangeIter struct {
	ctx     context.Context
	db      *Datastore
	version uint64
	it      *baseIterator
	start   []byte
	end     []byte
	reverse bool
	curItem *dsItem
}

func newPrefixIter(ctx context.Context, db *Datastore, prefix []byte, reverse bool, version uint64) *dsPrefixIter {
	pIter := &dsPrefixIter{
		ctx:     ctx,
		db:      db,
		version: version,
		it:      newBaseIterator(db.values, reverse),
		prefix:  prefix,
		reverse: reverse,
	}
	if reverse {
		if len(prefix) > 0 {
			pIter.Seek(bytesPrefixEnd(prefix))
			// Seek is equal to or greater, and the bytesPrefixEnd is the
			// exact largest value before the prefix is invalid, so we likely
			// don't match any exact key. Therefore the seek will go past
			// our desired prefix, and will need to backup one entry.
			if !validForPrefix(pIter.curItem, prefix) {
				pIter.Next()
			}
		}
	} else {
		pIter.Seek(prefix)
	}

	// if the first key is an exact match to the prefix, skip next
	// since prefix is a *strict* subset prefix
	if pIter.curItem != nil && bytes.Equal(pIter.Key(), prefix) {
		pIter.Next()
	}

	return pIter
}

func (iter *dsPrefixIter) Domain() (start []byte, end []byte) {
	return iter.prefix, iter.prefix
}

func (iter *dsPrefixIter) Valid() bool {
	return validForPrefix(iter.curItem, iter.prefix)
}

func validForPrefix(item *dsItem, prefix []byte) bool {
	if item == nil {
		return false
	}

	return bytes.HasPrefix(item.key, prefix) && !bytes.Equal(item.key, prefix)
}

func (iter *dsPrefixIter) Next() {
	if iter.it.Next() {
		iter.loadLatestItem()
	} else {
		iter.curItem = nil
	}
}

func (iter *dsPrefixIter) Key() []byte {
	if iter.curItem != nil {
		return iter.curItem.key
	}
	return nil
}

func (iter *dsPrefixIter) Value() ([]byte, error) {
	if iter.curItem != nil {
		return iter.curItem.val, nil
	}
	return nil, nil
}

func (iter *dsPrefixIter) Seek(key []byte) {
	// get the correct initial version for the seek
	// if there exists an exact match in keys, use the latest version
	// of that key, otherwise, use the provided DB version
	// TODO this could use some "peek" mechanic instead of a full lookup
	version := iter.version
	result := iter.db.get(key, version)
	if result.key != nil && !result.isDeleted {
		version = result.version
	}

	if iter.it.Seek(dsItem{key: key, version: version}) {
		iter.loadLatestItem()
	} else {
		iter.curItem = nil
	}
}

func (iter *dsPrefixIter) Close(ctx context.Context) error {
	return iter.it.Close()
}

func (iter *dsPrefixIter) loadLatestItem() {
	curItem := iter.it.Item()
	for iter.it.Next() {
		if bytes.Equal(curItem.key, iter.it.Item().key) {
			curItem = iter.it.Item()
			continue
		}
		iter.it.Prev()
		break
	}

	if curItem.isDeleted {
		iter.curItem = nil

		if iter.it.Next() {
			iter.loadLatestItem()
		}
		return
	}
	iter.curItem = &curItem
}

type baseIterator struct {
	it      btree.IterG[dsItem]
	reverse bool
}

func newRangeIter(ctx context.Context, db *Datastore, start, end []byte, reverse bool, version uint64) *dsRangeIter {
	rIter := &dsRangeIter{
		ctx:     ctx,
		db:      db,
		version: version,
		it:      newBaseIterator(db.values, reverse),
		start:   start,
		end:     end,
		reverse: reverse,
	}

	if len(end) > 0 && reverse {
		rIter.Seek(end)
		// end in range is exclusive, so we need to make sure to iterate
		// back till we are *before* the end target
		for rIter.curItem != nil && bytes.Compare(rIter.Key(), end) >= 0 {
			rIter.Next()
		}
	} else if len(start) > 0 && !reverse {
		rIter.Seek(start)
	}

	rIter.loadLatestItem()

	return rIter
}

func (iter *dsRangeIter) Domain() (start []byte, end []byte) {
	return iter.start, iter.end
}

func (iter *dsRangeIter) Valid() bool {
	if iter.curItem == nil {
		return false
	}

	if len(iter.end) > 0 && !lt(iter.curItem.key, iter.end) {
		return false
	}

	return gte(iter.curItem.key, iter.start)
}

func (iter *dsRangeIter) Next() {
	if iter.it.Next() {
		iter.loadLatestItem()
	} else {
		iter.curItem = nil
	}
}

func (iter *dsRangeIter) Key() []byte {
	return iter.curItem.key
}

func (iter *dsRangeIter) Value() ([]byte, error) {
	return iter.curItem.val, nil
}

func (iter *dsRangeIter) Seek(key []byte) {
	// get the correct initial version for the seek
	// if there exists an exact match in keys, use the latest version
	// of that key, otherwise, use the provided DB version
	// TODO this could use some "peek" mechanic instead of a full lookup
	version := iter.version
	result := iter.db.get(key, version)
	if result.key != nil && !result.isDeleted {
		version = result.version
	}

	if iter.it.Seek(dsItem{key: key, version: version}) {
		iter.loadLatestItem()
	} else {
		iter.curItem = nil
	}
}

func (iter *dsRangeIter) Close(ctx context.Context) error {
	return iter.it.Close()
}

// loadLatestItem gets the latest version of the current key
func (iter *dsRangeIter) loadLatestItem() {
	curItem := iter.it.Item()
	for iter.it.Next() {
		if bytes.Equal(curItem.key, iter.it.Item().key) {
			curItem = iter.it.Item()
			continue
		}
		iter.it.Prev()
		break
	}

	if curItem.isDeleted {
		iter.curItem = nil

		if iter.it.Next() {
			iter.loadLatestItem()
		}
		return
	}
	iter.curItem = &curItem
}

func newBaseIterator(bt *btree.BTreeG[dsItem], reverse bool) *baseIterator {
	bit := bt.Iter()
	if reverse {
		bit.Last()
	} else {
		bit.First()
	}

	return &baseIterator{
		it:      bit,
		reverse: reverse,
	}
}

func (bit *baseIterator) Next() bool {
	if bit.reverse {
		return bit.it.Prev()
	}
	return bit.it.Next()
}

func (bit *baseIterator) Prev() bool {
	if bit.reverse {
		return bit.it.Next()
	}
	return bit.it.Prev()
}

func (bit *baseIterator) Item() dsItem {
	return bit.it.Item()
}

func (bit *baseIterator) Seek(key dsItem) bool {
	return bit.it.Seek(key)
}

func (bit *baseIterator) Close() error {
	bit.it.Release()
	return nil
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

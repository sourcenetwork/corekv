//go:build js

package indexed_db

import (
	"bytes"
	"sync"
	"syscall/js"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/goji"
	"github.com/sourcenetwork/goji/indexed_db"
)

var _ corekv.Iterator = (*iterator)(nil)

type iterator struct {
	// key contains the last cursor key
	key []byte
	// value contains the last cursor value
	value []byte
	// cursor contains the last value returned from the iterator
	cursor *indexed_db.CursorValue
	// successCallback is called when a cursor "continue" call succeeds
	successCallback js.Func
	// errorCallback is called when a cursor "continue" call fails
	errorCallback js.Func
	// wait is used to wait for the success or error callback to be called
	wait sync.WaitGroup
	// options that the iterator was initialized with
	opts corekv.IterOptions
	// object store the iterator was created from
	store indexed_db.ObjectStoreValue
}

func newIterator(store indexed_db.ObjectStoreValue, opts corekv.IterOptions) corekv.Iterator {
	iterator := &iterator{opts: opts, store: store}
	iterator.init()
	return iterator
}

func (i *iterator) init() {
	var keyRange js.Value = js.Undefined()
	switch {
	case len(i.opts.Prefix) > 0:
		lower := toBytesValue(i.opts.Prefix)
		keyRange = js.Value(indexed_db.KeyRange.LowerBound(lower, false))

	case len(i.opts.Start) > 0 && len(i.opts.End) > 0:
		lower := toBytesValue(i.opts.Start)
		upper := toBytesValue(i.opts.End)
		keyRange = js.Value(indexed_db.KeyRange.Bound(lower, upper, false, true))

	case len(i.opts.Start) > 0:
		lower := toBytesValue(i.opts.Start)
		keyRange = js.Value(indexed_db.KeyRange.LowerBound(lower, false))

	case len(i.opts.End) > 0:
		upper := toBytesValue(i.opts.End)
		keyRange = js.Value(indexed_db.KeyRange.UpperBound(upper, true))
	}

	var direction string
	if i.opts.Reverse {
		direction = indexed_db.CursorDirectionPrev
	} else {
		direction = indexed_db.CursorDirectionNext
	}

	var request indexed_db.RequestValue[indexed_db.CursorValue]
	i.successCallback = goji.EventListener(func(event goji.EventValue) {
		result := request.Result()
		if js.Value(result).IsNull() {
			i.cursor = nil
		} else {
			i.cursor = &result
		}
		i.wait.Done()
	})
	i.errorCallback = goji.EventListener(func(event goji.EventValue) {
		i.cursor = nil
		i.wait.Done()
	})

	if i.opts.KeysOnly {
		request = i.store.OpenKeyCursor(keyRange, direction)
	} else {
		request = i.store.OpenCursor(keyRange, direction)
	}

	i.wait.Add(1)
	request.EventTarget().AddEventListener(indexed_db.ErrorEvent, i.errorCallback.Value)
	request.EventTarget().AddEventListener(indexed_db.SuccessEvent, i.successCallback.Value)
	i.wait.Wait()
}

func (i *iterator) Next() (bool, error) {
	if i.cursor == nil {
		i.key = []byte(nil)
		i.value = []byte(nil)
		return false, nil
	}
	i.key = fromBytesValue(i.cursor.Key())
	i.value = fromBytesValue(i.cursor.Value())
	// advance to next key
	i.wait.Add(1)
	i.cursor.Continue()
	i.wait.Wait()
	return true, nil
}

func (i *iterator) Key() []byte {
	return i.key
}

func (i *iterator) Value() ([]byte, error) {
	return i.value, nil
}

func (i *iterator) Seek(key []byte) (bool, error) {
	if i.cursor == nil {
		return false, nil
	}
	// do nothing if the cursor is already positioned at the key
	if bytes.Equal(key, fromBytesValue(i.cursor.Key())) {
		return i.Next()
	}
	var found = true
	// see https://developer.mozilla.org/en-US/docs/Web/API/IDBCursor/continue#exceptions
	defer func() { found = recover() == nil }()
	i.wait.Add(1)
	i.cursor.Continue(toBytesValue(key))
	i.wait.Wait()
	if i.cursor == nil {
		i.key = []byte(nil)
		i.value = []byte(nil)
	} else {
		i.key = fromBytesValue(i.cursor.Key())
		i.value = fromBytesValue(i.cursor.Value())
	}
	return found, nil
}

func (i *iterator) Close() error {
	i.successCallback.Release()
	i.errorCallback.Release()
	return nil
}

func (i *iterator) Reset() {
	i.successCallback.Release()
	i.errorCallback.Release()
	i.init()
}

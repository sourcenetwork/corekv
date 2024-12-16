//go:build js

package indexed_db

import (
	"bytes"
	"context"
	"sync"
	"syscall/js"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/goji"
	"github.com/sourcenetwork/goji/indexed_db"
)

var _ corekv.Iterator = (*iterator)(nil)

type iterator struct {
	cursor          *indexed_db.CursorValue
	successCallback js.Func
	errorCallback   js.Func
	wait            sync.WaitGroup
	opts            corekv.IterOptions
}

func newIterator(store indexed_db.ObjectStoreValue, opts corekv.IterOptions) *iterator {
	var keyRange js.Value
	switch {
	case len(opts.Prefix) > 0:
		lower := toBytesValue(opts.Prefix)
		keyRange = js.Value(indexed_db.KeyRange.LowerBound(lower, true))

	case len(opts.Start) > 0 && len(opts.End) > 0:
		lower := toBytesValue(opts.Start)
		upper := toBytesValue(opts.End)
		keyRange = js.Value(indexed_db.KeyRange.Bound(lower, upper, true, true))

	case len(opts.Start) > 0:
		lower := toBytesValue(opts.Start)
		keyRange = js.Value(indexed_db.KeyRange.LowerBound(lower, true))

	case len(opts.End) > 0:
		upper := toBytesValue(opts.End)
		keyRange = js.Value(indexed_db.KeyRange.UpperBound(upper, true))

	default:
		keyRange = js.Undefined()
	}

	var direction string
	if opts.Reverse {
		direction = indexed_db.CursorDirectionPrev
	} else {
		direction = indexed_db.CursorDirectionNext
	}

	iterator := &iterator{
		opts: opts,
	}

	var request indexed_db.RequestValue[indexed_db.CursorValue]
	iterator.successCallback = goji.EventListener(func(event goji.EventValue) {
		result := request.Result()
		if js.Value(result).IsNull() {
			iterator.cursor = nil
		} else {
			iterator.cursor = &result
		}
		iterator.wait.Done()
	})
	iterator.errorCallback = goji.EventListener(func(event goji.EventValue) {
		iterator.wait.Done()
	})

	if opts.KeysOnly {
		request = store.OpenKeyCursor(keyRange, direction)
	} else {
		request = store.OpenCursor(keyRange, direction)
	}

	iterator.wait.Add(1)
	request.EventTarget().AddEventListener(indexed_db.ErrorEvent, iterator.errorCallback.Value)
	request.EventTarget().AddEventListener(indexed_db.SuccessEvent, iterator.successCallback.Value)
	iterator.wait.Done()

	return iterator
}

func (i *iterator) Domain() (start []byte, end []byte) {
	if len(i.opts.Prefix) > 0 {
		return i.opts.Prefix, i.opts.Prefix
	}
	return i.opts.Start, i.opts.End
}

func (i *iterator) Valid() bool {
	if i.cursor == nil {
		return false
	}
	if len(i.opts.Prefix) > 0 {
		return bytes.HasPrefix(i.Key(), i.opts.Prefix)
	}
	return true
}

func (i *iterator) Next() {
	if i.cursor == nil {
		return
	}
	i.wait.Add(1)
	i.cursor.Continue()
	i.wait.Done()
}

func (i *iterator) Key() []byte {
	if i.cursor == nil {
		return []byte(nil)
	}
	return fromBytesValue(i.cursor.Key())
}

func (i *iterator) Value() []byte {
	if i.cursor == nil {
		return []byte(nil)
	}
	return fromBytesValue(i.cursor.Value())
}

func (i *iterator) Seek(key []byte) {
	if i.cursor == nil {
		return
	}
	i.wait.Add(1)
	i.cursor.Continue(toBytesValue(key))
	i.wait.Done()
}

func (i *iterator) Close(ctx context.Context) error {
	i.cursor = nil
	i.successCallback.Release()
	i.errorCallback.Release()
	return nil
}

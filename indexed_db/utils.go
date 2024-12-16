//go:build js

package indexed_db

import (
	"syscall/js"

	"github.com/sourcenetwork/goji"
)

// toBytesValue converts the given bytes into a JavaScript
// value that can be used as an indexed db key or value.
func toBytesValue(data []byte) js.Value {
	arr := goji.Uint8ArrayFromBytes(data)
	return js.Value(arr)
}

// fromBytesValue converts the given JavaScript value into
// a byte slice.
func fromBytesValue(val js.Value) []byte {
	if val.IsNull() {
		return []byte(nil)
	}
	arr := goji.Uint8Array.New(val)
	return goji.BytesFromUint8Array(arr)
}

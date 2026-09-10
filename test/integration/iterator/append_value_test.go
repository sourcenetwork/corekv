package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
)

func TestIteratorAppendValue(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k3"), nil),
			action.Set([]byte("k4"), []byte("v4")),
			action.Set([]byte("k2"), []byte("v2")),
			&action.IterateAppend{
				Expected: []action.KeyValue{
					{Key: []byte("k1"), Value: []byte("v1")},
					{Key: []byte("k2"), Value: []byte("v2")},
					{Key: []byte("k3"), Value: nil},
					{Key: []byte("k4"), Value: []byte("v4")},
				},
			},
		},
	}

	test.Execute(t)
}

// TestIteratorAppendValueIntoNonEmptyBuffer documents that `AppendValue` appends to the
// given buffer, leaving any existing content in place.
func TestIteratorAppendValueIntoNonEmptyBuffer(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k2"), []byte("a much longer value, to force the buffer to grow")),
			action.Set([]byte("k3"), []byte("v3")),
			&action.IterateAppend{
				Prefix: []byte("prefix:"),
				Expected: []action.KeyValue{
					{Key: []byte("k1"), Value: []byte("v1")},
					{Key: []byte("k2"), Value: []byte("a much longer value, to force the buffer to grow")},
					{Key: []byte("k3"), Value: []byte("v3")},
				},
			},
		},
	}

	test.Execute(t)
}

func TestIteratorAppendValueReverse(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k2"), []byte("v2")),
			action.Set([]byte("k3"), []byte("v3")),
			&action.IterateAppend{
				IterOptions: corekv.IterOptions{
					Reverse: true,
				},
				Expected: []action.KeyValue{
					{Key: []byte("k3"), Value: []byte("v3")},
					{Key: []byte("k2"), Value: []byte("v2")},
					{Key: []byte("k1"), Value: []byte("v1")},
				},
			},
		},
	}

	test.Execute(t)
}

// TestIteratorAppendValueKeysOnly documents that `AppendValue` returns the given buffer
// unchanged when the iterator was created with `KeysOnly`, mirroring `Value` returning
// nil.  The action asserts the agreement between the two for every item.
func TestIteratorAppendValueKeysOnly(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k2"), []byte("v2")),
			&action.IterateAppend{
				IterOptions: corekv.IterOptions{
					KeysOnly: true,
				},
				Expected: []action.KeyValue{
					{Key: []byte("k1"), Value: nil},
					{Key: []byte("k2"), Value: nil},
				},
			},
		},
	}

	test.Execute(t)
}

package iterator

import (
	"errors"
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestIteratorBorrowValue(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k3"), nil),
			action.Set([]byte("k4"), []byte("v4")),
			action.Set([]byte("k2"), []byte("v2")),
			&action.IterateBorrow{
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

func TestIteratorBorrowValueReverse(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k2"), []byte("v2")),
			action.Set([]byte("k3"), []byte("v3")),
			&action.IterateBorrow{
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

// TestIteratorBorrowValueKeysOnly documents that `BorrowValue` calls its callback with
// nil when the iterator was created with `KeysOnly`, mirroring `Value` returning nil.
//
// The memory store is excluded as it ignores the `KeysOnly` option entirely:
// https://github.com/sourcenetwork/corekv/issues/33
func TestIteratorBorrowValueKeysOnly(t *testing.T) {
	test := &integration.Test{
		Excludes: []multiplier.Name{
			multiplier.Memory,
		},
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k2"), []byte("v2")),
			&action.IterateBorrow{
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

// TestIteratorBorrowValueCallbackError documents that an error returned by the callback
// is returned by `BorrowValue` unchanged.
func TestIteratorBorrowValueCallbackError(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			&action.IterateBorrow{
				CallbackError: errors.New("an error from the callback"),
				Expected:      []action.KeyValue{},
			},
		},
	}

	test.Execute(t)
}

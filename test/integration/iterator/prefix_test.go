package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
)

func TestIteratorPrefix(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("1"), []byte("v1")),
			action.Set([]byte("k3"), nil),
			action.Set([]byte("4"), []byte("v4")),
			action.Set([]byte("k2"), []byte("v2")),
			&action.Iterate{
				IterOptions: corekv.IterOptions{
					Prefix: []byte("k"),
				},
				Expected: []action.KeyValue{
					// `1` must not be yielded
					{Key: []byte("k2"), Value: []byte("v2")},
					{Key: []byte("k3"), Value: nil},
					// `4` must not be yielded
				},
			},
		},
	}

	test.Execute(t)
}

func TestIteratorPrefix_DoesNotReturnSelf(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k"), []byte("v")),
			action.Set([]byte("k1"), []byte("v1")),
			&action.Iterate{
				IterOptions: corekv.IterOptions{
					Prefix: []byte("k"),
				},
				Expected: []action.KeyValue{
					{Key: []byte("k"), Value: []byte("v")},
					{Key: []byte("k1"), Value: []byte("v1")},
				},
			},
		},
	}

	test.Execute(t)
}

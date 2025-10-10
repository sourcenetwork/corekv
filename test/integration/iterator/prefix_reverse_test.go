package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestIteratorPrefixReverse(t *testing.T) {
	test := &integration.Test{
		Excludes: []multiplier.Name{
			// The chunk store only supports keys of a constant length
			multiplier.Chunk,
		},
		Actions: []action.Action{
			action.Set([]byte("1"), []byte("v1")),
			action.Set([]byte("k3"), nil),
			action.Set([]byte("4"), []byte("v4")),
			action.Set([]byte("k2"), []byte("v2")),
			&action.Iterate{
				IterOptions: corekv.IterOptions{
					Reverse: true,
					Prefix:  []byte("k"),
				},
				Expected: []action.KeyValue{
					// `4` must not be yielded
					{Key: []byte("k3"), Value: nil},
					{Key: []byte("k2"), Value: []byte("v2")},
					// `1` must not be yielded
				},
			},
		},
	}

	test.Execute(t)
}

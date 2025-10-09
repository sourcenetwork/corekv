package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestIteratorPrefixDelete(t *testing.T) {
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
			action.Delete([]byte("k2")),
			&action.Iterate{
				IterOptions: corekv.IterOptions{
					Prefix: []byte("k"),
				},
				Expected: []action.KeyValue{
					// `1` must not be yielded
					// `k2` must not be yielded
					{Key: []byte("k3"), Value: nil},
					// `4` must not be yielded
				},
			},
		},
	}

	test.Execute(t)
}

func TestIteratorPrefixDelete_DeleteLastItem(t *testing.T) {
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
			action.Delete([]byte("k3")),
			&action.Iterate{
				IterOptions: corekv.IterOptions{
					Prefix: []byte("k"),
				},
				Expected: []action.KeyValue{
					// `1` must not be yielded
					{Key: []byte("k2"), Value: []byte("v2")},
					// `k3` must not be yielded
					// `4` must not be yielded
				},
			},
		},
	}

	test.Execute(t)
}

func TestIteratorPrefixDelete_DeleteOnlyItem(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k4"), []byte("v4")),
			action.Delete([]byte("k4")),
			&action.Iterate{
				IterOptions: corekv.IterOptions{
					Prefix: []byte("k"),
				},
				Expected: []action.KeyValue{
					// `k4` should not be returned
				},
			},
		},
	}

	test.Execute(t)
}

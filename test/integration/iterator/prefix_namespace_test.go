package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestIteratorPrefixNamespace_ExcludesItemsOutsideOfNamespace(t *testing.T) {
	test := &integration.Test{
		Excludes: []multiplier.Name{
			// The chunk store will break if namespaced after writing values to it
			multiplier.Chunk,
		},
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k4"), []byte("v4")),
			action.Namespace([]byte("namespace")),
			action.Set([]byte("k2"), []byte("v2")),
			action.Set([]byte("k3"), []byte("v3")),
			&action.Iterate{
				IterOptions: corekv.IterOptions{
					Prefix: []byte("k"),
				},
				Expected: []action.KeyValue{
					{Key: []byte("k2"), Value: []byte("v2")},
					{Key: []byte("k3"), Value: []byte("v3")},
				},
			},
		},
	}

	test.Execute(t)
}

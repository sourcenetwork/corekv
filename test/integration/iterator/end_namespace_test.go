package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
)

func TestIteratorEndNamespace_ExcludesItemsOutsideOfNamespace(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k5"), []byte("v5")),
			action.Namespace([]byte("namespace")),
			action.Set([]byte("k2"), []byte("v2")),
			action.Set([]byte("k3"), []byte("v3")),
			action.Set([]byte("k4"), []byte("v4")),
			&action.Iterate{
				IterOptions: corekv.IterOptions{
					End: []byte("k4"),
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

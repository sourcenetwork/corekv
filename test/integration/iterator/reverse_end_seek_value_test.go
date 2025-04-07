package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestIteratorReverseEndSeekValue(t *testing.T) {
	test := &integration.Test{
		Excludes: []multiplier.Name{
			multiplier.IndexedDB,
		},
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k3"), nil),
			action.Set([]byte("k4"), []byte("v4")),
			action.Set([]byte("k2"), []byte("v2")),
			&action.Iterator{
				IterOptions: corekv.IterOptions{
					Reverse: true,
					End:     []byte("k3"),
				},
				ChildActions: []action.IteratorAction{
					action.Seek([]byte("k4"), true),
					action.Value([]byte("v2")),
				},
			},
		},
	}

	test.Execute(t)
}

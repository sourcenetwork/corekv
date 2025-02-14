package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
)

func TestIteratorTxnNextValue(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k3"), []byte("v3")),
			action.Set([]byte("k5"), []byte("v5")),
			action.NewTxn(),
			action.WithTxn(action.Set([]byte("k2"), []byte("v2"))),
			action.WithTxn(action.Set([]byte("k4"), []byte("v4"))),
			action.WithTxn(&action.Iterator{
				ChildActions: []action.IteratorAction{
					action.Next(true),
					action.Value([]byte("v1")),
					action.Next(true),
					action.Value([]byte("v2")),
					action.Next(true),
					action.Value([]byte("v3")),
					action.Next(true),
					action.Value([]byte("v4")),
					action.Next(true),
					action.Value([]byte("v5")),
					action.Next(false),
				},
			}),
		},
	}

	test.Execute(t)
}

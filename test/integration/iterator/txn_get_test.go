package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
)

func TestIteratorTxnGetWhileOpen(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k2"), []byte("v2")),
			action.NewTxn(),
			action.WithTxn(&action.Iterator{
				ChildActions: []action.IteratorAction{
					action.Next(true),
					action.Value([]byte("v1")),
					action.WhileOpen(action.Get([]byte("k2"), []byte("v2"))),
				},
			}),
		},
	}

	test.Execute(t)
}

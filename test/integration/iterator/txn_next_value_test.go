package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
	"github.com/sourcenetwork/corekv/test/multiplier"
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

func TestIteratorTxnNextValue_WithConcurrentAddition(t *testing.T) {
	test := &integration.Test{
		Excludes: []string{
			// LevelDB can only handle one transaction at a time.
			// This test is designed to verify that the iterator reflects changes made by
			// concurrent transactions, which is not applicable to LevelDB.
			multiplier.Level,
		},
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k3"), []byte("v3")),
			action.Set([]byte("k5"), []byte("v5")),
			action.NewTxn(),
			action.WithTxn(action.Set([]byte("k2"), []byte("v2"))),
			action.WithTxn(action.Set([]byte("k4"), []byte("v4"))),
			action.Set([]byte("k4"), []byte("v44")),
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

func TestIteratorTxnNextValue_WithConcurrentUpdate(t *testing.T) {
	test := &integration.Test{
		Excludes: []string{
			// LevelDB can only handle one transaction at a time.
			// This test is designed to verify that the iterator reflects changes made by
			// concurrent transactions, which is not applicable to LevelDB.
			multiplier.Level,
		},
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k3"), []byte("v3")),
			action.Set([]byte("k5"), []byte("v5")),
			action.NewTxn(),
			action.WithTxn(action.Set([]byte("k2"), []byte("v2"))),
			action.WithTxn(action.Set([]byte("k4"), []byte("v4"))),
			action.Set([]byte("k3"), []byte("v33")),
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

package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestIteratorCloseNextValue_NoTxn(t *testing.T) {
	test := &integration.Test{
		Excludes: []multiplier.Name{
			// Test behaviour varies a bit with the txn multipliers at the moment,
			// with the stores all failing in slightly different ways.
			// https://github.com/sourcenetwork/corekv/issues/68
			multiplier.TxnDiscard,
			multiplier.TxnCommit,
			multiplier.TxnMulti,
		},
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			&action.Iterator{
				ChildActions: []action.IteratorAction{
					action.Next(true),
					action.CloseRoot(),
					action.Value([]byte("v1")),
				},
			},
		},
	}

	test.Execute(t)
}

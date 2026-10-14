package integration

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestTxnTooBig_BadgerStore_ReturnsErrTxnTooBig(t *testing.T) {
	test := &Test{
		Includes: []string{
			multiplier.Badger,
		},
		Actions: []action.Action{
			action.FillTxn(1024, "transaction is too big"),
		},
	}

	test.Execute(t)
}

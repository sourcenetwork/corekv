package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
)

func TestIteratorCloseNext(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			&action.Iterator{
				ChildActions: []action.IteratorAction{
					action.CloseRoot(),
					action.NextE(corekv.ErrDBClosed.Error()),
				},
			},
		},
	}

	test.Execute(t)
}

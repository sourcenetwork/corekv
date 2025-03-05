package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
)

func TestIteratorCloseSeek(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			&action.Iterator{
				ChildActions: []action.IteratorAction{
					action.CloseRoot(),
					action.SeekE([]byte("any key"), corekv.ErrDBClosed.Error()),
				},
			},
		},
	}

	test.Execute(t)
}

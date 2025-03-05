package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
)

func TestIteratorClose(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Close(),
			&action.Iterate{
				ExpectedError: corekv.ErrDBClosed.Error(),
			},
		},
	}

	test.Execute(t)
}

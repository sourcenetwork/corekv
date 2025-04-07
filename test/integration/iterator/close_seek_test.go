package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestIteratorCloseSeek(t *testing.T) {
	test := &integration.Test{
		Excludes: []multiplier.Name{
			// The connection is not actually closed until
			// all transactions created using this connection
			// are complete. No new transactions can be created
			// for this connection once this method is called.
			// Methods that create transactions throw an exception
			// if a closing operation is pending.
			multiplier.IndexedDB,
		},
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

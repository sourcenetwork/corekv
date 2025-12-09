package integration

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
)

func TestDeleteClose_DeleteOnClosedStore_Errors(t *testing.T) {
	test := &Test{
		Actions: []action.Action{
			action.Close(),
			action.DeleteE([]byte("does not matter"), "datastore closed"),
		},
	}

	test.Execute(t)
}

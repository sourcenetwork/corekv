package integration

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
)

func TestSetClose_SetOnClosedStore_Errors(t *testing.T) {
	test := &Test{
		Actions: []action.Action{
			action.Close(),
			action.SetE([]byte("does not matter"), []byte("does not matter"), "datastore closed"),
		},
	}

	test.Execute(t)
}

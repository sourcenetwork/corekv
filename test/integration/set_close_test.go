package integration

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestSetClose_MemoryStoreSetOnClosedStore_Errors(t *testing.T) {
	test := &Test{
		Includes: []string{
			multiplier.Memory,
		},
		Actions: []action.Action{
			action.Close(),
			action.SetE([]byte("does not matter"), []byte("does not matter"), "datastore closed"),
		},
	}

	test.Execute(t)
}

func TestSetClose_BadgerStoreSetOnClosedStore_Errors(t *testing.T) {
	test := &Test{
		Includes: []string{
			multiplier.Badger,
		},
		Actions: []action.Action{
			action.Close(),
			action.SetE([]byte("does not matter"), []byte("does not matter"), "Writes are blocked, possibly due to DropAll or Close"),
		},
	}

	test.Execute(t)
}

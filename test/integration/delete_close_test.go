package integration

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestDeleteClose_MemoryStoreDeleteOnClosedStore_Errors(t *testing.T) {
	test := &Test{
		Includes: []string{
			multiplier.Memory,
		},
		Actions: []action.Action{
			action.Close(),
			action.DeleteE([]byte("does not matter"), "datastore closed"),
		},
	}

	test.Execute(t)
}

func TestDeleteClose_BadgerStoreDeleteOnClosedStore_Errors(t *testing.T) {
	test := &Test{
		Includes: []string{
			multiplier.Badger,
		},
		Excludes: []string{
			multiplier.Chunk,
		},
		Actions: []action.Action{
			action.Close(),
			action.DeleteE([]byte("does not matter"), "Writes are blocked, possibly due to DropAll or Close"),
		},
	}

	test.Execute(t)
}

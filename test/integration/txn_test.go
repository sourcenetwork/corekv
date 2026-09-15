package integration

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestTxnCommit_UpdateAfterRead(t *testing.T) {
	test := &Test{
		Excludes: []multiplier.Name{
			multiplier.Chunk,
			multiplier.Level,
		},
		Actions: []action.Action{
			action.Set([]byte("key"), []byte("old")),
			action.NewTxn(),
			action.WithTxn(action.Get([]byte("key"), []byte("old"))),
			action.WithTxn(action.Set([]byte("other"), []byte("value"))),
			action.Set([]byte("key"), []byte("new")),
			action.CommitE(corekv.ErrTxnConflict.Error()),
		},
	}

	test.Execute(t)
}

func TestTxnCommit_InsertAfterMissingRead(t *testing.T) {
	test := &Test{
		Excludes: []multiplier.Name{
			multiplier.Chunk,
			multiplier.Level,
		},
		Actions: []action.Action{
			action.NewTxn(),
			action.WithTxn(action.GetE([]byte("key"), corekv.ErrNotFound.Error())),
			action.WithTxn(action.Set([]byte("other"), []byte("value"))),
			action.Set([]byte("key"), []byte("new")),
			action.CommitE(corekv.ErrTxnConflict.Error()),
		},
	}

	test.Execute(t)
}

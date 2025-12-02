package integration

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

func TestSetDropAllHas(t *testing.T) {
	test := &Test{
		Excludes: []multiplier.Name{
			// The keys seem to persist when
			// using transactions via context.
			// I'm not quite sure why this would happen.
			multiplier.TxnContext,
		},
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k2"), []byte("v2")),
			action.DropAll(),
			action.Has([]byte("k1"), false),
			action.Has([]byte("k2"), false),
		},
	}

	test.Execute(t)
}

package action

import (
	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/state"
	"github.com/stretchr/testify/require"
)

// DropAllItems action will drop all the items in the store if the store
// supports it.  Otherwise skips the test.
type DropAllItems struct{}

var _ Action = (*DropAllItems)(nil)

// Cancel returns a new [*CancelCtx] action that will cancel the state's context when executed.
func DropAll() *DropAllItems {
	return &DropAllItems{}
}

func (a *DropAllItems) Execute(s *state.State) {
	dropable, ok := s.Store.(corekv.Dropable)
	if !ok {
		s.T.Skipf("Store does not support DropAll, test is irrelevant")
	}

	err := dropable.DropAll()
	require.NoError(s.T, err)
}

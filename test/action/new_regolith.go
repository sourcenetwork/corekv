//go:build regolith

package action

import (
	"github.com/sourcenetwork/corekv/regolith"
	"github.com/sourcenetwork/corekv/test/state"
	"github.com/stretchr/testify/require"
)

func (a *NewRegolithStore) Execute(s *state.State) {
	store, err := regolith.NewDatastore(s.T.TempDir())
	require.NoError(s.T, err)

	s.Rootstore = store
	s.Store = store
}

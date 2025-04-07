package action

import (
	"github.com/sourcenetwork/corekv/indexed_db"
	"github.com/sourcenetwork/corekv/test/state"
	"github.com/stretchr/testify/require"
)

func (a *NewBadgerStore) Execute(s *state.State) {
	s.T.Fatalf("badger store is not supported in JS builds")
}

func (a *NewIndexedDBStore) Execute(s *state.State) {
	store, err := indexed_db.NewDatastore(s.T.Name())
	require.NoError(s.T, err)

	s.Rootstore = store
	s.Store = store
}

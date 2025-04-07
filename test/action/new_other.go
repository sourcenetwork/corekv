//go:build !js

package action

import (
	badgerds "github.com/dgraph-io/badger/v4"
	"github.com/sourcenetwork/corekv/badger"
	"github.com/sourcenetwork/corekv/test/state"
	"github.com/stretchr/testify/require"
)

func (a *NewBadgerStore) Execute(s *state.State) {
	// This repo is not currently too concerned with badger-file vs badger-memory, so we only bother
	// testing with badger-memory at the moment.
	store, err := badger.NewDatastore("", badgerds.DefaultOptions("").WithInMemory(true))
	require.NoError(s.T, err)

	s.Rootstore = store
	s.Store = store
}

func (a *NewIndexedDBStore) Execute(s *state.State) {
	s.T.Fatalf("indexed_db store is not supported in non JS builds")
}

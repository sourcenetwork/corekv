package action

import (
	badgerds "github.com/dgraph-io/badger/v4"
	"github.com/sourcenetwork/corekv/badger"
	"github.com/sourcenetwork/corekv/leveldb"
	"github.com/sourcenetwork/corekv/memory"
	"github.com/sourcenetwork/corekv/test/state"
	"github.com/stretchr/testify/require"
)

// NewStore action will create a new [corekv.Store] when executed.
//
// This action type is a placeholder and will be replaced depending
// on the active complexity multipliers.
type NewStore struct{}

var _ Action = (*NewStore)(nil)

// New creates a new [*NewStore] action that will create a new [corekv.Store] when executed.
//
// The instance returned by this function is a placeholder and will be replaced depending
// on the active complexity multipliers.
func New() *NewStore {
	return &NewStore{}
}

func (a *NewStore) Execute(s *state.State) {
	s.T.Fatalf("Store type multiplier not specified")
}

// NewMemoryStore action will create a new [memory.Datastore] when executed.
type NewMemoryStore struct{}

var _ Action = (*NewMemoryStore)(nil)

func (a *NewMemoryStore) Execute(s *state.State) {
	store := memory.NewDatastore(s.Ctx)

	s.Rootstore = store
	s.Store = store
}

// NewBadgerStore action will create a new badger [corekv.Store] when executed.
type NewBadgerStore struct{}

var _ Action = (*NewBadgerStore)(nil)

func (a *NewBadgerStore) Execute(s *state.State) {
	// This repo is not currently too concerned with badger-file vs badger-memory, so we only bother
	// testing with badger-memory at the moment.
	store, err := badger.NewDatastore("", badgerds.DefaultOptions("").WithInMemory(true))
	require.NoError(s.T, err)

	s.Rootstore = store
	s.Store = store
}

// NewLevelStore action will create a new [leveldb.Datastore] when executed.
type NewLevelStore struct{}

var _ Action = (*NewLevelStore)(nil)

func (a *NewLevelStore) Execute(s *state.State) {
	store, err := leveldb.NewDatastore(s.T.TempDir(), nil)
	require.NoError(s.T, err)

	s.Rootstore = store
	s.Store = store
}

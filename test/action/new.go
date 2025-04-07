package action

import (
	"github.com/sourcenetwork/corekv/memory"
	"github.com/sourcenetwork/corekv/test/state"
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

// NewIndexedDBStore action will create a new [indexed_db.Datastore] when executed.
type NewIndexedDBStore struct{}

var _ Action = (*NewIndexedDBStore)(nil)

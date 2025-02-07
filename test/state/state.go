package state

import (
	"context"
	"testing"

	"github.com/sourcenetwork/corekv"
)

// State holds the current execution state of a set of [action.Actions].
type State struct {
	T         testing.TB
	Ctx       context.Context
	CtxCancel context.CancelFunc

	// The immutable [Options] used to initialize this state.
	Options Options

	// State that may mutate during the actions' execution should live below this line

	// The root [corekv.Store] containing all store state within this execution.
	Rootstore corekv.Store

	// The active [corekv.Store] layer, through which actions should interact with.
	//
	// This must be derived from the Rootstore.  For example it may be a namespace within the
	// Rootstore, or even a transaction of that namespace.
	Store corekv.Store

	Txns []corekv.Txn
}

func (s *State) Clone() *State {
	return &State{
		T:         s.T,
		Ctx:       s.Ctx,
		CtxCancel: s.CtxCancel,
		Options:   s.Options,
		Rootstore: s.Rootstore,
		Store:     s.Store,
		Txns:      append([]corekv.Txn{}, s.Txns...),
	}
}

func (s *State) WithStore(store corekv.Store) *State {
	newState := s.Clone()
	newState.Store = store
	return newState
}

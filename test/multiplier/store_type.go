package multiplier

import (
	"github.com/sourcenetwork/corekv/test/action"
)

func init() {
	Register(&badger{})
	Register(&memory{})
	Register(&level{})
}

const Badger Name = "badger"

// badger represents the badger store complexity multiplier.
//
// Applying the multiplier will replace all [action.NewStore] actions
// with [action.NewBadgerStore] instances.
type badger struct{}

var _ Multiplier = (*badger)(nil)

func (n *badger) Name() Name {
	return Badger
}

func (n *badger) Apply(source action.Actions) action.Actions {
	result := make([]action.Action, len(source))

	for i, sourceAction := range source {
		_, ok := sourceAction.(*action.NewStore)
		if ok {
			result[i] = &action.NewBadgerStore{}
		} else {
			result[i] = sourceAction
		}
	}

	return result
}

const Memory Name = "memory"

// memory represents the memory store complexity multiplier.
//
// Applying the multiplier will replace all [action.NewStore] actions
// with [action.NewMemoryStore] instances.
type memory struct{}

var _ Multiplier = (*memory)(nil)

func (n *memory) Name() Name {
	return Memory
}

func (n *memory) Apply(source action.Actions) action.Actions {
	result := make([]action.Action, len(source))

	for i, sourceAction := range source {
		_, ok := sourceAction.(*action.NewStore)
		if ok {
			result[i] = &action.NewMemoryStore{}
		} else {
			result[i] = sourceAction
		}
	}

	return result
}

const Level Name = "level"

// level represents the leveldb store complexity multiplier.
//
// Applying the multiplier will replace all [action.NewStore] actions
// with [action.NewLevelStore] instances.
type level struct{}

var _ Multiplier = (*level)(nil)

func (n *level) Name() Name {
	return Level
}

func (n *level) Apply(source action.Actions) action.Actions {
	result := make([]action.Action, len(source))

	for i, sourceAction := range source {
		_, ok := sourceAction.(*action.NewStore)
		if ok {
			result[i] = &action.NewLevelStore{}
		} else {
			result[i] = sourceAction
		}
	}

	return result
}

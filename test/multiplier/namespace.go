package multiplier

import "github.com/sourcenetwork/corekv/test/action"

func init() {
	Register(&namespace{})
}

const Namespace Name = "namespace"

// namespace represents the namespacing complexity multiplier.
//
// Applying the multiplier will insert an [action.NamespaceStore] action
// immediately after the last action that creates a new store.
type namespace struct{}

var _ Multiplier = (*namespace)(nil)

func (n *namespace) Name() Name {
	return Namespace
}

func (n *namespace) Apply(source action.Actions) action.Actions {
	lastStoreWrappingIndex := 0
	result := make(action.Actions, len(source)+1)

	for i, a := range source {
		switch a.(type) {
		// TODO this comment is for reviewers, but this was hard to track down, can this be done better?
		case *action.NewStore, *action.NewBadgerStore, *action.NewMemoryStore, *action.NewLevelStore:
			lastStoreWrappingIndex = i
		}
	}

	offset := 0
	for i, a := range source {
		newIndex := i + offset
		result[newIndex] = a

		if i == lastStoreWrappingIndex {
			result[newIndex+1] = action.Namespace([]byte("/example"))
			offset += 1
		}
	}

	return result
}

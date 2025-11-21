package multiplier

import "github.com/sourcenetwork/corekv/test/action"

func init() {
	Register(&chunk{})
}

const Chunk Name = "chunk"

// chunk represents the chunkstore complexity multiplier.
//
// Applying the multiplier will insert an [action.ChunkStore] action
// immediately after the last action that creates a new store.
type chunk struct{}

var _ Multiplier = (*chunk)(nil)

func (n *chunk) Name() Name {
	return Chunk
}

func (n *chunk) Apply(source action.Actions) action.Actions {
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
			// Chunk one byte at a time, to maximize the number of tests chunking values
			result[newIndex+1] = action.Chunk(1)
			offset += 1
		}
	}

	return result
}

package action

import (
	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/state"
	"github.com/stretchr/testify/require"
)

// KeyValue represents a key-value pair in a store.
type KeyValue struct {
	// The Key in which this item is stored.
	Key []byte

	// The value held against the given key.
	Value []byte
}

// Iterate action will iterate through the active store using the given options
// when executed.
type Iterate struct {
	corekv.IterOptions

	// The items expected to be yielded when iterating using the given options.
	//
	// Matching element order is required.
	Expected []KeyValue

	// The expected error message returned whilst iterating.
	ExpectedError string
}

var _ Action = (*Iterate)(nil)

func (a *Iterate) Execute(s *state.State) {
	iterator, err := s.Store.Iterator(s.Ctx, a.IterOptions)
	require.NoError(s.T, err)

	entries := make([]KeyValue, 0)
	for {
		hasValue, err := iterator.Next()
		require.NoError(s.T, err)

		if !hasValue {
			break
		}

		key := iterator.Key()

		value, err := iterator.Value()
		expectError(s, err, a.ExpectedError)

		entries = append(entries, KeyValue{
			Key:   key,
			Value: value,
		})
	}

	err = iterator.Close()
	require.NoError(s.T, err)

	require.Equal(s.T, a.Expected, entries)
}

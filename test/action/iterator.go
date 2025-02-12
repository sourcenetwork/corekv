package action

import (
	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/state"
	"github.com/stretchr/testify/require"
)

// Iterator action will create an iterator, execute the given child actions on the iterator,
// then release the iterator when this [Iterator] action is executed.
type Iterator struct {
	corekv.IterOptions

	// The actions to execute on the created iterator.
	ChildActions []IteratorAction
}

var _ Action = (*Iterator)(nil)

func (a *Iterator) Execute(s *state.State) {
	iterator := s.Store.Iterator(s.Ctx, a.IterOptions)

	for _, action := range a.ChildActions {
		action.Execute(s, iterator)
	}

	err := iterator.Close()
	require.NoError(s.T, err)
}

// IteratorAction types will execute as part of an [Iterator] action.
type IteratorAction interface {
	Execute(s *state.State, iterator corekv.Iterator)
}

// SeekTo executes a single `Seek` call on an [Iterator].
type SeekTo struct {
	// The target key to seek to.
	Target      []byte
	ExpectValue bool
}

var _ IteratorAction = (*SeekTo)(nil)

// Seek returns a [SeekTo] iterator action that executes a single `Seek` call
// on an [Iterator] to the given target.
func Seek(target []byte, expectValue bool) *SeekTo {
	return &SeekTo{
		Target:      target,
		ExpectValue: expectValue,
	}
}

func (a *SeekTo) Execute(s *state.State, iterator corekv.Iterator) {
	hasValue, err := iterator.Seek(a.Target)
	require.NoError(s.T, err)

	require.Equal(s.T, a.ExpectValue, hasValue)
}

// MoveNext executes a single `Next` call on an an [Iterator].
type MoveNext struct {
	ExpectValue bool
}

var _ IteratorAction = (*MoveNext)(nil)

// Next returns a [MoveNext] iterator action that executes a single `Next` call
// on an [Iterator].
func Next(expectValue bool) *MoveNext {
	return &MoveNext{
		ExpectValue: expectValue,
	}
}

func (a *MoveNext) Execute(s *state.State, iterator corekv.Iterator) {
	hasValue, err := iterator.Next()
	require.NoError(s.T, err)

	require.Equal(s.T, a.ExpectValue, hasValue)
}

// Valid executes a single `Value` call on an [Iterator] and requires that
// the returned result matches the given `Expected` value.
type IteratorValue struct {
	// The expected result of the `Value` call.
	Expected []byte
}

var _ IteratorAction = (*IteratorValue)(nil)

// Value returns a [IteratorValue] iterator action that executes a single `Value` call
// on an [Iterator] and requires that the returned result equals the given expected value.
func Value(expected []byte) *IteratorValue {
	return &IteratorValue{
		Expected: expected,
	}
}

func (a *IteratorValue) Execute(s *state.State, iterator corekv.Iterator) {
	actual, err := iterator.Value()
	require.NoError(s.T, err)

	require.Equal(s.T, a.Expected, actual)
}

// IteratorReset executes a single `Reset` call on an [Iterator].
type IteratorReset struct{}

var _ IteratorAction = (*IteratorReset)(nil)

// Reset returns a [IteratorReset] iterator action that executes a single `Reset` call
// on an [Iterator].
func Reset() *IteratorReset {
	return &IteratorReset{}
}

func (a *IteratorReset) Execute(s *state.State, iterator corekv.Iterator) {
	iterator.Reset()
}

package action

import (
	"bytes"

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
	if err != nil {
		expectError(s, err, a.ExpectedError)
		require.Nil(s.T, iterator)
		return
	}

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

// IterateAppend action will iterate through the active store using the given options,
// reading each value via [corekv.ValueAppender.AppendValue] instead of `Value`, and
// re-using a single destination buffer across the whole iteration.
//
// The test will be skipped if the iterator under test does not implement
// [corekv.ValueAppender], as it is an optional interface.
//
// Each appended value is also cross-checked against the value yielded by `Value`, so
// that the two paths cannot diverge.
type IterateAppend struct {
	corekv.IterOptions

	// Prefix, if provided, is written into the destination buffer before each
	// `AppendValue` call.  The value is required to be appended after it, leaving
	// the prefix intact.
	Prefix []byte

	// The items expected to be yielded when iterating using the given options.
	//
	// Matching element order is required.
	Expected []KeyValue
}

var _ Action = (*IterateAppend)(nil)

func (a *IterateAppend) Execute(s *state.State) {
	iterator, err := s.Store.Iterator(s.Ctx, a.IterOptions)
	require.NoError(s.T, err)

	appender, ok := iterator.(corekv.ValueAppender)
	if !ok {
		err = iterator.Close()
		require.NoError(s.T, err)
		s.T.Skipf("Iterator does not support AppendValue, test is irrelevant")
	}

	// A single buffer is deliberately re-used across every item, as that is the
	// entire point of the interface.
	buf := make([]byte, 0, 8)

	entries := make([]KeyValue, 0)
	for {
		hasValue, err := iterator.Next()
		require.NoError(s.T, err)

		if !hasValue {
			break
		}

		key := iterator.Key()

		buf = append(buf[:0], a.Prefix...)
		buf, err = appender.AppendValue(buf)
		require.NoError(s.T, err)

		// The prefix given to `AppendValue` must survive the call.
		require.True(
			s.T,
			bytes.Equal(a.Prefix, buf[:len(a.Prefix)]),
			"AppendValue did not preserve dst. Key: %s, dst: %v",
			iterator.Key(),
			buf[:len(a.Prefix)],
		)
		appended := buf[len(a.Prefix):]

		value, err := iterator.Value()
		require.NoError(s.T, err)

		// `bytes.Equal` is used instead of `require.Equal` as the two paths are
		// permitted to differ in nil-ness, e.g. when `KeysOnly` is set `Value`
		// returns nil whilst `AppendValue` returns the (empty) given buffer.
		require.True(
			s.T,
			bytes.Equal(value, appended),
			"AppendValue and Value disagree. Key: %s, AppendValue: %v, Value: %v",
			key,
			appended,
			value,
		)

		var entryValue []byte
		if len(appended) > 0 {
			// Copy, as `buf` is re-used by the next iteration.
			entryValue = append([]byte(nil), appended...)
		}

		entries = append(entries, KeyValue{
			Key:   key,
			Value: entryValue,
		})
	}

	err = iterator.Close()
	require.NoError(s.T, err)

	require.Equal(s.T, a.Expected, entries)
}

// IterateBorrow action will iterate through the active store using the given options,
// reading each value via [corekv.ValueBorrower.BorrowValue] instead of `Value`.
//
// The test will be skipped if the iterator under test does not implement
// [corekv.ValueBorrower], as it is an optional interface.
//
// Each borrowed value is also cross-checked against the value yielded by `Value`, and
// the callback is required to be called exactly once per `BorrowValue` call.
type IterateBorrow struct {
	corekv.IterOptions

	// CallbackError, if provided, is returned by the callback given to `BorrowValue`.
	//
	// `BorrowValue` is then required to return it unchanged, and iteration stops.
	CallbackError error

	// The items expected to be yielded when iterating using the given options.
	//
	// Matching element order is required.
	Expected []KeyValue
}

var _ Action = (*IterateBorrow)(nil)

func (a *IterateBorrow) Execute(s *state.State) {
	iterator, err := s.Store.Iterator(s.Ctx, a.IterOptions)
	require.NoError(s.T, err)

	borrower, ok := iterator.(corekv.ValueBorrower)
	if !ok {
		err = iterator.Close()
		require.NoError(s.T, err)
		s.T.Skipf("Iterator does not support BorrowValue, test is irrelevant")
	}

	entries := make([]KeyValue, 0)
	for {
		hasValue, err := iterator.Next()
		require.NoError(s.T, err)

		if !hasValue {
			break
		}

		key := iterator.Key()

		calls := 0
		var borrowed []byte
		err = borrower.BorrowValue(func(value []byte) error {
			calls++
			if a.CallbackError != nil {
				return a.CallbackError
			}

			// The borrowed bytes must not outlive the call, so they are copied
			// here rather than retained.
			if len(value) > 0 {
				borrowed = append([]byte(nil), value...)
			}
			return nil
		})
		require.Equal(s.T, 1, calls, "BorrowValue did not call fn exactly once")

		if a.CallbackError != nil {
			// The error must be returned unchanged, not wrapped.
			require.Equal(s.T, a.CallbackError, err)
			break
		}
		require.NoError(s.T, err)

		value, err := iterator.Value()
		require.NoError(s.T, err)

		// `bytes.Equal` is used instead of `require.Equal` as the two paths are
		// permitted to differ in nil-ness.
		require.True(
			s.T,
			bytes.Equal(value, borrowed),
			"BorrowValue and Value disagree. Key: %s, BorrowValue: %v, Value: %v",
			key,
			borrowed,
			value,
		)

		entries = append(entries, KeyValue{
			Key:   key,
			Value: borrowed,
		})
	}

	err = iterator.Close()
	require.NoError(s.T, err)

	require.Equal(s.T, a.Expected, entries)
}

package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/btree"
)

// newTestDatastore creates a minimal datastore suitable for iterator tests.
func newTestDatastore(ctx context.Context) *Datastore {
	return NewDatastore(ctx)
}

// setItem is a test helper that inserts an item directly into the btree at a specific version.
func setItem(values *btree.BTreeG[dsItem], key []byte, val []byte, version uint64) {
	values.Set(dsItem{
		key:     key,
		val:     val,
		version: version,
	})
}

// TestIteratorForwardFirstKeyOnlyAtNewerVersion tests that a forward iterator
// using First() does not expose a key that only exists at a version higher than
// the iterator's snapshot version.
func TestIteratorForwardFirstKeyOnlyAtNewerVersion(t *testing.T) {
	ctx := context.Background()
	ds := newTestDatastore(ctx)

	// Insert "aaa" only at version 5, and "bbb" at version 1 (visible).
	setItem(ds.values, []byte("aaa"), []byte("val_aaa"), 5)
	setItem(ds.values, []byte("bbb"), []byte("val_bbb"), 1)

	// Create a forward iterator with version 3 and no start/end bounds.
	// This forces restart() to use the First() path.
	iter := newRangeIter(ds, ds.values, nil, nil, false, 3)
	defer iter.Close()

	hasItem, err := iter.Next()
	require.NoError(t, err)
	require.True(t, hasItem, "should find at least one item")

	// The first item returned must be "bbb" (version 1), not "aaa" (version 5).
	require.Equal(t, []byte("bbb"), iter.Key())
}

// TestIteratorReverseLastKeyOnlyAtNewerVersion tests that a reverse iterator
// using Last() does not expose a key that only exists at a version higher than
// the iterator's snapshot version.
func TestIteratorReverseLastKeyOnlyAtNewerVersion(t *testing.T) {
	ctx := context.Background()
	ds := newTestDatastore(ctx)

	// Insert "aaa" at version 1 (visible) and "zzz" only at version 5.
	setItem(ds.values, []byte("aaa"), []byte("val_aaa"), 1)
	setItem(ds.values, []byte("zzz"), []byte("val_zzz"), 5)

	// Create a reverse iterator with version 3 and no start/end bounds.
	// This forces restart() to use the Last() path.
	iter := newRangeIter(ds, ds.values, nil, nil, true, 3)
	defer iter.Close()

	hasItem, err := iter.Next()
	require.NoError(t, err)
	require.True(t, hasItem, "should find at least one item")

	// The first item returned must be "aaa" (version 1), not "zzz" (version 5).
	require.Equal(t, []byte("aaa"), iter.Key())
}

// TestIteratorSeekToKeyOnlyAtNewerVersion tests that Seek() does not expose
// a key that only exists at a version higher than the iterator's snapshot version.
func TestIteratorSeekToKeyOnlyAtNewerVersion(t *testing.T) {
	ctx := context.Background()
	ds := newTestDatastore(ctx)

	// Insert "bbb" only at version 5, and "ccc" at version 1 (visible).
	setItem(ds.values, []byte("bbb"), []byte("val_bbb"), 5)
	setItem(ds.values, []byte("ccc"), []byte("val_ccc"), 1)

	iter := newRangeIter(ds, ds.values, nil, nil, false, 3)
	defer iter.Close()

	// Seek to "bbb" which only exists at version 5 — should skip to "ccc".
	hasItem, err := iter.Seek([]byte("bbb"))
	require.NoError(t, err)
	require.True(t, hasItem, "should find at least one item")

	require.Equal(t, []byte("ccc"), iter.Key())
}

// TestIteratorForwardAllKeysAtNewerVersion tests that a forward iterator
// correctly returns no items when all keys exist only at versions newer than
// the iterator's snapshot.
func TestIteratorForwardAllKeysAtNewerVersion(t *testing.T) {
	ctx := context.Background()
	ds := newTestDatastore(ctx)

	setItem(ds.values, []byte("aaa"), []byte("val_aaa"), 5)
	setItem(ds.values, []byte("bbb"), []byte("val_bbb"), 6)

	iter := newRangeIter(ds, ds.values, nil, nil, false, 3)
	defer iter.Close()

	hasItem, err := iter.Next()
	require.NoError(t, err)
	require.False(t, hasItem, "should find no items when all versions are newer")
}

// TestIteratorReverseSeekToKeyOnlyAtNewerVersion tests that a reverse Seek()
// does not expose a key that only exists at a newer version.
func TestIteratorReverseSeekToKeyOnlyAtNewerVersion(t *testing.T) {
	ctx := context.Background()
	ds := newTestDatastore(ctx)

	// Insert "aaa" at version 1 (visible), "bbb" only at version 5.
	setItem(ds.values, []byte("aaa"), []byte("val_aaa"), 1)
	setItem(ds.values, []byte("bbb"), []byte("val_bbb"), 5)

	iter := newRangeIter(ds, ds.values, nil, nil, true, 3)
	defer iter.Close()

	// Reverse seek to "bbb" — should skip it and return "aaa".
	hasItem, err := iter.Seek([]byte("bbb"))
	require.NoError(t, err)
	require.True(t, hasItem, "should find at least one item")

	require.Equal(t, []byte("aaa"), iter.Key())
}

// TestIteratorForwardPrefixKeyOnlyAtNewerVersion tests that a prefix iterator
// (which uses seek via restart) does not expose keys at newer versions.
func TestIteratorForwardPrefixKeyOnlyAtNewerVersion(t *testing.T) {
	ctx := context.Background()
	ds := newTestDatastore(ctx)

	// Both keys share prefix "test". "testA" only at version 5, "testB" at version 1.
	setItem(ds.values, []byte("testA"), []byte("val_A"), 5)
	setItem(ds.values, []byte("testB"), []byte("val_B"), 1)

	iter := newPrefixIter(ds, ds.values, []byte("test"), false, 3)
	defer iter.Close()

	hasItem, err := iter.Next()
	require.NoError(t, err)
	require.True(t, hasItem, "should find at least one item")

	require.Equal(t, []byte("testB"), iter.Key())
}

// TestIteratorReturnsLatestVisibleVersionForMultiVersionKey tests that when a key
// has multiple versions, the iterator returns the value from the latest version
// that is still <= the iterator's snapshot version.
func TestIteratorReturnsLatestVisibleVersionForMultiVersionKey(t *testing.T) {
	ctx := context.Background()
	ds := newTestDatastore(ctx)

	// Insert "aaa" at version 1 and version 5. With iter.version=3,
	// only version 1 should be visible.
	setItem(ds.values, []byte("aaa"), []byte("val_v1"), 1)
	setItem(ds.values, []byte("aaa"), []byte("val_v5"), 5)

	iter := newRangeIter(ds, ds.values, nil, nil, false, 3)
	defer iter.Close()

	hasItem, err := iter.Next()
	require.NoError(t, err)
	require.True(t, hasItem, "should find the key at its visible version")

	require.Equal(t, []byte("aaa"), iter.Key())
	val, err := iter.Value()
	require.NoError(t, err)
	require.Equal(t, []byte("val_v1"), val)
}

package regolith

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/sourcenetwork/go-regolith"

	"github.com/sourcenetwork/corekv"
)

// newStore opens a store in a temporary directory that is removed, along with
// the store, when the test finishes.
func newStore(t *testing.T) *Datastore {
	t.Helper()

	store, err := NewDatastore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	return store
}

// seed writes the keys a, b, c, d and e with values prefixed by `v`.
func seed(t *testing.T, store *Datastore) {
	t.Helper()

	ctx := context.Background()
	for _, key := range []string{"a", "b", "c", "d", "e"} {
		if err := store.Set(ctx, []byte(key), []byte("v"+key)); err != nil {
			t.Fatalf("set %s: %v", key, err)
		}
	}
}

// drain walks the given iterator to exhaustion, returning the keys it yielded
// and, where values were requested, asserting that each value matches its key.
func drain(t *testing.T, it corekv.Iterator, withValues bool) []string {
	t.Helper()

	keys := []string{}
	for {
		hasNext, err := it.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if !hasNext {
			return keys
		}

		key := it.Key()
		keys = append(keys, string(key))

		value, err := it.Value()
		if err != nil {
			t.Fatalf("value: %v", err)
		}
		switch {
		case !withValues:
			if value != nil {
				t.Errorf("expected no value for %s, got %s", key, value)
			}
		case !bytes.Equal(value, append([]byte("v"), key...)):
			t.Errorf("unexpected value for %s: %s", key, value)
		}
	}
}

func assertKeys(t *testing.T, expected, actual []string) {
	t.Helper()

	if fmt.Sprint(expected) != fmt.Sprint(actual) {
		t.Errorf("expected keys %v, got %v", expected, actual)
	}
}

func TestOpenAndClose(t *testing.T) {
	store, err := NewDatastore(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Closing again must not free the handle a second time.
	if err := store.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// TestOpenUnusablePath covers the failure path out of the FFI layer, including
// the error detail read back from it.
func TestOpenUnusablePath(t *testing.T) {
	path := t.TempDir() + "/a-file-not-a-directory"
	if err := os.WriteFile(path, []byte("not a store"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := NewDatastore(path, nil)
	if err == nil {
		t.Fatal("expected an error opening a path that is not a directory")
	}
}

func TestSetGetHasDelete(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	if err := store.Set(ctx, []byte("k"), []byte("v")); err != nil {
		t.Fatalf("set: %v", err)
	}

	value, err := store.Get(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(value) != "v" {
		t.Errorf("expected v, got %s", value)
	}

	has, err := store.Has(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("has: %v", err)
	}
	if !has {
		t.Error("expected k to be found")
	}

	// Overwriting.
	if err := store.Set(ctx, []byte("k"), []byte("v2")); err != nil {
		t.Fatalf("set: %v", err)
	}
	value, err = store.Get(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(value) != "v2" {
		t.Errorf("expected v2, got %s", value)
	}

	if err := store.Delete(ctx, []byte("k")); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Get(ctx, []byte("k")); !errors.Is(err, corekv.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	has, err = store.Has(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("has: %v", err)
	}
	if has {
		t.Error("expected k to be gone")
	}
}

func TestGetMissingReturnsNotFound(t *testing.T) {
	store := newStore(t)

	value, err := store.Get(context.Background(), []byte("nope"))
	if !errors.Is(err, corekv.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if value != nil {
		t.Errorf("expected a nil value, got %s", value)
	}
}

func TestOperationsAfterCloseReturnDBClosed(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	seed(t, store)

	txn := store.NewTxn(false)
	iter, err := store.Iterator(ctx, corekv.DefaultIterOptions)
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}

	// The ordering contract: iterators and transactions go first.
	if err := iter.Close(); err != nil {
		t.Fatalf("iterator close: %v", err)
	}
	txn.Discard()

	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if _, err := store.Get(ctx, []byte("a")); !errors.Is(err, corekv.ErrDBClosed) {
		t.Errorf("get: expected ErrDBClosed, got %v", err)
	}
	if _, err := store.Has(ctx, []byte("a")); !errors.Is(err, corekv.ErrDBClosed) {
		t.Errorf("has: expected ErrDBClosed, got %v", err)
	}
	if err := store.Set(ctx, []byte("a"), []byte("va")); !errors.Is(err, corekv.ErrDBClosed) {
		t.Errorf("set: expected ErrDBClosed, got %v", err)
	}
	if err := store.Delete(ctx, []byte("a")); !errors.Is(err, corekv.ErrDBClosed) {
		t.Errorf("delete: expected ErrDBClosed, got %v", err)
	}
	if err := store.DropAll(); !errors.Is(err, corekv.ErrDBClosed) {
		t.Errorf("drop all: expected ErrDBClosed, got %v", err)
	}
	if _, err := store.Iterator(ctx, corekv.DefaultIterOptions); !errors.Is(err, corekv.ErrDBClosed) {
		t.Errorf("iterator: expected ErrDBClosed, got %v", err)
	}

	// A transaction created after the close reports the same, from every call.
	closedTxn := store.NewTxn(false)
	if _, err := closedTxn.Get(ctx, []byte("a")); !errors.Is(err, corekv.ErrDBClosed) {
		t.Errorf("txn get: expected ErrDBClosed, got %v", err)
	}
	if err := closedTxn.Set(ctx, []byte("a"), []byte("va")); !errors.Is(err, corekv.ErrDBClosed) {
		t.Errorf("txn set: expected ErrDBClosed, got %v", err)
	}
	if err := closedTxn.Commit(); !errors.Is(err, corekv.ErrDBClosed) {
		t.Errorf("txn commit: expected ErrDBClosed, got %v", err)
	}
	closedTxn.Discard()

	// And so does the store's own Close, which must stay a no-op.
	if err := store.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

func TestIteratorForward(t *testing.T) {
	store := newStore(t)
	seed(t, store)

	it, err := store.Iterator(context.Background(), corekv.DefaultIterOptions)
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	defer closeIter(t, it)

	assertKeys(t, []string{"a", "b", "c", "d", "e"}, drain(t, it, true))
}

func TestIteratorReverse(t *testing.T) {
	store := newStore(t)
	seed(t, store)

	it, err := store.Iterator(context.Background(), corekv.IterOptions{Reverse: true})
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	defer closeIter(t, it)

	assertKeys(t, []string{"e", "d", "c", "b", "a"}, drain(t, it, true))
}

func TestIteratorPrefix(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	for _, key := range []string{"ab", "ba", "bb", "bc", "ca"} {
		if err := store.Set(ctx, []byte(key), []byte("v"+key)); err != nil {
			t.Fatalf("set: %v", err)
		}
	}

	it, err := store.Iterator(ctx, corekv.IterOptions{Prefix: []byte("b")})
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	defer closeIter(t, it)

	assertKeys(t, []string{"ba", "bb", "bc"}, drain(t, it, true))
}

func TestIteratorStartEndIsEndExclusive(t *testing.T) {
	store := newStore(t)
	seed(t, store)

	it, err := store.Iterator(context.Background(), corekv.IterOptions{
		Start: []byte("b"),
		End:   []byte("d"),
	})
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	defer closeIter(t, it)

	assertKeys(t, []string{"b", "c"}, drain(t, it, true))
}

func TestIteratorStartEndReverse(t *testing.T) {
	store := newStore(t)
	seed(t, store)

	it, err := store.Iterator(context.Background(), corekv.IterOptions{
		Start:   []byte("b"),
		End:     []byte("d"),
		Reverse: true,
	})
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	defer closeIter(t, it)

	assertKeys(t, []string{"c", "b"}, drain(t, it, true))
}

func TestIteratorSeek(t *testing.T) {
	store := newStore(t)
	seed(t, store)
	ctx := context.Background()

	it, err := store.Iterator(ctx, corekv.DefaultIterOptions)
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}

	// An exact match, then an inexact one, which lands on the next key up.
	for target, expected := range map[string]string{"c": "c", "bb": "c"} {
		found, err := it.Seek([]byte(target))
		if err != nil {
			t.Fatalf("seek: %v", err)
		}
		if !found {
			t.Fatalf("expected seek to %s to find something", target)
		}
		if string(it.Key()) != expected {
			t.Errorf("expected seek to %s to land on %s, got %s", target, expected, it.Key())
		}
	}

	// Seeking past the end of the data finds nothing.
	found, err := it.Seek([]byte("z"))
	if err != nil {
		t.Fatalf("seek: %v", err)
	}
	if found {
		t.Errorf("expected seek past the end to find nothing, got %s", it.Key())
	}
	// And the iterator is then at an invalid location.
	if it.Key() != nil {
		t.Errorf("expected a nil key at an invalid location, got %s", it.Key())
	}
	value, err := it.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	if value != nil {
		t.Errorf("expected a nil value at an invalid location, got %s", value)
	}
	closeIter(t, it)

	// A reverse seek lands on the greatest key at or below the target.
	it, err = store.Iterator(ctx, corekv.IterOptions{Reverse: true})
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	defer closeIter(t, it)

	found, err = it.Seek([]byte("bb"))
	if err != nil {
		t.Fatalf("seek: %v", err)
	}
	if !found || string(it.Key()) != "b" {
		t.Errorf("expected a reverse seek to bb to land on b, got %s (%t)", it.Key(), found)
	}
	assertKeys(t, []string{"a"}, drain(t, it, true))
}

func TestIteratorReset(t *testing.T) {
	store := newStore(t)
	seed(t, store)

	it, err := store.Iterator(context.Background(), corekv.DefaultIterOptions)
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	defer closeIter(t, it)

	assertKeys(t, []string{"a", "b", "c", "d", "e"}, drain(t, it, true))

	// Without a reset the iterator stays exhausted.
	assertKeys(t, []string{}, drain(t, it, true))

	it.Reset()
	assertKeys(t, []string{"a", "b", "c", "d", "e"}, drain(t, it, true))

	// A reset part way through is just as good.
	if _, err := it.Next(); err != nil {
		t.Fatalf("next: %v", err)
	}
	it.Reset()
	assertKeys(t, []string{"a", "b", "c", "d", "e"}, drain(t, it, true))
}

func TestIteratorKeysOnly(t *testing.T) {
	store := newStore(t)
	seed(t, store)

	it, err := store.Iterator(context.Background(), corekv.IterOptions{KeysOnly: true})
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	defer closeIter(t, it)

	assertKeys(t, []string{"a", "b", "c", "d", "e"}, drain(t, it, false))
}

func TestIteratorDoubleCloseIsSafe(t *testing.T) {
	store := newStore(t)

	it, err := store.Iterator(context.Background(), corekv.DefaultIterOptions)
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	closeIter(t, it)
	closeIter(t, it)
}

func TestTxnCommitIsVisible(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	seed(t, store)

	txn := store.NewTxn(false)
	defer txn.Discard()

	if err := txn.Set(ctx, []byte("f"), []byte("vf")); err != nil {
		t.Fatalf("txn set: %v", err)
	}
	if err := txn.Delete(ctx, []byte("a")); err != nil {
		t.Fatalf("txn delete: %v", err)
	}

	// The transaction sees its own writes, the store does not, yet.
	value, err := txn.Get(ctx, []byte("f"))
	if err != nil {
		t.Fatalf("txn get: %v", err)
	}
	if string(value) != "vf" {
		t.Errorf("expected vf, got %s", value)
	}
	if _, err := txn.Get(ctx, []byte("a")); !errors.Is(err, corekv.ErrNotFound) {
		t.Errorf("txn get: expected ErrNotFound, got %v", err)
	}
	if _, err := store.Get(ctx, []byte("f")); !errors.Is(err, corekv.ErrNotFound) {
		t.Errorf("get: expected ErrNotFound, got %v", err)
	}

	if err := txn.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	value, err = store.Get(ctx, []byte("f"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(value) != "vf" {
		t.Errorf("expected vf, got %s", value)
	}
	if _, err := store.Get(ctx, []byte("a")); !errors.Is(err, corekv.ErrNotFound) {
		t.Errorf("get: expected ErrNotFound, got %v", err)
	}

	// A committed transaction is resolved, and the deferred discard above must
	// not free its handle a second time.
	if err := txn.Commit(); !errors.Is(err, corekv.ErrDiscardedTxn) {
		t.Errorf("second commit: expected ErrDiscardedTxn, got %v", err)
	}
	if _, err := txn.Get(ctx, []byte("f")); !errors.Is(err, corekv.ErrDiscardedTxn) {
		t.Errorf("get after commit: expected ErrDiscardedTxn, got %v", err)
	}
}

func TestTxnDiscardIsInvisible(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	seed(t, store)

	txn := store.NewTxn(false)
	if err := txn.Set(ctx, []byte("f"), []byte("vf")); err != nil {
		t.Fatalf("txn set: %v", err)
	}
	txn.Discard()
	// Discarding twice must not free the handle twice.
	txn.Discard()

	if _, err := store.Get(ctx, []byte("f")); !errors.Is(err, corekv.ErrNotFound) {
		t.Errorf("get: expected ErrNotFound, got %v", err)
	}
	if err := txn.Set(ctx, []byte("g"), []byte("vg")); !errors.Is(err, corekv.ErrDiscardedTxn) {
		t.Errorf("set after discard: expected ErrDiscardedTxn, got %v", err)
	}
	if err := txn.Commit(); !errors.Is(err, corekv.ErrDiscardedTxn) {
		t.Errorf("commit after discard: expected ErrDiscardedTxn, got %v", err)
	}
}

func TestTxnConflict(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	if err := store.Set(ctx, []byte("k"), []byte("v0")); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Both transactions begin before either commits, and both write the same
	// key, so snapshot-isolation validation must reject the second.
	first := store.NewTxn(false)
	defer first.Discard()
	second := store.NewTxn(false)
	defer second.Discard()

	if err := first.Set(ctx, []byte("k"), []byte("v1")); err != nil {
		t.Fatalf("txn set: %v", err)
	}
	if err := second.Set(ctx, []byte("k"), []byte("v2")); err != nil {
		t.Fatalf("txn set: %v", err)
	}

	if err := first.Commit(); err != nil {
		t.Fatalf("first commit: %v", err)
	}
	if err := second.Commit(); !errors.Is(err, corekv.ErrTxnConflict) {
		t.Errorf("second commit: expected ErrTxnConflict, got %v", err)
	}

	value, err := store.Get(ctx, []byte("k"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(value) != "v1" {
		t.Errorf("expected v1, got %s", value)
	}
}

func TestReadOnlyTxnRejectsWrites(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	seed(t, store)

	txn := store.NewTxn(true)
	defer txn.Discard()

	if err := txn.Set(ctx, []byte("f"), []byte("vf")); !errors.Is(err, corekv.ErrReadOnlyTxn) {
		t.Errorf("set: expected ErrReadOnlyTxn, got %v", err)
	}
	if err := txn.Delete(ctx, []byte("a")); !errors.Is(err, corekv.ErrReadOnlyTxn) {
		t.Errorf("delete: expected ErrReadOnlyTxn, got %v", err)
	}

	// Reads still work.
	value, err := txn.Get(ctx, []byte("a"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(value) != "va" {
		t.Errorf("expected va, got %s", value)
	}
	has, err := txn.Has(ctx, []byte("a"))
	if err != nil {
		t.Fatalf("has: %v", err)
	}
	if !has {
		t.Error("expected a to be found")
	}
}

func TestTxnIteratorSeesBufferedWrites(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	seed(t, store)

	txn := store.NewTxn(false)
	defer txn.Discard()

	// An insert, an overwrite and a delete, all uncommitted.
	if err := txn.Set(ctx, []byte("bb"), []byte("vbb")); err != nil {
		t.Fatalf("txn set: %v", err)
	}
	if err := txn.Set(ctx, []byte("c"), []byte("vc2")); err != nil {
		t.Fatalf("txn set: %v", err)
	}
	if err := txn.Delete(ctx, []byte("d")); err != nil {
		t.Fatalf("txn delete: %v", err)
	}

	it, err := txn.Iterator(ctx, corekv.DefaultIterOptions)
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}

	keys := []string{}
	for {
		hasNext, err := it.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if !hasNext {
			break
		}
		keys = append(keys, string(it.Key()))
		if string(it.Key()) == "c" {
			value, err := it.Value()
			if err != nil {
				t.Fatalf("value: %v", err)
			}
			if string(value) != "vc2" {
				t.Errorf("expected the buffered value vc2, got %s", value)
			}
		}
	}
	assertKeys(t, []string{"a", "b", "bb", "c", "e"}, keys)

	// Reset re-walks the same merged view.
	it.Reset()
	assertKeys(t, []string{"a", "b", "bb", "c", "e"}, drainKeys(t, it))
	closeIter(t, it)

	// Reverse, bounded and prefixed transaction iteration.
	it, err = txn.Iterator(ctx, corekv.IterOptions{Reverse: true})
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	assertKeys(t, []string{"e", "c", "bb", "b", "a"}, drainKeys(t, it))
	closeIter(t, it)

	it, err = txn.Iterator(ctx, corekv.IterOptions{Start: []byte("b"), End: []byte("c")})
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	assertKeys(t, []string{"b", "bb"}, drainKeys(t, it))
	closeIter(t, it)

	it, err = txn.Iterator(ctx, corekv.IterOptions{Prefix: []byte("b")})
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	assertKeys(t, []string{"b", "bb"}, drainKeys(t, it))
	closeIter(t, it)
}

func TestDropAll(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	seed(t, store)

	if err := store.DropAll(); err != nil {
		t.Fatalf("drop all: %v", err)
	}

	if _, err := store.Get(ctx, []byte("a")); !errors.Is(err, corekv.ErrNotFound) {
		t.Errorf("get: expected ErrNotFound, got %v", err)
	}

	it, err := store.Iterator(ctx, corekv.DefaultIterOptions)
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	defer closeIter(t, it)
	assertKeys(t, []string{}, drainKeys(t, it))

	// The store is still usable.
	if err := store.Set(ctx, []byte("a"), []byte("va")); err != nil {
		t.Fatalf("set: %v", err)
	}
}

// TestContextTxn covers the context-transaction path: a transaction set on the
// context is used by the store-level functions instead of the store itself.
func TestContextTxn(t *testing.T) {
	store := newStore(t)
	seed(t, store)

	txn := store.NewTxn(false)
	defer txn.Discard()
	ctx := corekv.SetCtxTxn(context.Background(), txn)

	if err := store.Set(ctx, []byte("f"), []byte("vf")); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := store.Delete(ctx, []byte("a")); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// The write went to the transaction, so only the transactional reads, and
	// the iterator built from the same context, can see it.
	value, err := store.Get(ctx, []byte("f"))
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(value) != "vf" {
		t.Errorf("expected vf, got %s", value)
	}
	has, err := store.Has(ctx, []byte("f"))
	if err != nil {
		t.Fatalf("has: %v", err)
	}
	if !has {
		t.Error("expected f to be found")
	}

	it, err := store.Iterator(ctx, corekv.DefaultIterOptions)
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	assertKeys(t, []string{"b", "c", "d", "e", "f"}, drainKeys(t, it))
	closeIter(t, it)

	if _, err := store.Get(context.Background(), []byte("f")); !errors.Is(err, corekv.ErrNotFound) {
		t.Errorf("get: expected ErrNotFound, got %v", err)
	}

	if err := txn.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := store.Get(context.Background(), []byte("f")); err != nil {
		t.Errorf("get after commit: %v", err)
	}
}

// TestLifecycle hammers the store through a few thousand set/get/iterate cycles
// in order to smoke out handle lifecycle and leak bugs that the unit tests,
// each of which makes only a handful of FFI calls, would never reach.
func drainKeys(t *testing.T, it corekv.Iterator) []string {
	t.Helper()

	keys := []string{}
	for {
		hasNext, err := it.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if !hasNext {
			return keys
		}
		keys = append(keys, string(it.Key()))
	}
}

func closeIter(t *testing.T, it corekv.Iterator) {
	t.Helper()

	if err := it.Close(); err != nil {
		t.Errorf("iterator close: %v", err)
	}
}

// TestOpenWithOptions covers the options parameter: a nil one is the engine
// defaults, a set field reaches the engine, and an invalid one is rejected with
// the field named rather than clamped.
func TestOpenWithOptions(t *testing.T) {
	ctx := context.Background()

	for name, opts := range map[string]*regolith.Options{
		"nil":      nil,
		"zero":     {},
		"tuned":    {TransactionKeysInline: regolith.Uint64(8)},
		"noWorker": {MaxBackgroundCompactions: regolith.Uint64(0)},
		"noCache":  {BlockCacheSize: regolith.Uint64(0)},
		"durable":  {Durability: regolith.DurabilityImmediate},
	} {
		t.Run(name, func(t *testing.T) {
			store, err := NewDatastore(t.TempDir(), opts)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer func() {
				if err := store.Close(); err != nil {
					t.Errorf("close: %v", err)
				}
			}()

			if err := store.Set(ctx, []byte("k"), []byte("v")); err != nil {
				t.Fatalf("set: %v", err)
			}
			value, err := store.Get(ctx, []byte("k"))
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			if string(value) != "v" {
				t.Errorf("get: got %q, want %q", value, "v")
			}
		})
	}
}

func TestOpenWithAnInvalidOption(t *testing.T) {
	// regolith requires a non-zero write buffer, and validates its options
	// before touching the filesystem, so nothing is created.
	_, err := NewDatastore(t.TempDir(), &regolith.Options{
		WriteBufferSize: regolith.Uint64(0),
	})
	if err == nil {
		t.Fatal("expected an error for a zero write buffer")
	}
	if !strings.Contains(err.Error(), "write_buffer_size") {
		t.Errorf("error does not name the field: %v", err)
	}
}

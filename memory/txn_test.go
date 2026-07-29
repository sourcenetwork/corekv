package memory

import (
	"context"
	"testing"
	"time"

	"github.com/sourcenetwork/corekv"
	"github.com/stretchr/testify/require"
)

func TestTxnGetWhileIteratorOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewDatastore(ctx)
	require.NoError(t, store.Set(ctx, []byte("a"), []byte("first")))
	require.NoError(t, store.Set(ctx, []byte("b"), []byte("second")))

	txn := store.NewTxn(false)
	defer txn.Discard()
	iter, err := txn.Iterator(ctx, corekv.IterOptions{})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, iter.Close())
	}()
	requireNextValue(t, iter, []byte("first"))

	type getResult struct {
		value []byte
		err   error
	}
	result := make(chan getResult, 1)
	go func() {
		value, err := txn.Get(ctx, []byte("b"))
		result <- getResult{value: value, err: err}
	}()

	select {
	case got := <-result:
		require.NoError(t, got.err)
		require.Equal(t, []byte("second"), got.value)
	case <-time.After(time.Second):
		t.Fatal("transaction read blocked while iterator was open")
	}
}

func TestTxnReadConflict(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewDatastore(ctx)
	require.NoError(t, store.Set(ctx, []byte("key"), []byte("old")))

	txn := store.NewTxn(false)
	value, err := txn.Get(ctx, []byte("key"))
	require.NoError(t, err)
	require.Equal(t, []byte("old"), value)

	require.NoError(t, store.Set(ctx, []byte("key"), []byte("new")))
	require.ErrorIs(t, txn.Commit(), corekv.ErrTxnConflict)
}

func TestTxnMissingReadConflict(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewDatastore(ctx)
	txn := store.NewTxn(false)
	_, err := txn.Get(ctx, []byte("key"))
	require.ErrorIs(t, err, corekv.ErrNotFound)

	require.NoError(t, store.Set(ctx, []byte("key"), []byte("new")))
	require.ErrorIs(t, txn.Commit(), corekv.ErrTxnConflict)
}

func requireNextValue(t *testing.T, iter corekv.Iterator, expected []byte) {
	t.Helper()
	ok, err := iter.Next()
	require.NoError(t, err)
	require.True(t, ok)
	value, err := iter.Value()
	require.NoError(t, err)
	require.Equal(t, expected, value)
}

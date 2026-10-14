package badger

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v4"

	"github.com/sourcenetwork/corekv"
)

func TestTxnSet_TooBig_ReturnsErrTxnTooBig(t *testing.T) {
	ctx := context.Background()

	// A small memtable keeps the transaction size limit low, so a few writes are enough to hit it.
	opts := badger.DefaultOptions("").WithInMemory(true).WithMemTableSize(256 << 10).WithValueThreshold(1 << 10)
	store, err := NewDatastore("", opts)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	txn := store.NewTxn(false)
	defer txn.Discard()

	value := make([]byte, 1024)
	for i := 0; i < 10_000; i++ {
		err = txn.Set(ctx, []byte(fmt.Sprintf("k%d", i)), value)
		if err != nil {
			break
		}
	}

	if !errors.Is(err, corekv.ErrTxnTooBig) {
		t.Fatalf("expected %v, got %v", corekv.ErrTxnTooBig, err)
	}
}

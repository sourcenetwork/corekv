package integration

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/multiplier"
)

// The first four tests below exclude multiplier.TxnContext: under txn-context the batch
// action still runs against the store (test/action/txn.go leaves s.Store alone and only
// sets the context value), so the batch lands in the store while the reads that follow
// run inside a transaction whose snapshot predates it. The remaining tests target one
// store directly with Includes, where that reasoning does not apply.

func TestWriteBatch_SetsAndDeletes(t *testing.T) {
	test := &Test{
		Excludes: []multiplier.Name{
			multiplier.TxnContext,
		},
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k2"), []byte("v2")),
			action.WriteBatch(
				corekv.BatchOp{Key: []byte("k3"), Value: []byte("v3")},
				corekv.BatchOp{Key: []byte("k1"), Delete: true},
				corekv.BatchOp{Key: []byte("k2"), Value: []byte("v2b")},
			),
			action.Get([]byte("k3"), []byte("v3")),
			action.Has([]byte("k1"), false),
			action.Get([]byte("k2"), []byte("v2b")),
		},
	}

	test.Execute(t)
}

func TestWriteBatch_LastOperationOnAKeyWins(t *testing.T) {
	test := &Test{
		Excludes: []multiplier.Name{
			multiplier.TxnContext,
		},
		Actions: []action.Action{
			action.WriteBatch(
				corekv.BatchOp{Key: []byte("k1"), Value: []byte("v1")},
				corekv.BatchOp{Key: []byte("k1"), Value: []byte("v2")},
				corekv.BatchOp{Key: []byte("k2"), Delete: true},
				corekv.BatchOp{Key: []byte("k2"), Value: []byte("v3")},
				corekv.BatchOp{Key: []byte("k3"), Value: []byte("v4")},
				corekv.BatchOp{Key: []byte("k3"), Delete: true},
			),
			action.Get([]byte("k1"), []byte("v2")),
			action.Get([]byte("k2"), []byte("v3")),
			action.Has([]byte("k3"), false),
		},
	}

	test.Execute(t)
}

func TestWriteBatch_EmptyBatchIsANoOp(t *testing.T) {
	test := &Test{
		Excludes: []multiplier.Name{
			multiplier.TxnContext,
		},
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.WriteBatch(),
			action.Get([]byte("k1"), []byte("v1")),
			action.Has([]byte("k2"), false),
		},
	}

	test.Execute(t)
}

// TestWriteBatch_ReusedBatchDoesNotReplayPreviousOps guards the pooled-batch adapters:
// a WriteBatch that is not reset before it goes back in the pool would still hold the
// first call's operations, and the second call would silently reapply them on top of
// its own.
func TestWriteBatch_ReusedBatchDoesNotReplayPreviousOps(t *testing.T) {
	test := &Test{
		Excludes: []multiplier.Name{
			multiplier.TxnContext,
		},
		Actions: []action.Action{
			action.WriteBatch(
				corekv.BatchOp{Key: []byte("k1"), Value: []byte("v1")},
			),
			action.Set([]byte("k1"), []byte("v2")),
			action.WriteBatch(
				corekv.BatchOp{Key: []byte("k9"), Value: []byte("v9")},
			),
			action.Get([]byte("k1"), []byte("v2")),
			action.Get([]byte("k9"), []byte("v9")),
		},
	}

	test.Execute(t)
}

func TestWriteBatchClose_BadgerStoreWriteOnClosedStore_Errors(t *testing.T) {
	test := &Test{
		Includes: []multiplier.Name{
			multiplier.Badger,
		},
		Actions: []action.Action{
			action.Close(),
			action.WriteBatchE("datastore closed",
				corekv.BatchOp{Key: []byte("k1"), Value: []byte("v1")}),
		},
	}

	test.Execute(t)
}

func TestWriteBatchClose_RegolithStoreWriteOnClosedStore_Errors(t *testing.T) {
	test := &Test{
		Includes: []multiplier.Name{
			multiplier.Regolith,
		},
		Actions: []action.Action{
			action.Close(),
			action.WriteBatchE("datastore closed",
				corekv.BatchOp{Key: []byte("k1"), Value: []byte("v1")}),
		},
	}

	test.Execute(t)
}

// TestWriteBatch_BadgerStoreEmptyKeyErrors is badger-only: go-regolith packs an empty
// key into a batch without complaint, but badger's underlying WriteBatch.Set rejects
// one, which is what exercises (and guards) the per-operation error return in
// [github.com/sourcenetwork/corekv/badger.Datastore.WriteBatch].
func TestWriteBatch_BadgerStoreEmptyKeyErrors(t *testing.T) {
	test := &Test{
		Includes: []multiplier.Name{
			multiplier.Badger,
		},
		Actions: []action.Action{
			action.WriteBatchE("empty key",
				corekv.BatchOp{Key: []byte("k1"), Value: []byte("v1")},
				corekv.BatchOp{Key: nil, Value: []byte("v2")},
			),
		},
	}

	test.Execute(t)
}

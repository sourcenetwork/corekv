package bench

import (
	"bytes"
	"testing"
)

// TestPRNGMatchesRustBaseline pins the shuffle to the native Rust baseline. The golden
// values below were produced by compiling rust-baseline's Rng + shuffled() verbatim and
// printing the permutation; if this test fails, the Go and Rust lanes no longer touch
// keys in the same order and the three-lane comparison is invalid.
func TestPRNGMatchesRustBaseline(t *testing.T) {
	got := shuffled(PrefillCount)
	if len(got) != PrefillCount {
		t.Fatalf("len = %d, want %d", len(got), PrefillCount)
	}

	wantHead := []uint64{77803, 87264, 46469, 78918, 37063, 10674, 75887, 32566}
	wantTail := []uint64{59396, 21994, 2435, 96590, 27786, 61398, 94685, 93600}
	assertPerm(t, "shuffled(100000)", got, wantHead, wantTail)

	got = shuffled(writeN)
	assertPerm(t, "shuffled(10000)", got,
		[]uint64{2029, 5564, 518, 3695, 4826, 923, 2711, 9719}, nil)
}

func assertPerm(t *testing.T, name string, got, wantHead, wantTail []uint64) {
	t.Helper()

	for i, w := range wantHead {
		if got[i] != w {
			t.Fatalf("%s[%d] = %d, want %d", name, i, got[i], w)
		}
	}
	for i, w := range wantTail {
		at := len(got) - len(wantTail) + i
		if got[at] != w {
			t.Fatalf("%s[%d] = %d, want %d", name, at, got[at], w)
		}
	}

	seen := make([]bool, len(got))
	for _, v := range got {
		if seen[v] {
			t.Fatalf("%s is not a permutation: %d repeats", name, v)
		}
		seen[v] = true
	}
}

// TestKeyFormat pins the key encoding to the Rust lane's format!("key:{i:012}").
func TestKeyFormat(t *testing.T) {
	for _, c := range []struct {
		i    uint64
		want string
	}{
		{0, "key:000000000000"},
		{42, "key:000000000042"},
		{99999, "key:000000099999"},
		{1_000_000, "key:000001000000"},
	} {
		if got := key(c.i); !bytes.Equal(got, []byte(c.want)) {
			t.Errorf("key(%d) = %q, want %q", c.i, got, c.want)
		}
	}

	// scanPrefix must select exactly scanPrefixN of the prefilled keys.
	n := 0
	for i := 0; i < PrefillCount; i++ {
		if bytes.HasPrefix(presentKeys[i], scanPrefix) {
			n++
		}
	}
	if n != scanPrefixN {
		t.Errorf("scanPrefix %q matches %d keys, want %d", scanPrefix, n, scanPrefixN)
	}
}

// TestBatchWriteNativeMirrorsBatchWrite pins the new row to the row it is read against.
// The two are only comparable if they touch the same keys, the same number of times,
// against the same fixture, so a divergence here silently invalidates the pair.
func TestBatchWriteNativeMirrorsBatchWrite(t *testing.T) {
	native, ok := lookupWorkload("BatchWriteNative")
	if !ok {
		t.Fatal("BatchWriteNative workload is not registered")
	}
	txn, ok := lookupWorkload("BatchWrite")
	if !ok {
		t.Fatal("BatchWrite workload is not registered")
	}

	if native.opsPerIter != txn.opsPerIter {
		t.Errorf("opsPerIter = %d, want %d (BatchWrite's)", native.opsPerIter, txn.opsPerIter)
	}
	if native.prefillN != txn.prefillN {
		t.Errorf("prefillN = %d, want %d (BatchWrite's)", native.prefillN, txn.prefillN)
	}
	if native.readOnly {
		t.Error("BatchWriteNative is marked readOnly; it writes")
	}
	if !native.movesValues {
		t.Error("BatchWriteNative does not report bytes; it moves values")
	}
}

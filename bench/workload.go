// Package bench holds the shared, store-agnostic workload definitions used to compare
// corekv store implementations (badger, memory, and - behind `-tags regolith` - regolith
// over cgo) against each other, and against the native Rust baseline in `rust-baseline/`.
//
// The workloads are defined exactly once here and are driven purely through the
// [corekv.Store] / [corekv.TxnStore] interfaces, so every lane executes the same
// sequence of operations against the same keys.
//
// FIDELITY: this file mirrors rust-baseline/benches/workloads.rs. The PRNG, the shuffle,
// the key formats, the per-iteration operation counts and the value bytes are all
// reproduced bit-for-bit. Do not change one side without the other. The spec table lives
// in ../PLAN-regolith.md.
package bench

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/sourcenetwork/corekv"
)

const (
	// Seed is the fixed PRNG seed used for every random ordering in every lane, so
	// that all lanes touch keys in exactly the same sequence.
	Seed = 42

	// PrefillCount is the number of keys in the read-only fixture (PREFILL_N).
	PrefillCount = 100_000

	// writeN is the number of `Set`s in one SeqWrite/RandWrite iteration (WRITE_N).
	writeN = 10_000

	// pointOps is the number of point operations in one GetHit/GetMiss/Has iteration
	// (POINT_OPS).
	pointOps = 1_000

	// missOffset is added to the shuffled index to produce a key the prefill never
	// wrote. Matches the Rust lane's `key(1_000_000 + miss.next())`.
	missOffset = 1_000_000

	// txnWriteN is the number of `Set`s in one TxnWrite transaction (TXN_WRITE_N).
	txnWriteN = 100

	// txnRWN is the number of `Get`s, and separately of `Set`s, in one TxnReadWrite
	// transaction (TXN_RW_N). The workload therefore performs 2*txnRWN operations.
	txnRWN = 10

	// batchWriteN is the number of `Set`s in one BatchWrite transaction
	// (BATCH_WRITE_N). It is also the prefill size of the transactional fixture, so
	// that TxnReadWrite's reads are hits.
	//
	// Note: "BatchWrite" is deliberately a 1000-key transaction rather than a native
	// batch primitive - corekv's TxnStore exposes no batch API, so a native
	// WriteBatch number would have no Go counterpart.
	batchWriteN = 1_000

	// parallelThreads / parallelOpsPerThread: ParallelMixed is exactly 4 goroutines of
	// 1000 operations each per iteration (PARALLEL_THREADS / PARALLEL_OPS_PER_THREAD).
	parallelThreads      = 4
	parallelOpsPerThread = 1_000

	// scanPrefixN is the number of keys matched by scanPrefix (SCAN_PREFIX_N).
	scanPrefixN = 1_000
)

// scanPrefix matches keys key:000000099000 .. key:000000099999 - exactly 1000 of the
// 100k prefilled keys (SCAN_PREFIX).
var scanPrefix = []byte("key:000000099")

// sink keeps values read by the scan and read workloads from being optimised away.
// It is the analogue of the Rust lane's black_box.
var sink int

// rng is xorshift64* (Marsaglia/Vigna). It reproduces rust-baseline's Rng exactly: the
// state is a single u64 initialised to the seed, and the final multiply is an output
// scramble that does NOT feed back into the state.
type rng struct{ x uint64 }

func newRng(seed uint64) *rng { return &rng{x: seed} }

func (r *rng) next() uint64 {
	r.x ^= r.x >> 12
	r.x ^= r.x << 25
	r.x ^= r.x >> 27
	return r.x * 0x2545F4914F6CDD1D
}

// shuffled returns [0, n) permuted by a descending Fisher-Yates driven by newRng(Seed).
//
// The index is `next() % (i + 1)` - plain modulo, not rejection sampling. The modulo
// bias is identical on both sides of the comparison and therefore cancels; do not
// "fix" it, or the two lanes stop touching keys in the same order.
func shuffled(n uint64) []uint64 {
	v := make([]uint64, n)
	for i := range v {
		v[i] = uint64(i)
	}

	r := newRng(Seed)
	for i := n - 1; i > 0; i-- {
		j := r.next() % (i + 1)
		v[i], v[j] = v[j], v[i]
	}
	return v
}

// key returns the benchmark key for index i: `key:` followed by i zero-padded to 12
// digits, 16 bytes total. Identical to the Rust lane's `format!("key:{i:012}")`.
//
// A fresh slice is allocated per call because stores are allowed to retain the key
// (badger's transaction write-set does exactly that), so a shared scratch buffer would
// be unsound. The allocation is identical in every lane.
func key(i uint64) []byte {
	b := make([]byte, 16)
	copy(b, "key:")
	for p := 15; p >= 4; p-- {
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return b
}

// value returns the benchmark value of n bytes: 0xAB repeated, matching the Rust lane's
// `vec![0xAB; vsize]`.
func value(n int) []byte {
	return bytes.Repeat([]byte{0xAB}, n)
}

var (
	// order is the shuffled permutation of [0, PrefillCount) shared by the read
	// workloads. All of the Rust lane's cursors use the same seed, so one slice with
	// independent cursors is equivalent to its three Cursor values.
	order []uint64

	// writeKeys are the 10k keys of SeqWrite, in ascending order.
	writeKeys [][]byte

	// randWriteKeys are the same 10k keys in shuffled order (RandWrite).
	randWriteKeys [][]byte

	// presentKeys[i] == key(i) for the prefill range, precomputed so the read
	// workloads measure the store rather than key formatting.
	presentKeys [][]byte

	// missKeys[i] == key(missOffset + i); guaranteed absent from a prefilled store.
	missKeys [][]byte
)

func init() {
	order = shuffled(PrefillCount)

	writeKeys = make([][]byte, writeN)
	for i := range writeKeys {
		writeKeys[i] = key(uint64(i))
	}

	writeOrder := shuffled(writeN)
	randWriteKeys = make([][]byte, writeN)
	for i, o := range writeOrder {
		randWriteKeys[i] = key(o)
	}

	presentKeys = make([][]byte, PrefillCount)
	missKeys = make([][]byte, PrefillCount)
	for i := range presentKeys {
		presentKeys[i] = key(uint64(i))
		missKeys[i] = key(missOffset + uint64(i))
	}
}

// factory constructs a store for one benchmark lane.
//
// `new` must return a ready-to-use store and register its own teardown (Close, and
// removal of any on-disk data) via tb.Cleanup.
type factory struct {
	name string
	new  func(tb testing.TB) corekv.TxnStore
}

// factories is the set of lanes the suite runs. The regolith lane appends itself from
// stores_regolith.go, which is behind `-tags regolith`.
var factories []factory

// regolithNoop, when non-nil, calls the regolith FFI no-op entry point. It is set by
// stores_regolith.go; BenchmarkFFINoop skips when it is nil.
var regolithNoop func()

// workload is one row of the spec table.
type workload struct {
	name string

	// prefillN is the number of keys written to the store before `run` starts; 0 for
	// none. Prefill always happens outside the timed region.
	prefillN int

	// readOnly marks a workload that never mutates the store, allowing one prefilled
	// store to be shared by all read-only workloads of the same (lane, value size).
	// It must never be set on a workload that writes.
	readOnly bool

	// opsPerIter is the number of store operations (keys touched) performed by a
	// single b.N iteration of `run`, matching the Rust lane's Throughput::Elements.
	// It is the divisor used to report "ns/op-key", the number the lanes are
	// compared on.
	opsPerIter int

	// movesValues indicates each counted op transfers a whole value, enabling a
	// meaningful b.SetBytes throughput figure.
	movesValues bool

	// run performs the whole b.N loop. Timer management and metric reporting are the
	// caller's job; `run` must only do the work.
	run func(b *testing.B, s corekv.TxnStore, val []byte)
}

// workloads are the 12 workloads of the spec table, in spec order, plus ScanAllAppend
// and ScanAllBorrow (variants of ScanAll that read values through the optional
// [corekv.ValueAppender] / [corekv.ValueBorrower] interfaces, and have no Rust
// counterparts).
var workloads = []workload{
	{
		// Successive iterations rewrite the same 10k keys, exactly as the Rust lane
		// does, so the engine sees overwrites after the first iteration.
		name: "SeqWrite", opsPerIter: writeN, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, val []byte) {
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				for _, k := range writeKeys {
					if err := s.Set(ctx, k, val); err != nil {
						b.Fatal(err)
					}
				}
			}
		},
	},
	{
		name: "RandWrite", opsPerIter: writeN, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, val []byte) {
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				for _, k := range randWriteKeys {
					if err := s.Set(ctx, k, val); err != nil {
						b.Fatal(err)
					}
				}
			}
		},
	},
	{
		// The cursor into the permutation wraps and persists across iterations, so
		// the whole 100k key set is eventually touched rather than the first 1000
		// entries over and over.
		name: "GetHit", prefillN: PrefillCount, readOnly: true, opsPerIter: pointOps, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, val []byte) {
			ctx := context.Background()
			at := 0
			for i := 0; i < b.N; i++ {
				for j := 0; j < pointOps; j++ {
					v, err := s.Get(ctx, presentKeys[order[at]])
					if err != nil {
						b.Fatal(err)
					}
					if len(v) != len(val) {
						b.Fatalf("unexpected value length: got %d, want %d", len(v), len(val))
					}
					sink += len(v)
					at = (at + 1) % len(order)
				}
			}
		},
	},
	{
		// The most sensitive probe of raw call overhead: a bloom-filter
		// short-circuit, no value copied.
		name: "GetMiss", prefillN: PrefillCount, readOnly: true, opsPerIter: pointOps,
		run: func(b *testing.B, s corekv.TxnStore, _ []byte) {
			ctx := context.Background()
			at := 0
			for i := 0; i < b.N; i++ {
				for j := 0; j < pointOps; j++ {
					_, err := s.Get(ctx, missKeys[order[at]])
					if !errors.Is(err, corekv.ErrNotFound) {
						b.Fatalf("expected ErrNotFound, got %v", err)
					}
					at = (at + 1) % len(order)
				}
			}
		},
	},
	{
		name: "Has", prefillN: PrefillCount, readOnly: true, opsPerIter: pointOps,
		run: func(b *testing.B, s corekv.TxnStore, _ []byte) {
			ctx := context.Background()
			at := 0
			for i := 0; i < b.N; i++ {
				for j := 0; j < pointOps; j++ {
					ok, err := s.Has(ctx, presentKeys[order[at]])
					if err != nil {
						b.Fatal(err)
					}
					if !ok {
						b.Fatal("Has missed a prefilled key")
					}
					at = (at + 1) % len(order)
				}
			}
		},
	},
	{
		name: "ScanAll", prefillN: PrefillCount, readOnly: true, opsPerIter: PrefillCount, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, _ []byte) {
			for i := 0; i < b.N; i++ {
				scan(b, s, corekv.DefaultIterOptions, PrefillCount)
			}
		},
	},
	{
		// The same scan as ScanAll, but reading each value through
		// [corekv.ValueAppender] with a single re-used buffer where the iterator
		// implements it, falling back to `Value` where it does not. The difference
		// against ScanAll is the cost of one allocation per value.
		//
		// This workload has no Rust counterpart - it measures a property of the Go
		// interface, not of the engine - so it is excluded from the fidelity
		// comparison. Its opsPerIter matches ScanAll so the two are directly
		// comparable.
		name: "ScanAllAppend", prefillN: PrefillCount, readOnly: true, opsPerIter: PrefillCount, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, _ []byte) {
			for i := 0; i < b.N; i++ {
				scanAppend(b, s, corekv.DefaultIterOptions, PrefillCount)
			}
		},
	},
	{
		// The same scan again, but reading each value through [corekv.ValueBorrower],
		// which copies nothing at all, falling back to `Value` where the iterator
		// does not implement it.
		//
		// Like ScanAllAppend this has no Rust counterpart and shares ScanAll's
		// opsPerIter, so the three are directly comparable:
		// ScanAll = alloc + copy, ScanAllAppend = copy, ScanAllBorrow = neither.
		name: "ScanAllBorrow", prefillN: PrefillCount, readOnly: true, opsPerIter: PrefillCount, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, _ []byte) {
			for i := 0; i < b.N; i++ {
				scanBorrow(b, s, corekv.DefaultIterOptions, PrefillCount)
			}
		},
	},
	{
		name: "ScanReverse", prefillN: PrefillCount, readOnly: true, opsPerIter: PrefillCount, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, _ []byte) {
			for i := 0; i < b.N; i++ {
				scan(b, s, corekv.IterOptions{Reverse: true}, PrefillCount)
			}
		},
	},
	{
		name: "ScanPrefix", prefillN: PrefillCount, readOnly: true, opsPerIter: scanPrefixN, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, _ []byte) {
			for i := 0; i < b.N; i++ {
				scan(b, s, corekv.IterOptions{Prefix: scanPrefix}, scanPrefixN)
			}
		},
	},
	{
		// Transactions run one at a time, so no commit can conflict; a conflict here
		// would be a bug and is fatal.
		name: "TxnWrite", prefillN: batchWriteN, opsPerIter: txnWriteN, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, val []byte) {
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				txn := s.NewTxn(false)
				for j := uint64(0); j < txnWriteN; j++ {
					if err := txn.Set(ctx, key(j), val); err != nil {
						txn.Discard()
						b.Fatal(err)
					}
				}
				if err := txn.Commit(); err != nil {
					txn.Discard()
					b.Fatal(err)
				}
			}
		},
	},
	{
		name: "TxnReadWrite", prefillN: batchWriteN, opsPerIter: 2 * txnRWN, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, val []byte) {
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				txn := s.NewTxn(false)
				for j := uint64(0); j < txnRWN; j++ {
					v, err := txn.Get(ctx, key(j))
					if err != nil {
						txn.Discard()
						b.Fatal(err)
					}
					sink += len(v)
				}
				for j := uint64(0); j < txnRWN; j++ {
					if err := txn.Set(ctx, key(j), val); err != nil {
						txn.Discard()
						b.Fatal(err)
					}
				}
				if err := txn.Commit(); err != nil {
					txn.Discard()
					b.Fatal(err)
				}
			}
		},
	},
	{
		name: "BatchWrite", prefillN: batchWriteN, opsPerIter: batchWriteN, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, val []byte) {
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				txn := s.NewTxn(false)
				for j := uint64(0); j < batchWriteN; j++ {
					if err := txn.Set(ctx, key(j), val); err != nil {
						txn.Discard()
						b.Fatal(err)
					}
				}
				if err := txn.Commit(); err != nil {
					txn.Discard()
					b.Fatal(err)
				}
			}
		},
	},
	{
		// 4 goroutines x 1000 ops per iteration, 90% Get / 10% Set, each goroutine
		// starting its cursor at t*len(order)/4.
		//
		// This deliberately does NOT use b.RunParallel: RunParallel fixes the
		// goroutine count at p*GOMAXPROCS and splits b.N between them, which cannot
		// reproduce "4 threads of exactly 1000 ops" on a machine with any other CPU
		// count. Spawning the four goroutines explicitly matches the Rust lane's
		// thread::scope shape exactly; the spawn cost per iteration is amortised over
		// 4000 store operations (and the Rust lane pays a larger one).
		name: "ParallelMixed", prefillN: PrefillCount, opsPerIter: parallelThreads * parallelOpsPerThread, movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, val []byte) {
			ctx := context.Background()
			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup
				for t := 0; t < parallelThreads; t++ {
					wg.Add(1)
					go func(t int) {
						defer wg.Done()
						base := t * (len(order) / parallelThreads)
						for n := 0; n < parallelOpsPerThread; n++ {
							k := presentKeys[order[(base+n)%len(order)]]
							if n%10 == 9 {
								if err := s.Set(ctx, k, val); err != nil {
									// b.Fatal is illegal off the benchmark goroutine.
									b.Error(err)
									return
								}
								continue
							}
							v, err := s.Get(ctx, k)
							if err != nil {
								b.Error(err)
								return
							}
							if len(v) != len(val) {
								b.Errorf("unexpected value length: got %d, want %d", len(v), len(val))
								return
							}
						}
					}(t)
				}
				wg.Wait()
			}
		},
	},
}

// scan runs one full iteration pass with the given options, summing value lengths, and
// fails the benchmark unless exactly wantCount items were yielded. The count assertion
// is what stops a silently-empty iterator from looking like a fast one; the Rust lane
// asserts the same counts.
func scan(b *testing.B, s corekv.TxnStore, opts corekv.IterOptions, wantCount int) {
	ctx := context.Background()

	it, err := s.Iterator(ctx, opts)
	if err != nil {
		b.Fatal(err)
	}

	n, total := 0, 0
	for {
		ok, err := it.Next()
		if err != nil {
			b.Fatal(err)
		}
		if !ok {
			break
		}

		v, err := it.Value()
		if err != nil {
			b.Fatal(err)
		}
		total += len(v)
		n++
	}

	if err := it.Close(); err != nil {
		b.Fatal(err)
	}
	if n != wantCount {
		b.Fatalf("iterated %d items, want %d", n, wantCount)
	}
	sink += total
}

// scanAppend is `scan`, but reading values through [corekv.ValueAppender] with a single
// buffer re-used across the whole iteration, so that the scan performs no per-value
// allocation. Iterators that do not implement the optional interface fall back to
// `Value`, making this workload meaningful (if identical to ScanAll) in every lane.
func scanAppend(b *testing.B, s corekv.TxnStore, opts corekv.IterOptions, wantCount int) {
	ctx := context.Background()

	it, err := s.Iterator(ctx, opts)
	if err != nil {
		b.Fatal(err)
	}

	appender, canAppend := it.(corekv.ValueAppender)
	buf := make([]byte, 0, 4096)

	n, total := 0, 0
	for {
		ok, err := it.Next()
		if err != nil {
			b.Fatal(err)
		}
		if !ok {
			break
		}

		var v []byte
		if canAppend {
			v, err = appender.AppendValue(buf[:0])
			// Retain any buffer growth for the next item.
			buf = v
		} else {
			v, err = it.Value()
		}
		if err != nil {
			b.Fatal(err)
		}
		total += len(v)
		n++
	}

	if err := it.Close(); err != nil {
		b.Fatal(err)
	}
	if n != wantCount {
		b.Fatalf("iterated %d items, want %d", n, wantCount)
	}
	sink += total
}

// scanBorrow is `scan`, but reading values through [corekv.ValueBorrower], so that the
// store copies nothing at all. Iterators that do not implement the optional interface
// fall back to `Value`, making this workload meaningful (if identical to ScanAll) in
// every lane.
//
// The value length is summed inside the callback and ends up in `sink`, the harness'
// black_box, so that neither the read nor the callback can be optimised away.
func scanBorrow(b *testing.B, s corekv.TxnStore, opts corekv.IterOptions, wantCount int) {
	ctx := context.Background()

	it, err := s.Iterator(ctx, opts)
	if err != nil {
		b.Fatal(err)
	}

	borrower, canBorrow := it.(corekv.ValueBorrower)

	n, total := 0, 0

	// The callback is built once, outside the loop: a function literal created inside
	// the loop would be heap-allocated on every iteration, which is exactly the cost
	// this workload exists to measure the absence of.
	accumulate := func(value []byte) error {
		total += len(value)
		return nil
	}

	for {
		ok, err := it.Next()
		if err != nil {
			b.Fatal(err)
		}
		if !ok {
			break
		}

		if canBorrow {
			err = borrower.BorrowValue(accumulate)
		} else {
			var v []byte
			v, err = it.Value()
			total += len(v)
		}
		if err != nil {
			b.Fatal(err)
		}
		n++
	}

	if err := it.Close(); err != nil {
		b.Fatal(err)
	}
	if n != wantCount {
		b.Fatalf("iterated %d items, want %d", n, wantCount)
	}
	sink += total
}

// prefill writes keys key(0)..key(n) with a value of the given size. It is always
// called outside of the timed region.
//
// Writes are grouped into transactions of roughly 256 KiB, which keeps prefill time
// sane without tripping badger's per-transaction size limit. The grouping is not
// observable in the measurements.
func prefill(tb testing.TB, s corekv.TxnStore, n int, valueSize int) {
	ctx := context.Background()
	val := value(valueSize)

	perTxn := (256 * 1024) / valueSize
	if perTxn < 1 {
		perTxn = 1
	}

	for start := 0; start < n; start += perTxn {
		end := min(start+perTxn, n)

		txn := s.NewTxn(false)
		for i := start; i < end; i++ {
			if err := txn.Set(ctx, key(uint64(i)), val); err != nil {
				txn.Discard()
				tb.Fatal(err)
			}
		}
		if err := txn.Commit(); err != nil {
			txn.Discard()
			tb.Fatal(err)
		}
	}
}

// --- TxnContended ------------------------------------------------------------------
//
// Every other workload in this suite is uncontended: transactions run one at a time, so
// no commit can ever conflict. TxnContended is the opposite, and it is the only workload
// whose headline number is not a latency. regolith's transactions are optimistic, so
// under contention a commit can fail validation and surface corekv.ErrTxnConflict to the
// caller, who must replay the whole transaction; badger uses serializable snapshot
// isolation and conflicts as well. What that costs is what this measures.
//
// It has no counterpart in rust-baseline/benches/workloads.rs, so it is outside the
// bit-for-bit fidelity contract above. Nothing it touches changes the existing
// workloads: the draws below come from their own newRng(Seed) walk.

const (
	// txnContendedWorkers is the number of goroutines committing concurrently. It is
	// parallelThreads, deliberately, so that TxnContended and ParallelMixed describe
	// the same amount of concurrency.
	txnContendedWorkers = parallelThreads

	// txnContendedTxnsPerWorker is the number of transactions each worker must get
	// *committed* per b.N iteration. Retried attempts do not count towards it.
	txnContendedTxnsPerWorker = 25

	// txnContendedReads / txnContendedWrites size one transaction's read-modify-write
	// over the hot range. Reads are what make a conflict possible at all under SSI,
	// writes are what make other transactions' reads conflict.
	txnContendedReads  = 4
	txnContendedWrites = 4

	// txnContendedMaxRetries caps the replays of a single logical transaction. A
	// livelock must fail the benchmark rather than hang it, so hitting the cap is
	// fatal and is reported as such.
	txnContendedMaxRetries = 100
)

// txnContendedHotSizes are the hot-range sizes registered as contention levels: 8 keys,
// where four workers writing four keys each collide almost every time, and 4096 keys,
// where they rarely meet. The hot range is the only knob; everything else is held equal
// between the two levels.
var txnContendedHotSizes = []int{8, 4096}

// txnContendedDraws are the seeded random draws that pick which hot keys each
// transaction touches: one draw per read and per write slot, for every worker and every
// transaction of an iteration. Taking them from a dedicated newRng(Seed) walk (rather
// than from `order`) keeps key selection identical in every lane and at every value
// size, while leaving the fidelity-pinned sequences in init() untouched. Scheduling is
// of course not deterministic - which transactions actually race is up to the runtime -
// but which keys they reach for is.
var txnContendedDraws []uint64

func init() {
	r := newRng(Seed)
	n := txnContendedWorkers * txnContendedTxnsPerWorker * (txnContendedReads + txnContendedWrites)
	txnContendedDraws = make([]uint64, n)
	for i := range txnContendedDraws {
		txnContendedDraws[i] = r.next()
	}

	for _, hot := range txnContendedHotSizes {
		workloads = append(workloads, txnContended(hot))
	}
}

// txnContended builds the TxnContended workload for one hot-range size.
//
// opsPerIter counts only the operations of transactions that *committed*: the unit of
// this benchmark is a successful commit, so the time spent on attempts that conflicted
// and were thrown away is charged to the surviving ones. That is what a caller actually
// pays, and it keeps ns/op-key comparable with every other row of the table.
func txnContended(hotN int) workload {
	return workload{
		name: fmt.Sprintf("TxnContendedHot%d", hotN),
		// The fixture is at least as large as the other transactional workloads', so
		// the hot range sits inside a store of a realistic size rather than in a
		// store that holds nothing else.
		prefillN:    max(hotN, batchWriteN),
		opsPerIter:  txnContendedWorkers * txnContendedTxnsPerWorker * (txnContendedReads + txnContendedWrites),
		movesValues: true,
		run: func(b *testing.B, s corekv.TxnStore, val []byte) {
			runTxnContended(b, s, val, hotN)
		},
	}
}

func runTxnContended(b *testing.B, s corekv.TxnStore, val []byte, hotN int) {
	ctx := context.Background()

	// attempts counts every Commit() call, commits only the ones that succeeded;
	// retries is the difference. Both are local to this call, so each of Go's
	// growing-b.N attempts reports the rate it actually measured.
	var attempts, commits atomic.Int64

	// b.Fatal off the benchmark's own goroutine only calls runtime.Goexit on that
	// goroutine: the benchmark would carry on with a worker silently missing. Workers
	// therefore record the first error and stop; the benchmark goroutine fails on it
	// below.
	var (
		errMu    sync.Mutex
		firstErr error
	)
	fail := func(err error) {
		errMu.Lock()
		defer errMu.Unlock()
		if firstErr == nil {
			firstErr = err
		}
	}
	failed := func() bool {
		errMu.Lock()
		defer errMu.Unlock()
		return firstErr != nil
	}

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for w := 0; w < txnContendedWorkers; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				for t := 0; t < txnContendedTxnsPerWorker; t++ {
					if failed() {
						return
					}

					base := (w*txnContendedTxnsPerWorker + t) * (txnContendedReads + txnContendedWrites)
					for retries := 0; ; retries++ {
						attempts.Add(1)
						err := txnContendedAttempt(ctx, s, val, hotN, base)
						if err == nil {
							commits.Add(1)
							break
						}
						if !errors.Is(err, corekv.ErrTxnConflict) {
							fail(err)
							return
						}
						// A conflict is the caller's problem to retry, from a
						// wholly fresh transaction - the conflicted one cannot
						// be reused.
						if retries >= txnContendedMaxRetries {
							fail(fmt.Errorf(
								"transaction livelocked: still conflicting after %d retries, hot range %d keys",
								txnContendedMaxRetries, hotN,
							))
							return
						}
					}
				}
			}(w)
		}
		wg.Wait()

		if failed() {
			break
		}
	}

	if firstErr != nil {
		b.Fatal(firstErr)
	}

	// The point of the whole workload. conflicts/attempt is retries / total attempts,
	// i.e. the fraction of transactions that had to be thrown away; retries/txn is the
	// same information per successful commit, which is the figure that explains the
	// ns/op-key.
	att, com := attempts.Load(), commits.Load()
	if want := int64(b.N) * txnContendedWorkers * txnContendedTxnsPerWorker; com != want {
		b.Fatalf("committed %d transactions, want %d", com, want)
	}
	retries := att - com
	b.ReportMetric(float64(retries)/float64(att), "conflicts/attempt")
	b.ReportMetric(float64(retries)/float64(com), "retries/txn")
}

// txnContendedAttempt runs one transaction: a read-modify-write over the hot range,
// followed by a commit. It returns corekv.ErrTxnConflict if the commit lost, in which
// case the caller must replay it from a fresh transaction.
//
// The transaction is discarded on every path, including the conflict and error ones -
// Discard after a successful Commit is a no-op in every store, and for regolith a
// transaction left open is not merely untidy.
func txnContendedAttempt(ctx context.Context, s corekv.TxnStore, val []byte, hotN, base int) error {
	txn := s.NewTxn(false)
	defer txn.Discard()

	for i := 0; i < txnContendedReads; i++ {
		v, err := txn.Get(ctx, presentKeys[txnContendedDraws[base+i]%uint64(hotN)])
		if err != nil {
			return err
		}
		// Stands in for the black-box `sink` the other workloads use: writing to
		// that global from four goroutines would be a data race.
		if len(v) != len(val) {
			return fmt.Errorf("unexpected value length: got %d, want %d", len(v), len(val))
		}
	}

	for i := 0; i < txnContendedWrites; i++ {
		k := presentKeys[txnContendedDraws[base+txnContendedReads+i]%uint64(hotN)]
		if err := txn.Set(ctx, k, val); err != nil {
			return err
		}
	}

	return txn.Commit()
}

package bench

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	badgerds "github.com/dgraph-io/badger/v4"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/badger"
	"github.com/sourcenetwork/corekv/memory"
)

// valueSizes are the two value sizes every workload is reported at, per the spec.
var valueSizes = []int{64, 4096}

func init() {
	// The memory store is not a contender - it is the sanity control. It should be the
	// fastest lane at everything; if it is not, the workload is measuring the harness.
	factories = append(factories, factory{
		name: "memory",
		new: func(tb testing.TB) corekv.TxnStore {
			s := memory.NewDatastore(context.Background())
			tb.Cleanup(func() {
				if err := s.Close(); err != nil {
					tb.Error(err)
				}
			})
			return s
		},
	})

	// badger is run ON DISK, in a temp dir, with its default options. regolith is a
	// disk engine, so comparing it against an in-memory badger would flatter it.
	// (test/action/new.go uses badger-in-memory - that is for correctness tests, not
	// for this.) Neither engine's knobs are tuned: defaults only, both lanes.
	factories = append(factories, factory{
		name: "badger",
		new: func(tb testing.TB) corekv.TxnStore {
			dir := tb.TempDir()
			s, err := badger.NewDatastore(dir, badgerds.DefaultOptions(dir))
			if err != nil {
				tb.Fatal(err)
			}
			tb.Cleanup(func() {
				if err := s.Close(); err != nil {
					tb.Error(err)
				}
			})
			return s
		},
	})
}

// TestMain tears down the stores shared between the read-only benchmarks, which by
// design outlive the benchmark that created them.
func TestMain(m *testing.M) {
	code := m.Run()
	closeSharedStores()
	os.Exit(code)
}

// sharedTB adapts a *testing.B so that resources it creates outlive that benchmark.
// Failures still go to the benchmark that triggered the construction, but TempDir and
// Cleanup are redirected to process-lifetime equivalents drained by TestMain.
type sharedTB struct {
	testing.TB
}

func (s sharedTB) TempDir() string {
	dir, err := os.MkdirTemp("", "corekv-bench-")
	if err != nil {
		s.Fatal(err)
	}
	sharedMu.Lock()
	defer sharedMu.Unlock()
	sharedCleanups = append(sharedCleanups, func() { _ = os.RemoveAll(dir) })
	return dir
}

func (s sharedTB) Cleanup(f func()) {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	sharedCleanups = append(sharedCleanups, f)
}

var (
	sharedMu       sync.Mutex
	sharedStores   = map[string]corekv.TxnStore{}
	sharedCleanups []func()
)

// sharedPrefilled returns the prefilled store for the given (lane, value size),
// building it on first use. It is only ever handed to read-only workloads, so sharing
// cannot leak state between benchmarks; it exists because prefilling 100k keys per
// benchmark attempt would dominate the suite's runtime.
func sharedPrefilled(b *testing.B, f factory, valueSize int) corekv.TxnStore {
	cacheKey := fmt.Sprintf("%s/v%d", f.name, valueSize)

	sharedMu.Lock()
	s, ok := sharedStores[cacheKey]
	sharedMu.Unlock()
	if ok {
		return s
	}

	s = f.new(sharedTB{b})
	prefill(b, s, PrefillCount, valueSize)

	sharedMu.Lock()
	defer sharedMu.Unlock()
	sharedStores[cacheKey] = s
	return s
}

func closeSharedStores() {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	for _, f := range sharedCleanups {
		f()
	}
	sharedCleanups = nil
	sharedStores = map[string]corekv.TxnStore{}
}

// runWorkload runs the named workload against every registered lane at every value
// size, producing benchmark names of the form `BenchmarkGetHit/badger/v64`.
func runWorkload(b *testing.B, name string) {
	w, ok := lookupWorkload(name)
	if !ok {
		b.Fatalf("unknown workload %q", name)
	}
	if len(factories) == 0 {
		b.Fatal("no store factories registered")
	}

	for _, f := range factories {
		b.Run(f.name, func(b *testing.B) {
			for _, valueSize := range valueSizes {
				b.Run(fmt.Sprintf("v%d", valueSize), func(b *testing.B) {
					runLane(b, f, w, valueSize)
				})
			}
		})
	}
}

func runLane(b *testing.B, f factory, w workload, valueSize int) {
	// Everything up to ResetTimer is setup and must not be measured.
	var s corekv.TxnStore
	switch {
	case w.readOnly:
		s = sharedPrefilled(b, f, valueSize)
	default:
		s = f.new(b)
		if w.prefillN > 0 {
			prefill(b, s, w.prefillN, valueSize)
		}
	}
	val := value(valueSize)

	if w.movesValues {
		b.SetBytes(int64(w.opsPerIter) * int64(valueSize))
	}
	b.ReportAllocs()

	b.ResetTimer()
	w.run(b, s, val)
	b.StopTimer()

	// The default ns/op covers a whole iteration, which for most workloads is many
	// keys. ns/op-key is the per-key figure the three lanes are compared on.
	ops := float64(b.N) * float64(w.opsPerIter)
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/ops, "ns/op-key")
}

func lookupWorkload(name string) (workload, bool) {
	for _, w := range workloads {
		if w.name == name {
			return w, true
		}
	}
	return workload{}, false
}

// One top-level benchmark per workload, so that names are stable and greppable.

func BenchmarkSeqWrite(b *testing.B)      { runWorkload(b, "SeqWrite") }
func BenchmarkRandWrite(b *testing.B)     { runWorkload(b, "RandWrite") }
func BenchmarkGetHit(b *testing.B)        { runWorkload(b, "GetHit") }
func BenchmarkGetMiss(b *testing.B)       { runWorkload(b, "GetMiss") }
func BenchmarkHas(b *testing.B)           { runWorkload(b, "Has") }
func BenchmarkScanAll(b *testing.B)       { runWorkload(b, "ScanAll") }
func BenchmarkScanReverse(b *testing.B)   { runWorkload(b, "ScanReverse") }
func BenchmarkScanPrefix(b *testing.B)    { runWorkload(b, "ScanPrefix") }
func BenchmarkTxnWrite(b *testing.B)      { runWorkload(b, "TxnWrite") }
func BenchmarkTxnReadWrite(b *testing.B)  { runWorkload(b, "TxnReadWrite") }
func BenchmarkBatchWrite(b *testing.B)    { runWorkload(b, "BatchWrite") }
func BenchmarkParallelMixed(b *testing.B) { runWorkload(b, "ParallelMixed") }

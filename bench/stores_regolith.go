//go:build regolith

// This file is the seam for the regolith lane. The default build does not include it,
// so the suite compiles and runs with only the badger and memory lanes. Build with
// `-tags regolith` (and after `make -C ../../go-regolith ffi`, which produces the Rust
// staticlib this links against).
//
// The cgo preamble below is here, rather than in a separate file, because the
// regolithNoop wrapper must be a single C call with nothing else in its body: the
// BenchmarkFFINoop figure is the floor of any cgo call, so a defer or an allocation in
// the wrapper would land in the measurement.
package bench

// #cgo CFLAGS: -I${SRCDIR}/../../go-regolith/ffi/include
// #cgo LDFLAGS: ${SRCDIR}/../../go-regolith/ffi/target/release/libregolith_ffi.a
// #include "regolith_ffi.h"
import "C"

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/regolith"
)

func init() {
	factories = append(factories, factory{
		name: "regolith",
		new: func(tb testing.TB) corekv.TxnStore {
			// On disk, engine defaults. regolith's own Options are not exposed across
			// the FFI, so NewDatastore takes no options parameter - the defaults
			// (OptimisticTransactionDb + SnapshotIsolation + DurabilityMode::Eventual)
			// are always used. Nothing is tuned, matching the untuned badger lane.
			dir := tb.TempDir()
			s, err := regolith.NewDatastore(dir)
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

	regolithNoop = func() { C.regolith_noop() }
}

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
	goregolith "github.com/sourcenetwork/go-regolith"
)

// regolithLanes registers one lane per engine configuration under test, so that
// configurations are compared within a single run rather than across runs, where
// machine state drifts.
//
//   - regolith          engine defaults, the baseline every recorded figure used
//   - regolith-ser      serializable isolation, comparable with badger's SSI
//   - regolith-inline   transaction_keys_inline raised past the largest transaction
//
// The default lane must keep passing nil, so it stays comparable with the untuned
// badger lane and with the numbers already published for it.
func init() {
	lanes := []struct {
		name string
		opts *goregolith.Options
	}{
		{"regolith", nil},
		{"regolith-ser", &goregolith.Options{Isolation: goregolith.IsolationSerializable}},
		// The transaction write buffer indexes itself past this many keys, cloning
		// the key on every later insert. A transaction that only writes never reads
		// that index back, so above the largest transaction in the suite (1000 keys)
		// the indexing is pure cost.
		{"regolith-inline", &goregolith.Options{TransactionKeysInline: goregolith.Uint64(4096)}},
	}

	for _, lane := range lanes {
		factories = append(factories, factory{
			name: lane.name,
			new: func(tb testing.TB) corekv.TxnStore {
				dir := tb.TempDir()
				s, err := regolith.NewDatastore(dir, lane.opts)
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

	regolithNoop = func() { C.regolith_noop() }
}

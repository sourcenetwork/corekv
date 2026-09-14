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

// The regolith lane runs the configuration we would actually ship.
//
//   - Serializable isolation, matching badger's serializable snapshot isolation.
//     It costs nothing on uncontended work (measured within noise on every such
//     workload) and it is what makes the contended comparison honest: regolith's
//     default snapshot isolation conflicts less than badger only because it
//     validates less, admitting write skew that badger rejects.
//   - transaction_keys_inline above the largest transaction in the suite. Past
//     this many keys the transaction write buffer indexes itself, cloning the key
//     on every later insert, and a transaction that only writes never reads that
//     index back. Worth about 23% on transactional writes.
func init() {
	factories = append(factories, factory{
		name: "regolith",
		new: func(tb testing.TB) corekv.TxnStore {
			dir := tb.TempDir()
			s, err := regolith.NewDatastore(dir, &goregolith.Options{
				Isolation:             goregolith.IsolationSerializable,
				TransactionKeysInline: goregolith.Uint64(4096),
			})
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

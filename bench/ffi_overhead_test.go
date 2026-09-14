package bench

import "testing"

// BenchmarkFFINoop measures the raw cost of a single cgo call into the regolith FFI
// layer: `regolith_noop()` is a C function that does nothing at all, so this is the
// floor on what any Go-through-FFI store operation can cost. Subtracting it from the
// go-ffi lane's per-op numbers separates "cgo call overhead" from "marshalling + copy".
//
// It skips unless the regolith lane is present (`-tags regolith`), which is what sets
// regolithNoop.
//
// TODO(regolith-lane): this needs no change here. Just set regolithNoop in
// stores_regolith.go to a function whose entire body is the cgo call, e.g.
//
//	//go:build regolith
//	package bench
//	// #cgo LDFLAGS: ...
//	// #include "regolith_ffi.h"
//	import "C"
//	func init() { regolithNoop = func() { C.regolith_noop() } }
//
// Keep the wrapper body to exactly that one call - anything else (a defer, an
// allocation, a bounds check) lands in the measurement and corrupts the delta.
func BenchmarkFFINoop(b *testing.B) {
	if regolithNoop == nil {
		b.Skip("regolith lane absent; rebuild with -tags regolith")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		regolithNoop()
	}
	b.StopTimer()

	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N), "ns/op-key")
}

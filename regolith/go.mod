module github.com/sourcenetwork/corekv/regolith

go 1.24.6

require (
	github.com/sourcenetwork/corekv v0.0.0
	github.com/sourcenetwork/go-regolith v0.0.0
)

replace github.com/sourcenetwork/corekv v0.0.0 => ./..

// go-regolith is cgo over a Rust staticlib that its own `make ffi` builds into
// `ffi/target/release/libregolith_ffi.a` inside the module directory.  That
// directory is read-only in the module cache, so the staticlib cannot be built
// where a versioned dependency would live and consuming go-regolith by version
// does not currently work.  A sibling checkout is therefore required, and
// `make -C ../../go-regolith ffi` must have been run before any build here.
// This is a known limitation of go-regolith, tracked separately.
replace github.com/sourcenetwork/go-regolith v0.0.0 => ../../go-regolith

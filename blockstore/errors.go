package blockstore

import "errors"

var (
	// ErrHashMismatch is an error returned when the hash of a block is different than expected.
	ErrHashMismatch = errors.New("block in storage has different hash than requested")
)

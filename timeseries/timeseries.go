package timeseries

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/sourcenetwork/corekv"
)

/*
#cgo CFLAGS: -g -O2 -I${SRCDIR}/libs/
#cgo LDFLAGS: -L${SRCDIR}/libs -labi

#include <stdlib.h>
#include <abi.h>
*/
import "C"

type Store struct {
	indexStore   corekv.Store
	location     string
	startMs      uint64 //unix ms
	resolutionMs uint64
}

var _ corekv.Store = (*Store)(nil)

func New(location string, start time.Time, resolution time.Duration) *Store {
	maxSize := []uint64{100000000000}

	C.create_db(
		getCString(location),
		C.Vec_uint64_t{
			ptr: (*C.uint64_t)(&maxSize[0]),
			len: C.size_t(len(maxSize)),
		},
	)

	return &Store{
		location:     location,
		startMs:      uint64(start.UnixMilli()),
		resolutionMs: uint64(resolution.Milliseconds()),
	}
}

func (s *Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	location := s.getLocation(key)

	result := C.read_value(
		C.Store_t{
			location:      getCString(s.location),
			start_ms:      C.uint64_t(s.startMs),
			resolution_ms: C.uint64_t(s.resolutionMs),
		},
		C.Vec_uint64_t{
			ptr: (*C.uint64_t)(&location[0]),
			len: C.size_t(len(location)), //todo - this can be computed in the constructor
		},
	)

	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, (uint64)(result))

	return buf, nil
}

func (s *Store) Has(ctx context.Context, key []byte) (bool, error) {
	// if we store not-optional values, every key has a value.
	//
	// however... we may decide to only store optional values, in which case we'll need to check
	return true, nil
}

func (s *Store) Iterator(ctx context.Context, opts corekv.IterOptions) (corekv.Iterator, error) {
	panic("todo")
}

func (s *Store) Set(ctx context.Context, key, value []byte) error {
	location := s.getLocation(key)

	C.write_value(
		C.Store_t{
			location:      getCString(s.location),
			start_ms:      C.uint64_t(s.startMs),
			resolution_ms: C.uint64_t(s.resolutionMs),
		},
		C.Vec_uint64_t{
			ptr: (*C.uint64_t)(&location[0]),
			len: C.size_t(len(location)), //todo - this can be computed in the constructor
		},
		C.uint64_t(binary.LittleEndian.Uint64(value)),
	)

	return nil
}

func (s *Store) Delete(ctx context.Context, key []byte) error {
	panic("todo")
}

func (s *Store) Close() error {
	panic("todo")
}

func getCString(v string) C.Vec_uint8_t {
	locationBytes := []byte(v)
	return C.Vec_uint8_t{
		ptr: (*C.uint8_t)(&locationBytes[0]),
		len: C.size_t(len(locationBytes)),
	}
}

func (s *Store) getLocation(key []byte) []uint64 {
	return []uint64{binary.LittleEndian.Uint64(key)}
	/* This is incorrect, as encoded uint64 can have `/`... redo/rethink
	keyElements := bytes.Split(key, []byte("/"))
	location := make([]uint64, 0, len(keyElements))

	for i, keyElement := range keyElements { // todo - last must be datetime!
		if i == len(keyElements)-1 {
			unixMSeconds := binary.LittleEndian.Uint64(keyElement) //todo - we can go as small as nano seconds
			location = append(
				location,
				unixMSeconds,
			)
		} else {
			// todo - handle strings using reg key-value store as sec. index? Could be important for Defra doc-keys
			// this would make array max sizes much more important if using the same db...
			//
			// question: Is having large number of files (i.e. 1 per doc) problematic?
			//
			// the ax and key files could be shared across all docs in the same col
			//
			// arrays can like against one defra doc. each index can be of any type, if not integer is can be aliased
			// - this includes foriegn keys!
			location = append(
				location,
				binary.LittleEndian.Uint64(keyElement), // warning: this requires exactly 8 bytes
			)
		}
	}

	return location
	*/
}

// CRDT thoughts:
/*
- must be configurable locally, not globally
- blocks cant be mutated

- timestamps, or at least start timestamp must be included in the block.  So must any array indexes in their user-visible form (no secondary index aliases)

- could construct the block in memory, allowing it to collect updates until a certain threshold has been met,
  at which point it will be commited and transmitted
	- timeseries/array/crdt specific
	- real-time-ish sync not possible, unless commit size==1
	- fairly efficient, user handles when it gets commited/pushed not us, although that might involve them
	  pushing nil values if they want it synced early? (could eventually allow users to manually trigger commit)
	- could commit/sync on timeout, but that puts more work on db
	- risk of data loss on db restart if block in memory only (could persist it on disk while constructing though, longer-term-dev)

- could tie it to external transactions, if multiple blocks are added in a single transaction they get merged into one on commit
	- would work for all stuff
	- requires user to manually manage transaction
	- is not mutually exclusive with first option
	- no way to avoid uncommited data loss on db restart

- could allow blocks to be removed from local blockstore when certain conditions are met
	- would work for all stuff
	- increases risk of user permanently losing their blocks
	- no risk to data on resart
	- very handy when paired with archive node(s)

- remember different use-cases:
	- Might only care about latest value in datastore, with the history in the blocks (not really timeseries)
	- Might only care about latest window in datastore, with much of the history in the blocks (limited timeseries)
	- Might want entire history in datastore (full timeseries)
	- These will be node specific (local config required)

- Probably makes sense to define a time-value GQL type (used when on latest is in datastore?), then timeseries becomes an array of them

A good initial use case would be a source node which locally only cares about the current value (or no reads), broadcasting kvp blocks
to aggregate node, which cares about entire history.

thought: aggregate node likely cares about multiple sources - how should we combine them?  Perhaps views joining multiple docs?
	- single doc shared by all souces not an option as that would create merge hell and waste silly amounts of space and compute
	- a view would probably not be computationally efficient unless individual docs did not store entire history
		- question: would there be demand for config on a per-doc basis?

- timeseries store can be circle buffer (or perhaps that is a new store implementation), allowing very efficient windows
*/

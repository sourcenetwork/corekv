package array

import (
	"bytes"
	"context"
	"encoding/binary"
	"unsafe"

	"github.com/sourcenetwork/corekv"
)

/*
#cgo CFLAGS: -g -O2 -I${SRCDIR}/libs/
#cgo LDFLAGS: -L${SRCDIR}/libs -labi

#include <abi.h>
*/
import "C"

type Store struct{}

var _ corekv.Store = (*Store)(nil)

func (s *Store) Get(ctx context.Context, key []byte) ([]byte, error) {
	keyElements := bytes.Split(key, []byte("/"))
	location := make([]uint64, len(keyElements))
	for _, keyElement := range keyElements {
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

	result := C.get(C.Vec_uint64_t{
		ptr: (*C.uint64_t)(&location[0]),
		len: C.size_t(len(location)), //todo - this can be computed in the constructor
	})

	data := unsafe.Slice((*byte)(result.ptr), result.len)
	return data, nil
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
	panic("todo")
}

func (s *Store) Delete(ctx context.Context, key []byte) error {
	panic("todo")
}

func (s *Store) Close() error {
	panic("todo")
}

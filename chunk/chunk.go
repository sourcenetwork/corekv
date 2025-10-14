package chunk

import (
	"bytes"
	"context"
	"errors"
	"slices"

	"github.com/sourcenetwork/corekv"
)

// Chunkstore is a corekv store that wraps other corekv ReaderWriters and
// stores given values as 'chunks' with a value byte length of up `chunkSize`.
//
// This allows the storage of very large values, regardless of the underlying
// store's ability.  For example, the badger-memory store only accepts values
// of 1MB or smaller, wrapping the store in a Chunkstore allows values of any
// size to be held by it.
//
// It may only accept keys of a single length.
type Chunkstore struct {
	store     corekv.ReaderWriter
	chunkSize int
	keyLen    int
}

var _ corekv.ReaderWriter = (*Chunkstore)(nil)

// New wraps the given ReaderWriter in a new Chunkstore.
//
// The keyLen is determined dynamically.  If the store already contains values,
// keyLen will be set to the length of the first iterated value minus one (it
// assumes chunkstore key format).
func New(
	ctx context.Context,
	store corekv.ReaderWriter,
	chunkSize int,
) (*Chunkstore, error) {
	it, err := store.Iterator(ctx, corekv.IterOptions{
		KeysOnly: true,
	})
	if err != nil {
		return nil, err
	}

	hasKey, err := it.Next()
	if err != nil {
		return nil, errors.Join(err, it.Close())
	}

	var keyLen int
	if hasKey {
		key := it.Key()
		// Assume the key is for the first chunk in an external key, and trim the 0-index from
		// the length.
		keyLen = len(key) - 1
	}

	err = it.Close()
	if err != nil {
		return nil, err
	}

	return &Chunkstore{
		store:     store,
		chunkSize: chunkSize,
		keyLen:    keyLen,
	}, nil
}

// NewSized creates a new Chunkstore instance with keys of a known
// length.
func NewSized(
	store corekv.ReaderWriter,
	chunkSize int,
	keyLen int,
) *Chunkstore {
	return &Chunkstore{
		store:     store,
		chunkSize: chunkSize,
		keyLen:    keyLen,
	}
}

func (s *Chunkstore) Get(ctx context.Context, key []byte) ([]byte, error) {
	it, err := s.store.Iterator(ctx, corekv.IterOptions{
		Prefix: key,
	})
	if err != nil {
		return nil, err
	}

	var chunks [][]byte
	for {
		hasNext, err := it.Next()
		if err != nil {
			return nil, errors.Join(err, it.Close())
		}

		if !hasNext {
			err = it.Close()
			if err != nil {
				return nil, err
			}

			if len(chunks) == 0 {
				return nil, corekv.ErrNotFound
			}

			return bytes.Join(chunks, []byte{}), nil
		}

		chunk, err := it.Value()
		if err != nil {
			return nil, errors.Join(err, it.Close())
		}

		chunks = append(chunks, chunk)
	}
}

func (s *Chunkstore) Has(ctx context.Context, key []byte) (bool, error) {
	it, err := s.store.Iterator(ctx, corekv.IterOptions{
		Prefix:   key,
		KeysOnly: true,
	})
	if err != nil {
		return false, err
	}

	hasNext, err := it.Next()
	if err != nil {
		return false, errors.Join(err, it.Close())
	}

	return hasNext, it.Close()
}

func (s *Chunkstore) Set(ctx context.Context, key []byte, value []byte) error {
	// Append a zero-byte to the end of the key, all keys must have at least a zero-suffix,
	// this makes deconstructing them a lot simpler.
	chunkKey := append(key, byte(0))

	if len(value) == 0 {
		// If an empty value has been provided we don't need to chunk, but we do want the
		// chunkKey suffix (for now) in order to simplify other functions.
		err := s.store.Set(ctx, chunkKey, value)
		if err != nil {
			return err
		}
	} else {
		for chunk := range slices.Chunk(value, s.chunkSize) {
			err := s.store.Set(ctx, chunkKey, chunk)
			if err != nil {
				return err
			}

			// TODO - this is incorrect beyond 256 chunks! It has been commited as-is as we need this store
			// asap and this limit is unlikely to impact the immediate use-case.
			//
			// https://github.com/sourcenetwork/corekv/issues/93
			chunkKey = bytesPrefixEnd(chunkKey)
		}
	}

	if s.keyLen == 0 {
		// If this is the first write, set the length of keys
		s.keyLen = len(key)
	}

	return nil
}

func (s *Chunkstore) Delete(ctx context.Context, key []byte) error {
	it, err := s.store.Iterator(ctx, corekv.IterOptions{
		Prefix:   key,
		KeysOnly: true,
	})
	if err != nil {
		return err
	}

	keys := [][]byte{}
	for {
		hasNext, err := it.Next()
		if err != nil {
			return errors.Join(err, it.Close())
		}

		if !hasNext {
			break
		}

		chunkKey := it.Key()
		keys = append(keys, chunkKey)

	}

	err = it.Close()
	if err != nil {
		return err
	}

	// Deletion must be done after iteration as the memory store will deadlock if writing during iteration
	for _, key := range keys {
		err = s.store.Delete(ctx, key)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Chunkstore) Iterator(ctx context.Context, opts corekv.IterOptions) (corekv.Iterator, error) {
	inner, err := s.store.Iterator(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &iterator{
		it:     inner,
		opts:   opts,
		keyLen: s.keyLen,
	}, nil
}

type iterator struct {
	it     corekv.Iterator
	opts   corekv.IterOptions
	keyLen int

	// The current, full key (not a chunk key)
	currentKey []byte

	// The current, full value
	currentValue []byte

	currentChunk    []byte
	currentChunkKey []byte
}

var _ corekv.Iterator = (*iterator)(nil)

func (it *iterator) Reset() {
	it.currentKey = nil
	it.currentValue = nil
	it.currentChunk = nil
	it.currentChunkKey = nil

	it.it.Reset()
}

func (it *iterator) Next() (bool, error) {
	var chunks [][]byte
	for {
		if it.currentChunkKey != nil {
			if it.opts.Reverse {
				chunks = append([][]byte{it.currentChunk}, chunks...)
			} else {
				chunks = append(chunks, it.currentChunk)
			}
			it.currentKey = it.currentChunkKey[:it.keyLen]
			it.currentChunkKey = nil
			it.currentChunk = nil
		}

		hasNext, err := it.it.Next()
		if err != nil {
			return false, err
		}

		if !hasNext {
			break
		}

		it.currentChunkKey = it.it.Key()
		it.currentChunk, err = it.it.Value()
		if err != nil {
			return false, err
		}

		if !bytes.HasPrefix(it.currentChunkKey, it.currentKey) {
			break
		}
	}

	if len(chunks) == 0 {
		return false, nil
	}

	it.currentValue = bytes.Join(chunks, []byte{})

	return true, nil
}

func (it *iterator) Key() []byte {
	return it.currentKey
}

func (it *iterator) Value() ([]byte, error) {
	return it.currentValue, nil
}

func (it *iterator) Seek(key []byte) (bool, error) {
	if it.opts.Reverse {
		// Because the keys in the underlying store have been suffixed, would-be exact matches
		// are actually larger than the key and would get missed.  Because of this, if we are
		// reverse-iterating we need to increment the Seek-key.
		key = bytesPrefixEnd(key)
	}

	hasValue, err := it.it.Seek(key)
	if err != nil {
		return false, err
	}
	if !hasValue {
		return false, nil
	}

	it.currentChunkKey = it.it.Key()
	it.currentChunk, err = it.it.Value()
	if err != nil {
		return false, err
	}

	return it.Next()
}

func (it *iterator) Close() error {
	return it.it.Close()
}

func bytesPrefixEnd(b []byte) []byte {
	end := make([]byte, len(b))
	copy(end, b)
	for i := len(end) - 1; i >= 0; i-- {
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	// This statement will only be reached if the key is already a
	// maximal byte string (i.e. already \xff...).
	return b
}

package encrypted

import (
	"context"

	"github.com/sourcenetwork/corekv"
)

// encryptedStore wraps a store and encrypts / decrypts stored values
type encryptedStore struct {
	store corekv.Store
	mode  Mode
}

// Wrap returns a store that encrypts and decrypts values using
// the given mode.
func Wrap(store corekv.Store, mode Mode) corekv.Store {
	return &encryptedStore{
		store: store,
		mode:  mode,
	}
}

func (s *encryptedStore) Set(ctx context.Context, key, value []byte) error {
	if len(value) > 0 {
		return s.store.Set(ctx, key, s.mode.Encrypt(value))
	}
	return s.store.Set(ctx, key, value)
}

func (s *encryptedStore) Delete(ctx context.Context, key []byte) error {
	return s.store.Delete(ctx, key)
}

func (s *encryptedStore) Get(ctx context.Context, key []byte) ([]byte, error) {
	value, err := s.store.Get(ctx, key)
	if err != nil {
		return []byte(nil), err
	}
	if len(value) > 0 {
		return s.mode.Decrypt(value), nil
	}
	return []byte(nil), nil
}

func (s *encryptedStore) Has(ctx context.Context, key []byte) (bool, error) {
	return s.store.Has(ctx, key)
}

func (s *encryptedStore) Iterator(ctx context.Context, opts corekv.IterOptions) corekv.Iterator {
	return newEncryptedIter(s.store.Iterator(ctx, opts), s.mode)
}

func (s *encryptedStore) Close() {
	s.store.Close()
}

package encrypted

import (
	"context"

	"github.com/sourcenetwork/corekv"
)

type encryptedIter struct {
	inner corekv.Iterator
	mode  Mode
}

func newEncryptedIter(inner corekv.Iterator, mode Mode) *encryptedIter {
	return &encryptedIter{
		inner: inner,
		mode:  mode,
	}
}

func (i *encryptedIter) Domain() (start []byte, end []byte) {
	return i.inner.Domain()
}

func (i *encryptedIter) Valid() bool {
	return i.inner.Valid()
}

func (i *encryptedIter) Next() {
	i.inner.Next()
}

func (i *encryptedIter) Key() []byte {
	return i.inner.Key()
}

func (i *encryptedIter) Value() []byte {
	return i.mode.Decrypt(i.inner.Value())
}

func (i *encryptedIter) Seek(key []byte) {
	i.inner.Seek(key)
}

func (i *encryptedIter) Close(ctx context.Context) error {
	return i.inner.Close(ctx)
}

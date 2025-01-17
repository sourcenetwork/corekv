package encrypted

import (
	"context"
	"crypto/aes"
	"crypto/rand"
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/memory"
	kvtest "github.com/sourcenetwork/corekv/test"
	"github.com/stretchr/testify/require"
)

func TestSuiteWithCFBMode(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	block, err := aes.NewCipher(key)
	require.NoError(t, err)

	iv := make([]byte, block.BlockSize())
	_, err = rand.Read(key)
	require.NoError(t, err)

	var store corekv.Store
	store = memory.NewDatastore(context.Background())
	store = Wrap(store, WithCFBMode(block, iv))

	t.Cleanup(store.Close)
	kvtest.SubtestAll(t, store)
}

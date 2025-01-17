//go:build js

package indexed_db

import (
	"testing"

	kvtest "github.com/sourcenetwork/corekv/test"
	"github.com/stretchr/testify/require"
)

func TestSuite(t *testing.T) {
	store, err := NewDatastore("test")
	require.NoError(t, err)

	t.Cleanup(store.Close)
	kvtest.SubtestAll(t, store)
}

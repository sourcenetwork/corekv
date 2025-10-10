package action

import (
	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/chunk"
	"github.com/sourcenetwork/corekv/test/state"
	"github.com/stretchr/testify/require"
)

// ChunkStore action will chunk the current store (replacing it) with the
// given chunk store when executed.
type ChunkStore struct {
	ChunkSize int
}

var _ Action = (*ChunkStore)(nil)

func Chunk(chunkSize int) *ChunkStore {
	return &ChunkStore{
		ChunkSize: chunkSize,
	}
}

func (a *ChunkStore) Execute(s *state.State) {
	var err error
	switch store := s.Store.(type) {
	case corekv.Txn:
		s.Store, err = chunk.NewTxn(s.Ctx, store, a.ChunkSize)

	case corekv.TxnReaderWriter:
		s.Store, err = chunk.NewTS(s.Ctx, store, a.ChunkSize)

	default:
		s.Store, err = chunk.New(s.Ctx, store, a.ChunkSize)

	}

	require.NoError(s.T, err)
}

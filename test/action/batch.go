package action

import (
	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/state"
)

// WriteBatchOps action will apply the given operations to the store in one batch if the
// store supports it.  Otherwise skips the test.
type WriteBatchOps struct {
	Ops           []corekv.BatchOp
	ExpectedError string
}

var _ Action = (*WriteBatchOps)(nil)

// WriteBatch returns a new WriteBatchOps action that will apply the given operations to
// the store in one batch when executed.
func WriteBatch(ops ...corekv.BatchOp) *WriteBatchOps {
	return &WriteBatchOps{Ops: ops}
}

// WriteBatchE returns a new WriteBatchOps action that will apply the given operations to
// the store in one batch when executed, and require that the returned error contains the
// given string.
func WriteBatchE(expectedErr string, ops ...corekv.BatchOp) *WriteBatchOps {
	return &WriteBatchOps{Ops: ops, ExpectedError: expectedErr}
}

func (a *WriteBatchOps) Execute(s *state.State) {
	writer, ok := s.Store.(corekv.BatchWriter)
	if !ok {
		s.T.Skipf("Store does not support WriteBatch, test is irrelevant")
	}

	err := writer.WriteBatch(s.Ctx, a.Ops)
	expectError(s, err, a.ExpectedError)
}

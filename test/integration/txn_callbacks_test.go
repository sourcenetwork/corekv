package integration

import (
	"sync/atomic"
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/stretchr/testify/assert"
)

func TestTxnCommit_WithSuccessCallback(t *testing.T) {
	var callbackCount atomic.Uint64
	test := &Test{
		Actions: []action.Action{
			action.NewTxn(),
			action.OnSuccess(func() { callbackCount.Add(1) }),
			action.OnDiscard(func() { callbackCount.Add(3) }),
			action.OnError(func() { callbackCount.Add(5) }),
			action.Commit(),
		},
	}

	test.Execute(t)
	assert.Equal(t, uint64(1), callbackCount.Load())
}

func TestTxnDiscard_WithDiscardCallback(t *testing.T) {
	var callbackCount atomic.Uint64
	test := &Test{
		Actions: []action.Action{
			action.NewTxn(),
			action.OnSuccess(func() { callbackCount.Add(1) }),
			action.OnDiscard(func() { callbackCount.Add(3) }),
			action.OnError(func() { callbackCount.Add(5) }),
			action.Discard(),
		},
	}

	test.Execute(t)
	assert.Equal(t, uint64(3), callbackCount.Load())
}

func TestTxnCommitThenDiscard_WithSuccessAndDiscardCallback(t *testing.T) {
	var callbackCount atomic.Uint64
	test := &Test{
		Actions: []action.Action{
			action.NewTxn(),
			action.OnSuccess(func() { callbackCount.Add(1) }),
			action.OnDiscard(func() { callbackCount.Add(3) }),
			action.OnError(func() { callbackCount.Add(5) }),
			action.Commit(),
			action.Discard(),
		},
	}

	test.Execute(t)
	assert.Equal(t, uint64(4), callbackCount.Load())
}

func TestTxnCommit_WithErrorCallback(t *testing.T) {
	var callbackCount atomic.Uint64
	test := &Test{
		Actions: []action.Action{
			action.NewTxn(),
			action.OnSuccess(func() { callbackCount.Add(1) }),
			action.OnDiscard(func() { callbackCount.Add(3) }),
			action.OnError(func() { callbackCount.Add(5) }),
			action.WithTxn(action.Set([]byte("k1"), []byte("v1"))),
			action.Close(),
			action.CommitE(corekv.ErrDBClosed.Error()),
		},
	}

	test.Execute(t)
	assert.Equal(t, uint64(5), callbackCount.Load())
}

func TestTxnCommitThenDiscard_WithDiscardAndErrorCallback(t *testing.T) {
	var callbackCount atomic.Uint64
	test := &Test{
		Actions: []action.Action{
			action.NewTxn(),
			action.OnSuccess(func() { callbackCount.Add(1) }),
			action.OnDiscard(func() { callbackCount.Add(3) }),
			action.OnError(func() { callbackCount.Add(5) }),
			action.WithTxn(action.Set([]byte("k1"), []byte("v1"))),
			action.Close(),
			action.CommitE(corekv.ErrDBClosed.Error()),
			action.Discard(),
		},
	}

	test.Execute(t)
	assert.Equal(t, uint64(8), callbackCount.Load())
}

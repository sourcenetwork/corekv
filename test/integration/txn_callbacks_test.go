package integration

import (
	"sync/atomic"
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/stretchr/testify/assert"
)

func TestTxnCommitSuccessCallbacks(t *testing.T) {
	var callbackCount atomic.Uint64
	test := &Test{
		Actions: []action.Action{
			action.NewTxn(),
			action.OnSuccess(func() { callbackCount.Add(1) }),
			action.OnDiscard(func() { callbackCount.Add(1) }),
			action.OnError(func() { callbackCount.Add(1) }),
			action.Commit(),
		},
	}

	test.Execute(t)
	assert.Equal(t, uint64(1), callbackCount.Load())
}

func TestTxnDiscardCallbacks(t *testing.T) {
	var callbackCount atomic.Uint64
	test := &Test{
		Actions: []action.Action{
			action.NewTxn(),
			action.OnSuccess(func() { callbackCount.Add(1) }),
			action.OnDiscard(func() { callbackCount.Add(1) }),
			action.OnError(func() { callbackCount.Add(1) }),
			action.Discard(),
		},
	}

	test.Execute(t)
	assert.Equal(t, uint64(1), callbackCount.Load())
}

func TestTxnCommitErrorCallbacks(t *testing.T) {
	var callbackCount atomic.Uint64
	test := &Test{
		Actions: []action.Action{
			action.NewTxn(),
			action.OnSuccess(func() { callbackCount.Add(1) }),
			action.OnDiscard(func() { callbackCount.Add(1) }),
			action.OnError(func() { callbackCount.Add(1) }),
			action.WithTxn(action.Set([]byte("k1"), []byte("v1"))),
			action.Close(),
			action.CommitE(corekv.ErrDBClosed.Error()),
		},
	}

	test.Execute(t)
	assert.Equal(t, uint64(1), callbackCount.Load())
}

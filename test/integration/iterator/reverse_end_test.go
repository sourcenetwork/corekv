package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
)

func TestIteratorReverseEnd(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k1"), []byte("v1")),
			action.Set([]byte("k3"), nil),
			action.Set([]byte("k4"), []byte("v4")),
			action.Set([]byte("k2"), []byte("v2")),
			&action.Iterate{
				IterOptions: corekv.IterOptions{
					Reverse: true,
					End:     []byte("k3"),
				},
				Expected: []action.KeyValue{
					// `k3` and `k4` must not be yielded
					{Key: []byte("k2"), Value: []byte("v2")},
					{Key: []byte("k1"), Value: []byte("v1")},
				},
			},
		},
	}

	test.Execute(t)
}

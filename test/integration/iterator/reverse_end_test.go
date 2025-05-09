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

// Due to the way the memory iterator is coded in `Seek` it would not be impossible for a coding mistake to
// result in store with a single item greater/equal to than the iterator `End` to be erroneously yieled when
// reversing.
func TestIteratorReverseEnd_SingleItemOutOfBounds(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("k3"), nil),
			&action.Iterate{
				IterOptions: corekv.IterOptions{
					Reverse: true,
					End:     []byte("k3"),
				},
				Expected: []action.KeyValue{},
			},
		},
	}

	test.Execute(t)
}

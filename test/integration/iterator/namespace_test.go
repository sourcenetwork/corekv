package iterator

import (
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
	"github.com/sourcenetwork/corekv/test/integration"
)

func TestIteratorNamespace_DoesNotReturnSelf_NamespaceMatch(t *testing.T) {
	test := &integration.Test{
		Actions: []action.Action{
			action.Set([]byte("namespace"), []byte("namespace exact match")),
			action.Namespace([]byte("namespace")),
			action.Set([]byte("k1"), []byte("v1")),
			&action.Iterate{
				Expected: []action.KeyValue{
					{Key: []byte(""), Value: []byte("namespace exact match")},
					{Key: []byte("k1"), Value: []byte("v1")},
				},
			},
		},
	}

	test.Execute(t)
}

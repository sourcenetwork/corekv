package action

import (
	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/namespace"
	"github.com/sourcenetwork/corekv/test/state"
)

// NamespaceStore action will namespace the current store (replacing it) with the
// given namespace when executed.
//
// Multiple NamespaceStore actions maybe be executed during a test.
type NamespaceStore struct {
	Namespace []byte
}

var _ Action = (*NamespaceStore)(nil)

// Namespace returns a new [NamespaceStore] action that will namespace the current store
// with the given namespace when executed.
func Namespace(namespace []byte) *NamespaceStore {
	return &NamespaceStore{
		Namespace: namespace,
	}
}

func (a *NamespaceStore) Execute(s *state.State) {
	switch store := s.Store.(type) {
	case corekv.Txn:
		s.Store = namespace.WrapTxn(store, a.Namespace)

	case corekv.TxnReaderWriter:
		s.Store = namespace.WrapTS(store, a.Namespace)

	default:
		s.Store = namespace.Wrap(s.Store, a.Namespace)

	}
}

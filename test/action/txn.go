package action

import (
	"github.com/stretchr/testify/require"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/state"
)

// TxnAction wraps an action within the context of a transaction.
//
// Executing this TxnAction will execute the given action within the scope
// of the given transaction.
type TxnAction[T Action] struct {
	TxnID  int
	Action T
}

var _ Action = (*TxnAction[Action])(nil)

// WithTxn wraps the given action within the scope of the default (ID: 0) transaction.
//
// If a transaction with the default ID (0) has not been created by the time this action
// executes, executing the transaction will panic.
func WithTxn[T Action](action T) *TxnAction[T] {
	return &TxnAction[T]{
		Action: action,
	}
}

// WithTxn wraps the given action within the scope of given transaction.
//
// If a transaction with the given ID has not been created by the time this action
// executes, executing the transaction will panic.
func WithTxnI[T Action](action T, txnID int) *TxnAction[T] {
	return &TxnAction[T]{
		TxnID:  txnID,
		Action: action,
	}
}

func (a *TxnAction[T]) Execute(s *state.State) {
	originalStore := s.Store
	// Replace the active store with the transaction, allowing the inner action
	// to act on the transaction without needing to be aware of it.
	s.Store = s.Txns[a.TxnID]
	defer func() {
		// Make sure the original store is restored after executing otherwise
		// subsequent actions will erroneously act on the transaction.
		s.Store = originalStore
	}()

	storeBeforeExecution := s.Store
	a.Action.Execute(s)

	if storeBeforeExecution != s.Store {
		if txn, ok := s.Store.(corekv.Txn); ok {
			// If the inner action replaced the store during execution, and if
			// the replacement is a `Txn`, replace the txn with the replacement store.
			//
			// This allows for actions such as `Namespace` to wrap existing transactions
			// without explicitly handling such a case.
			s.Txns[a.TxnID] = txn
		}
	}
}

// CreateNewTxn creates a new transaction when executed.
//
// It assumes that the active store supports this, and will panic during execution if
// it doesn't.
type CreateNewTxn struct {
	ID       int
	ReadOnly bool
}

var _ Action = (*CreateNewTxn)(nil)

// NewTxn creates a new [CreateNewTxn] with the default (0) transaction ID.
//
// It will create a new transaction that may be used to scope later actions when executed.
func NewTxn() *CreateNewTxn {
	return &CreateNewTxn{}
}

// NewTxnI creates a new [CreateNewTxn] with the given transaction ID.
//
// It will create a new transaction that may be used to scope later actions when executed.
func NewTxnI(id int) *CreateNewTxn {
	return &CreateNewTxn{
		ID: id,
	}
}

func (a *CreateNewTxn) Execute(s *state.State) {
	txn := s.Store.(corekv.TxnStore).NewTxn(a.ReadOnly)

	if a.ID >= len(s.Txns) {
		// Expand the slice if needed.
		s.Txns = append(s.Txns, make([]corekv.Txn, a.ID+1)...)
	}

	s.Txns[a.ID] = txn
}

// DiscardTxn discards the given transaction when executed.
type DiscardTxn struct {
	ID int
}

var _ Action = (*DiscardTxn)(nil)

// Discard returns a new [DiscardTxn] that discards the default (ID: 0) transaction
// when executed.
func Discard() *DiscardTxn {
	return &DiscardTxn{}
}

// DiscardI returns a new [DiscardTxn] that discards the given transaction
// when executed.
func DiscardI(id int) *DiscardTxn {
	return &DiscardTxn{
		ID: id,
	}
}

func (a *DiscardTxn) Execute(s *state.State) {
	txn := s.Txns[a.ID]

	txn.Discard()
}

// CommitTxn commits the given transaction when executed.
type CommitTxn struct {
	ID int
}

var _ Action = (*CommitTxn)(nil)

// Commit returns a new [CommitTxn] that commits the default (ID: 0) transaction
// when executed.
func Commit() *CommitTxn {
	return &CommitTxn{}
}

func (a *CommitTxn) Execute(s *state.State) {
	txn := s.Txns[a.ID]

	err := txn.Commit()
	require.NoError(s.T, err)
}

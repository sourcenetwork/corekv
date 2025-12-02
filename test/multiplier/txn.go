package multiplier

import (
	"github.com/sourcenetwork/corekv/test/action"
)

func init() {
	Register(&txnCommit{})
	Register(&txnDiscard{})
	Register(&txnMulti{})
	Register(&txnContext{})
}

const TxnCommit Name = "txn-commit"

// txnCommit represents the transaction-commit complexity multiplier.
//
// Applying the multiplier will amend tests to assert that activities performed
// on a transaction are both visible to other actions in the commit, and that they are commited
// to the underlying store upon transaction commit.
//
// It achieves this by:
//   - Inserting a new [CreateNewTxn] action immediately after the last action that creates
//     a new store (e.g. [NewStore], [NewBadgerStore]).
//   - Inserting a [CommitTxn] immediately before the first action that closes the store
//     (e.g. [CancelCtx], [CloseStore]).
//   - Copying all original read actions after the last write action and moving the copies
//     to immediately after the inserted [CommitTxn] action.
//   - Modifying all original read and write actions (e.g. [GetValue], [SetValue]) to apply to
//     the created transaction instead of the underlying store.
type txnCommit struct{}

var _ Multiplier = (*txnCommit)(nil)

func (n *txnCommit) Name() Name {
	return TxnCommit
}

func (n *txnCommit) Apply(source action.Actions) action.Actions {
	lastCreateStoreIndex := 0
	firstCloseIndex := 0
	newActions := make(action.Actions, 0, len(source))
	postCommitActions := action.Actions{}

	for i, a := range source {
		switch typedA := a.(type) {
		case *action.NewStore, *action.NewBadgerStore, *action.NewMemoryStore, *action.NewLevelStore:
			lastCreateStoreIndex = i

		case *action.CancelCtx, *action.CloseStore:
			if firstCloseIndex == 0 {
				firstCloseIndex = i
			}

		case *action.Iterator:
		childLoop:
			for _, childAction := range typedA.ChildActions {
				switch childAction.(type) {
				case *action.IteratorCloseRoot:
					firstCloseIndex = i
					break childLoop
				}
			}

		case *action.CreateNewTxn:
			// If the action set already contains txns we should not adjust it
			return source
		}
	}

	newLastCreateStoreIndex := lastCreateStoreIndex
	newFirstCloseIndex := firstCloseIndex

	for i, a := range source {
		switch a.(type) {
		case *action.NewStore, *action.NewBadgerStore, *action.NewMemoryStore, *action.NewLevelStore:
			newActions = append(newActions, a)
			continue

		case *action.CancelCtx, *action.CloseStore:
			newActions = append(newActions, a)
			continue

		case *action.GetValue, *action.HasValue, *action.Iterate, *action.Iterator:
			if firstCloseIndex == 0 {
				postCommitActions = append(postCommitActions, a)
			}

		case *action.DeleteValue, *action.SetValue:
			postCommitActions = action.Actions{}
		}

		if i > lastCreateStoreIndex && i < firstCloseIndex {
			newActions = append(newActions, action.WithTxn(a))
		} else {
			newActions = append(newActions, a)
		}
	}

	// Add two to the length in order to accomodate the new txn and discard actions
	result := make(action.Actions, len(newActions)+2+len(postCommitActions))

	indexOffset := 0
	for i, a := range newActions {
		newIndex := func() int {
			return i + indexOffset
		}

		if i == newLastCreateStoreIndex {
			result[newIndex()] = a

			indexOffset += 1
			result[newIndex()] = action.NewTxn()
		} else if i == newFirstCloseIndex {
			result[newIndex()] = action.Commit()
			indexOffset += 1

			for _, postCommitAction := range postCommitActions {
				result[newIndex()] = postCommitAction
				indexOffset += 1
			}

			result[newIndex()] = a
		} else {
			result[newIndex()] = a
		}
	}

	return result
}

const TxnDiscard Name = "txn-discard"

// txnDiscard represents the transaction-discard complexity multiplier.
//
// Applying the multiplier will amend tests to assert that activities performed
// on a transaction are discarded on discard (or at least not applied without commit).
//
// It achieves this by:
//   - Inserting a new [CreateNewTxn] action immediately after the last action that creates
//     a new store (e.g. [NewStore], [NewBadgerStore]).
//   - Inserting a [DiscardTxn] immediately before the first action that closes the store
//     (e.g. [CancelCtx], [CloseStore]).
//   - Modifying all original read and write actions (e.g. [GetValue], [SetValue]) to apply to
//     the created transaction instead of the underlying store.
//   - Inserting an [Iterate] action immediately after the added [DiscardTxn] action asserting
//     that the underlying store is still empty.
type txnDiscard struct{}

var _ Multiplier = (*txnDiscard)(nil)

func (n *txnDiscard) Name() Name {
	return TxnDiscard
}

func (n *txnDiscard) Apply(source action.Actions) action.Actions {
	lastCreateStoreIndex := 0
	firstCloseIndex := 0
	indexOffset := 0

	result := make(action.Actions, len(source)+3)

	for i, a := range source {
		switch typedA := a.(type) {
		case *action.NewStore, *action.NewBadgerStore, *action.NewMemoryStore, *action.NewLevelStore:
			lastCreateStoreIndex = i

		case *action.CancelCtx, *action.CloseStore:
			if firstCloseIndex == 0 {
				firstCloseIndex = i
			}

		case *action.Iterator:
		childLoop:
			for _, childAction := range typedA.ChildActions {
				switch childAction.(type) {
				case *action.IteratorCloseRoot:
					firstCloseIndex = i
					break childLoop
				}
			}

		case *action.CreateNewTxn:
			// If the action set already contains txns we should not adjust it
			return source
		}
	}

	for i, a := range source {
		newIndex := i + indexOffset

		switch a.(type) {
		case *action.NewStore, *action.NewBadgerStore, *action.NewMemoryStore, *action.NewLevelStore:
			result[newIndex] = a

			if i == lastCreateStoreIndex {
				result[newIndex+1] = action.NewTxn()
				indexOffset += 1
			}

		default:
			if i == firstCloseIndex {
				firstCloseIndex = i

				result[newIndex] = action.Discard()
				result[newIndex+1] = &action.Iterate{
					Expected: []action.KeyValue{},
				}

				indexOffset += 2
				newIndex = i + indexOffset
			}

			if i > lastCreateStoreIndex && i < firstCloseIndex {
				result[newIndex] = action.WithTxn(a)
			} else {
				result[newIndex] = a
			}
		}
	}

	return result
}

const TxnMulti Name = "txn-multi"

// txnMulti represents the multiple-transaction complexity multiplier.
//
// Applying the multiplier will amend tests to assert that activities performed
// on a transaction are isolated from other transactions.
//
// It achieves this by:
//   - Inserting two [CreateNewTxn] actions immediately after the last action that creates
//     a new store (e.g. [NewStore], [NewBadgerStore]).
//   - Inserting two [DiscardTxn] actions immediately before the first action that closes the store
//     (e.g. [CancelCtx], [CloseStore]).
//   - Modifying all original read and write actions (e.g. [GetValue], [SetValue]) to apply to
//     the first created transaction only instead of the underlying store.
//   - Inserting an [Iterate] action immediately before the added [DiscardTxn] actions, scoping it to
//     the second transaction, and asserting that the second transaction has remained empty.
type txnMulti struct{}

var _ Multiplier = (*txnMulti)(nil)

func (n *txnMulti) Name() Name {
	return TxnMulti
}

func (n *txnMulti) Apply(source action.Actions) action.Actions {
	const primaryTxnID int = 0
	const secondaryTxnID int = 1

	lastCreateStoreIndex := 0
	firstCloseIndex := 0
	indexOffset := 0

	result := make(action.Actions, len(source)+4)

	for i, a := range source {
		switch typedA := a.(type) {
		case *action.NewStore, *action.NewBadgerStore, *action.NewMemoryStore, *action.NewLevelStore:
			lastCreateStoreIndex = i

		case *action.CancelCtx, *action.CloseStore:
			if firstCloseIndex == 0 {
				firstCloseIndex = i
			}

		case *action.Iterator:
		childLoop:
			for _, childAction := range typedA.ChildActions {
				switch childAction.(type) {
				case *action.IteratorCloseRoot:
					firstCloseIndex = i
					break childLoop
				}
			}

		case *action.CreateNewTxn:
			// If the action set already contains txns we should not adjust it
			return source
		}
	}

	for i, a := range source {
		newIndex := i + indexOffset

		switch a.(type) {
		case *action.NewStore, *action.NewBadgerStore, *action.NewMemoryStore, *action.NewLevelStore:
			result[newIndex] = a

			if i == lastCreateStoreIndex {
				result[newIndex+1] = action.NewTxnI(primaryTxnID)
				result[newIndex+2] = action.NewTxnI(secondaryTxnID)
				indexOffset += 2
			}

		default:
			if i == firstCloseIndex {
				result[newIndex] = action.WithTxnI(
					&action.Iterate{
						Expected: []action.KeyValue{},
					},
					secondaryTxnID,
				)
				result[newIndex+1] = action.DiscardI(primaryTxnID)
				result[newIndex+2] = action.DiscardI(secondaryTxnID)

				indexOffset += 2
				newIndex = i + indexOffset
			}

			if i > lastCreateStoreIndex && i < firstCloseIndex {
				result[newIndex] = action.WithTxnI(a, primaryTxnID)
			} else {
				result[newIndex] = a
			}
		}
	}

	return result
}

const TxnContext Name = "txn-context"

// txnContext represents the txn-context complexity multiplier.
//
// Applying the multiplier will replace all [action.TxnAction] actions
// with [action.TxnCtxAction] instances.
type txnContext struct{}

var _ Multiplier = (*txnContext)(nil)

func (n *txnContext) Name() Name {
	return TxnContext
}

func (n *txnContext) Apply(source action.Actions) action.Actions {
	result := make([]action.Action, len(source))

	for i, a := range source {
		switch t := a.(type) {
		case *action.TxnAction[action.Action]:
			result[i] = action.WithTxnCtx(t.Action, t.TxnID)
		default:
			result[i] = a
		}
	}

	return result
}

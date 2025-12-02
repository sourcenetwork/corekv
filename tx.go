package corekv

import "context"

// TxnStore represents a [Store] that supports transactions.
type TxnStore interface {
	Store

	// NewTxn returns a new transaction.
	NewTxn(readonly bool) Txn
}

// TxnReaderWriter contains the functions for reading and writing values within the store
// and supports transactions.
type TxnReaderWriter interface {
	ReaderWriter

	// NewTxn returns a new transaction.
	NewTxn(readonly bool) Txn
}

// Txn isolates changes made to the underlying store from this object,
// and isolates changes made via this object from the underlying store
// until `Commit` is called.
type Txn interface {
	ReaderWriter

	// Commit applies all changes made via this [Txn] to the underlying
	// [Store].
	Commit() error

	// Discard discards all changes made via this object so far, returning
	// it to the state it was at at time of construction.
	Discard()
}

type ctxTxnKey struct{}

// CtxTxnKey is the unique transaction context key that can be used
// to get or set the current transaction on the context.
var CtxTxnKey = &ctxTxnKey{}

// MustGetCtxTxn returns the transaction from the context or panics.
func MustGetCtxTxn(ctx context.Context) Txn {
	return ctx.Value(CtxTxnKey).(Txn) //nolint:forcetypeassert
}

// TryGetCtxTxn returns a transaction and a bool indicating if the
// txn was retrieved from the given context.
func TryGetCtxTxn(ctx context.Context) (Txn, bool) {
	txn, ok := ctx.Value(CtxTxnKey).(Txn)
	return txn, ok
}

// CtxSetTxn returns a new context with the txn value set.
//
// This will overwrite any previously set transaction value.
func SetCtxTxn(ctx context.Context, txn Txn) context.Context {
	return context.WithValue(ctx, CtxTxnKey, txn)
}

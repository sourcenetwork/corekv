package corekv

import (
	"context"
	"sync/atomic"
)

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
	TxnCore
}

// TxnCore implements the core transaction methods. This is used by
// stores that wrap other stores such as chunk and namespace.
type TxnCore interface {
	TxnState

	// Commit applies all changes made via this [Txn] to the underlying
	// [Store].
	Commit() error

	// Discard discards all changes made via this object so far, returning
	// it to the state it was at at time of construction.
	Discard()
}

// TxnState contains functions for managing transaction callbacks
// and reading the unique transaction id.
type TxnState interface {
	// ID returns the unique immutable identifier for this transaction.
	ID() uint64

	// OnSuccess registers a function to be called when the transaction is committed.
	OnSuccess(fn func())

	// OnError registers a function to be called when the transaction is rolled back.
	OnError(fn func())

	// OnDiscard registers a function to be called when the transaction is discarded.
	OnDiscard(fn func())

	// OnSuccessAsync registers a function to be called asynchronously when the transaction is committed.
	OnSuccessAsync(fn func())

	// OnErrorAsync registers a function to be called asynchronously when the transaction is rolled back.
	OnErrorAsync(fn func())

	// OnDiscardAsync registers a function to be called asynchronously when the transaction is discarded.
	OnDiscardAsync(fn func())
}

var txnCounter atomic.Uint64

type txnEmbed struct {
	id         uint64
	errorFns   []func()
	discardFns []func()
	successFns []func()
}

// NewTxnState returns a struct that is meant to be embedded in
// transaction implementations and functions to dispatch callbacks.
//
// This is meant to be used by store implementors and not corekv consumers.
func NewTxnState() (TxnState, func(), func(), func()) {
	state := &txnEmbed{
		id: txnCounter.Add(1),
	}
	onSuccess := func() {
		for _, fn := range state.successFns {
			fn()
		}
	}
	onError := func() {
		for _, fn := range state.errorFns {
			fn()
		}
	}
	onDiscard := func() {
		for _, fn := range state.discardFns {
			fn()
		}
	}
	return state, onSuccess, onError, onDiscard
}

func (t *txnEmbed) ID() uint64 {
	return t.id
}

func (t *txnEmbed) OnSuccess(fn func()) {
	t.successFns = append(t.successFns, fn)
}

func (t *txnEmbed) OnSuccessAsync(fn func()) {
	t.successFns = append(t.successFns, func() { go fn() })
}

func (t *txnEmbed) OnError(fn func()) {
	t.errorFns = append(t.errorFns, fn)
}

func (t *txnEmbed) OnErrorAsync(fn func()) {
	t.errorFns = append(t.errorFns, func() { go fn() })
}

func (t *txnEmbed) OnDiscard(fn func()) {
	t.discardFns = append(t.discardFns, fn)
}

func (t *txnEmbed) OnDiscardAsync(fn func()) {
	t.discardFns = append(t.discardFns, func() { go fn() })
}

type ctxTxnKey struct{}

// CtxTxnKey is the unique transaction context key that can be used
// to get or set the current transaction on the context.
var CtxTxnKey = &ctxTxnKey{}

// MustGetCtxTxn returns the transaction from the context or panics.
func MustGetCtxTxn(ctx context.Context) Txn {
	return MustGetCtxTxnG[Txn](ctx)
}

// MustGetCtxTxn returns the transaction from the context or panics.
func MustGetCtxTxnG[T Txn](ctx context.Context) T {
	return ctx.Value(CtxTxnKey).(T) //nolint:forcetypeassert
}

// TryGetCtxTxn returns a transaction and a bool indicating if the
// txn was retrieved from the given context.
func TryGetCtxTxn(ctx context.Context) (Txn, bool) {
	return TryGetCtxTxnG[Txn](ctx)
}

// TryGetCtxTxnG returns a transaction and a bool indicating if the
// txn was retrieved from the given context.
func TryGetCtxTxnG[T Txn](ctx context.Context) (T, bool) {
	txn, ok := ctx.Value(CtxTxnKey).(T)
	return txn, ok
}

// CtxSetTxn returns a new context with the txn value set.
//
// This will overwrite any previously set transaction value.
func SetCtxTxn(ctx context.Context, txn Txn) context.Context {
	return context.WithValue(ctx, CtxTxnKey, txn)
}

package corekv

import "errors"

var (
	ErrNotFound     = errors.New("key not found")
	ErrEmptyKey     = errors.New("empty key")
	ErrValueNil     = errors.New("value is nil")
	ErrDiscardedTxn = errors.New("transaction discarded")
	ErrDBClosed     = errors.New("datastore closed")
	ErrTxnConflict  = errors.New("transaction conflict. Please retry")
	ErrReadOnlyTxn  = errors.New("read only transaction")
)

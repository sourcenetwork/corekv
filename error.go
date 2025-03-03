package corekv

import "errors"

var (
	ErrNotFound     = errors.New("kv: key not found")
	ErrEmptyKey     = errors.New("kv: empty key")
	ErrValueNil     = errors.New("kv: value is nil")
	ErrDiscardedTxn = errors.New("kv: transaction discarded")
	ErrDBClosed     = errors.New("kv: datastore closed")
	ErrTxnConflict  = errors.New("kv: transaction Conflict. Please retry")
	ErrReadOnlyTxn  = errors.New("kv: read only transaction")
)

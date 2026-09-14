package regolith

import (
	"errors"

	"github.com/sourcenetwork/go-regolith"

	"github.com/sourcenetwork/corekv"
)

var regolithErrToKVErrMap = map[error]error{
	regolith.ErrNotFound:  corekv.ErrNotFound,
	regolith.ErrClosed:    corekv.ErrDBClosed,
	regolith.ErrConflict:  corekv.ErrTxnConflict,
	regolith.ErrReadOnly:  corekv.ErrReadOnlyTxn,
	regolith.ErrDiscarded: corekv.ErrDiscardedTxn,
}

// regolithErrToKVErr maps a go-regolith sentinel onto its corekv equivalent.
//
// The errors that have no corekv equivalent - `ErrInvalidArgument`, `ErrPanic`
// and `ErrUnexpected` - are returned as they are, carrying the detail string
// go-regolith wrapped them with.
func regolithErrToKVErr(err error) error {
	if err == nil {
		return nil
	}
	mappedErr, ok := regolithErrToKVErrMap[err]
	if ok {
		return mappedErr
	}
	for k, v := range regolithErrToKVErrMap {
		if errors.Is(err, k) {
			return v
		}
	}
	return err
}

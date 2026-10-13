package action

import (
	"fmt"

	"github.com/sourcenetwork/corekv"
	"github.com/sourcenetwork/corekv/test/state"
)

// maxFillTxnSets stops [FillTxn] from running forever on a store whose transactions have no size
// limit.
const maxFillTxnSets = 1_000_000

// FillTxnAction writes values into one new transaction until a write fails, and checks that error.
//
// It opens its own transaction on the root store, so it behaves the same whichever multipliers
// are active.
type FillTxnAction struct {
	ValueSize     int
	ExpectedError string
}

var _ Action = (*FillTxnAction)(nil)

// FillTxn returns a new [FillTxnAction] that writes values of the given size into one transaction
// until a write fails, and requires that the error contains the given string.
func FillTxn(valueSize int, expectedErr string) *FillTxnAction {
	return &FillTxnAction{
		ValueSize:     valueSize,
		ExpectedError: expectedErr,
	}
}

func (a *FillTxnAction) Execute(s *state.State) {
	txn := s.Rootstore.(corekv.TxnStore).NewTxn(false)
	defer txn.Discard()

	value := make([]byte, a.ValueSize)
	for i := 0; i < maxFillTxnSets; i++ {
		err := txn.Set(s.Ctx, []byte(fmt.Sprintf("key%d", i)), value)
		if err != nil {
			expectError(s, err, a.ExpectedError)
			return
		}
	}
	s.T.Fatalf("transaction accepted %d writes without an error", maxFillTxnSets)
}

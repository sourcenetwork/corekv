//go:build !regolith

package action

import (
	"github.com/sourcenetwork/corekv/test/state"
)

func (a *NewRegolithStore) Execute(s *state.State) {
	s.T.Skip("regolith store tests require the `regolith` build tag, run `make test:regolith`")
}

package multiplier

import "github.com/sourcenetwork/corekv/test/action"

func init() {
	Register(&namespace{})
}

const Namespace Name = "namespace"

// namespace represents the namespacing complexity multiplier.
//
// Applying the multiplier will prepend a [action.NamespaceStore] action
// to the front of the action set.
type namespace struct{}

var _ Multiplier = (*namespace)(nil)

func (n *namespace) Name() Name {
	return Namespace
}

func (n *namespace) Apply(source action.Actions) action.Actions {
	result := make(action.Actions, 1, len(source)+1)
	result[0] = action.Namespace([]byte("/example"))
	result = append(result, source...)

	return result
}

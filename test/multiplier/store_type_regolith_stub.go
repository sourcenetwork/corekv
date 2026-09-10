//go:build !regolith

package multiplier

// registerRegolith does nothing without the `regolith` build tag, see
// `store_type_regolith.go`.
func registerRegolith() {}

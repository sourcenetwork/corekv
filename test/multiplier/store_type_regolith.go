//go:build regolith

package multiplier

// registerRegolith registers the regolith store complexity multiplier.
//
// The regolith store links a Rust staticlib via cgo, so it is only registered
// when built with the `regolith` build tag.
func registerRegolith() {
	Register(&regolith{})
}

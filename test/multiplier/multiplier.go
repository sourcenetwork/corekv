package multiplier

import (
	"os"
	"strings"
	"testing"

	"github.com/sourcenetwork/corekv/test/action"
)

var activeMultipliers []Multiplier
var availableMultipliers []Multiplier

// Name represents the unique name of a multiplier.
//
// The alias provides a more descriptive type to use besides `string`.
type Name = string

// Multiplier represents a complexity multiplier of the system under test.
//
// It represents a concept that multiplies the surface area and complexity
// of other proximal features, for example, database transactions are
// complexity modifiers, as when adding many new database actions such as
// a new filter operation, the new action must be tested both with, and without
// a transaction - the transaction concept multiplies the complexity of the system.
//
// The identification and automatic representation of complexity multipiers are critical
// to the long term maintainability and scalability of a codebase. Without them, introducing
// fairly simple features becomes high risk, and requires a forever growing number of manually
// constructed tests, when the feature developer needs to remember, and write tests for every
// multiplier that affects the feature under test.  This is error prone, very tedious, and
// distracts from the feature itself - often degrading the quality of both test and production code.
//
// Complexity multipliers defined using this type may redefine the set of test actions
// to perform within a test, allowing a single test to scale with the complexity
// multiplier.  For example, applying a 'namespace' multplier may add an action that
// namespaces the store under test, reducing, but not removing, testing cost of the
// namespace multiplier.
type Multiplier interface {
	// Name returns the unique name of the multiplier.
	Name() Name

	// Apply applies the complexity multiplier to the given action set, returning a new
	// action set testing the multiplier.
	Apply(source action.Actions) action.Actions
}

// Register adds the given multiplier to the internal set of available multipliers.
//
// Multipliers must be registered before calling [Init] in order to be applied.
func Register(multiplier Multiplier) {
	availableMultipliers = append(availableMultipliers, multiplier)
}

// Init initializes the multiplier engine, determining which multipliers are to
// be applied to action sets.
//
// It must be called after all required [Multiplier]s are [Register]ed.
//
// It will check to see if the given environment variable is set, and if so,
// sources the comma sperated set of multiplier names from it.  If the
// environment variable is not set, it will use the provided defaults.
//
// Multipliers will be applied in the order in which they are given.
func Init(envVarName string, defaults ...Name) {
	var multiplierNames []string
	multipliersString, ok := os.LookupEnv(envVarName)
	if ok {
		multiplierNames = strings.Split(multipliersString, ",")
	} else {
		multiplierNames = defaults
	}

	for i, name := range multiplierNames {
		multiplierNames[i] = strings.TrimSpace(name)
	}

	availableMultipliersByName := make(map[Name]Multiplier, len(availableMultipliers))
	for _, multiplier := range availableMultipliers {
		availableMultipliersByName[multiplier.Name()] = multiplier
	}

	activeMultipliers = make([]Multiplier, 0, len(multiplierNames))
	for _, multiplierName := range multiplierNames {
		if multiplier, ok := availableMultipliersByName[multiplierName]; ok {
			activeMultipliers = append(activeMultipliers, multiplier)
		}
	}
}

// Apply applies all active multipliers to the given action set, in the order in which the
// multipliers are provided.
func Apply(actions action.Actions) action.Actions {
	for _, multiplier := range activeMultipliers {
		actions = multiplier.Apply(actions)
	}

	return actions
}

// Skip skips the test if:
//   - The active set of multipliers does not contain a multiplier with a name exactly matching
//     a value in the given `includes` set.
//   - The active set of multipliers contains a multiplier with a name exactly matching
//     a value in the given `excludes` set.
func Skip(t testing.TB, includes []Name, excludes []Name) {
	for _, multiplier := range activeMultipliers {
		for _, exclude := range excludes {
			if multiplier.Name() == exclude {
				t.Skipf("skipping, multiplier is excluded. Name: %s", exclude)
			}
		}
	}

	for _, include := range includes {
		included := false
		for _, multiplier := range activeMultipliers {
			if multiplier.Name() == include {
				included = true
				break
			}
		}

		if !included {
			t.Skipf("skipping, required multiplier is not included. Name: %s", include)
		}
	}
}

// Get returns a comma-seperated string containing the current active multiplier names.
func Get() string {
	multipliers := make([]string, len(activeMultipliers))
	for i, multiplier := range activeMultipliers {
		multipliers[i] = multiplier.Name()
	}

	return strings.Join(multipliers, ",")
}

package permissions

import (
	"strings"

	"github.com/hapyco/dygo/internal/shape"
)

type actionSpec struct {
	Action Action
	Column string
}

var actionSpecs = []actionSpec{
	{Action: ActionRead, Column: `"read"`},
	{Action: ActionCreate, Column: `"create"`},
	{Action: ActionUpdate, Column: `"update"`},
	{Action: ActionDelete, Column: `"delete"`},
	{Action: ActionExport, Column: `"export"`},
	{Action: ActionPrint, Column: `"print"`},
}

// SupportedActions returns the stable permission actions supported by dygo.
func SupportedActions() []Action {
	actions := make([]Action, len(actionSpecs))
	for index, spec := range actionSpecs {
		actions[index] = spec.Action
	}
	return actions
}

// ParseAction normalizes and validates a permission action name.
func ParseAction(value string) (Action, error) {
	action := Action(strings.TrimSpace(value))
	if err := shape.ValidateMetadataName("permission action", string(action)); err != nil {
		return "", err
	}
	return action, nil
}

func actionColumn(action Action) (string, bool) {
	for _, spec := range actionSpecs {
		if spec.Action == action {
			return spec.Column, true
		}
	}
	return "", false
}

// IsBuiltInAction reports whether action is stored in a dedicated Permission column.
func IsBuiltInAction(action Action) bool {
	_, ok := actionColumn(action)
	return ok
}

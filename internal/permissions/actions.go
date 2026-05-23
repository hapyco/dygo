package permissions

import (
	"fmt"

	"github.com/dygo-dev/dygo/internal/db"
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

func actionColumn(action Action) (string, bool) {
	for _, spec := range actionSpecs {
		if spec.Action == action {
			return spec.Column, true
		}
	}
	return "", false
}

// ValidateMetadata verifies that core.permission metadata supports the runtime action catalog.
func ValidateMetadata(meta db.MetadataEntityMeta) error {
	fields := db.MetadataFieldsByName(meta)
	for _, spec := range actionSpecs {
		field, ok := db.RecordAddressableFieldByName(fields, string(spec.Action))
		if !ok {
			return fmt.Errorf("permission action field %q is missing", spec.Action)
		}
		if field.Type != "boolean" {
			return fmt.Errorf("permission action field %q must be boolean", spec.Action)
		}
	}
	return nil
}

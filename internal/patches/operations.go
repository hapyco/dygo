package patches

import "sort"

const (
	OperationRenameField     = "rename-field"
	OperationRenameEntity    = "rename-entity"
	OperationCopyField       = "copy-field"
	OperationBackfillField   = "backfill-field"
	OperationDropField       = "drop-field"
	OperationChangeFieldType = "change-field-type"
	OperationSQL             = "sql"
)

// OperationSpec describes the authored shape of one patch operation type.
type OperationSpec struct {
	Type     string
	Required []string
	Optional []string
}

var operationSpecs = map[string]OperationSpec{
	OperationRenameField:     {Type: OperationRenameField, Required: []string{"type", "entity", "from", "to"}},
	OperationRenameEntity:    {Type: OperationRenameEntity, Required: []string{"type", "from", "to"}},
	OperationCopyField:       {Type: OperationCopyField, Required: []string{"type", "entity", "from", "to"}, Optional: []string{"when"}},
	OperationBackfillField:   {Type: OperationBackfillField, Required: []string{"type", "entity", "field", "value"}, Optional: []string{"when"}},
	OperationDropField:       {Type: OperationDropField, Required: []string{"type", "entity", "field"}},
	OperationChangeFieldType: {Type: OperationChangeFieldType, Required: []string{"type", "entity", "field", "to", "using"}},
	OperationSQL:             {Type: OperationSQL, Required: []string{"type", "name", "reason", "statement"}},
}

// OperationSpecFor returns the authored field contract for one operation type.
func OperationSpecFor(operationType string) (OperationSpec, bool) {
	spec, ok := operationSpecs[operationType]
	if !ok {
		return OperationSpec{}, false
	}
	return OperationSpec{
		Type:     spec.Type,
		Required: append([]string(nil), spec.Required...),
		Optional: append([]string(nil), spec.Optional...),
	}, true
}

// SupportedOperationTypes returns known operation types in stable order.
func SupportedOperationTypes() []string {
	types := make([]string, 0, len(operationSpecs))
	for operationType := range operationSpecs {
		types = append(types, operationType)
	}
	sort.Strings(types)
	return types
}

// AllowedFields returns the full set of allowed YAML keys for this operation.
func (s OperationSpec) AllowedFields() []string {
	fields := make([]string, 0, len(s.Required)+len(s.Optional))
	fields = append(fields, s.Required...)
	fields = append(fields, s.Optional...)
	sort.Strings(fields)
	return fields
}

// SupportedPhases returns known patch phases in stable order.
func SupportedPhases() []string {
	return []string{PhasePreSync, PhasePostSync}
}

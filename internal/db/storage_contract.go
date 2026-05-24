package db

import (
	"fmt"
	"strings"

	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/dygo-dev/dygo/internal/entity/schema"
)

const (
	systemFieldID        = "id"
	systemFieldName      = "name"
	systemFieldCreatedAt = "created-at"
	systemFieldUpdatedAt = "updated-at"

	systemColumnID             = "id"
	systemColumnName           = "name"
	systemColumnCreatedAt      = "created_at"
	systemColumnUpdatedAt      = "updated_at"
	systemColumnParentEntityID = "parent_entity_id"
	systemColumnParentRecordID = "parent_record_id"
	systemColumnParentFieldID  = "parent_field_id"
	systemColumnPosition       = "position"
)

type systemRecordField struct {
	Name          string
	Label         string
	Type          string
	Column        string
	Listable      bool
	Nameable      bool
	ValueKind     string
	StudioEditor  string
	StudioDisplay string
}

var systemRecordFields = []systemRecordField{
	{Name: systemFieldID, Label: "ID", Type: "bigint", Column: systemColumnID, Listable: true, ValueKind: fieldtype.ValueInteger, StudioEditor: "number", StudioDisplay: "number"},
	{Name: systemFieldName, Label: "Name", Type: "text", Column: systemColumnName, Listable: true, Nameable: true, ValueKind: fieldtype.ValueString, StudioEditor: "text", StudioDisplay: "text"},
	{Name: systemFieldCreatedAt, Label: "Created At", Type: "datetime", Column: systemColumnCreatedAt, Listable: true, ValueKind: fieldtype.ValueDatetime, StudioEditor: "datetime", StudioDisplay: "datetime"},
	{Name: systemFieldUpdatedAt, Label: "Updated At", Type: "datetime", Column: systemColumnUpdatedAt, Listable: true, ValueKind: fieldtype.ValueDatetime, StudioEditor: "datetime", StudioDisplay: "datetime"},
}

func systemRecordFieldByName(name string) (systemRecordField, bool) {
	for _, field := range systemRecordFields {
		if field.Name == name {
			return field, true
		}
	}
	return systemRecordField{}, false
}

func systemRecordFieldByColumn(column string) (systemRecordField, bool) {
	for _, field := range systemRecordFields {
		if field.Column == column {
			return field, true
		}
	}
	return systemRecordField{}, false
}

func systemRecordSelectColumns() []string {
	columns := make([]string, len(systemRecordFields))
	for index, field := range systemRecordFields {
		columns[index] = field.Column
	}
	return columns
}

func isSystemRecordField(name string) bool {
	_, ok := systemRecordFieldByName(name)
	return ok
}

func isSystemColumn(column string) bool {
	_, ok := systemRecordFieldByColumn(column)
	return ok
}

func columnForField(field schema.Field) (string, error) {
	definition, ok := fieldtype.DefaultDefinition(field.Type)
	if !ok {
		return "", fmt.Errorf("unsupported field type %q", field.Type)
	}
	if !definition.Behavior.Stored {
		return "", fmt.Errorf("field type %q does not have direct column storage", field.Type)
	}
	return storageName(field.Name) + definition.Behavior.ColumnSuffix, nil
}

func recordColumnForField(name string, fieldType string) string {
	column, _ := storageColumnForField(name, fieldType)
	return column
}

func storageColumnForField(name string, fieldType string) (string, bool) {
	definition, ok := fieldtype.DefaultDefinition(fieldType)
	if !ok {
		return storageName(name), false
	}
	return storageName(name) + definition.Behavior.ColumnSuffix, true
}

func storageName(value string) string {
	return strings.ReplaceAll(value, "-", "_")
}

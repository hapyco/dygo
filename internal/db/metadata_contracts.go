package db

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
)

// MetadataFieldsByName indexes Entity fields by metadata name.
func MetadataFieldsByName(meta MetadataEntityMeta) map[string]MetadataField {
	fields := map[string]MetadataField{}
	for _, field := range meta.Fields {
		fields[field.Name] = field
	}
	return fields
}

// MetadataFieldByName returns an Entity field or a supported system Record field.
func MetadataFieldByName(fields map[string]MetadataField, name string) (MetadataField, bool) {
	if field, ok := fields[name]; ok {
		return field, true
	}
	if name == "name" {
		return MetadataField{Name: "name", Label: "Name", Type: "text", Required: true, Unique: true, Stored: true, Listable: true, NameRenderable: true, ValueKind: fieldtype.ValueString}, true
	}
	return MetadataField{}, false
}

// MetadataFieldStored reports whether field has runtime storage.
func MetadataFieldStored(field MetadataField) bool {
	if field.Stored {
		return true
	}
	definition, ok := fieldtype.DefaultDefinition(field.Type)
	return ok && definition.Behavior.Stored
}

// LinkFieldTarget returns the target Entity key for a link field.
func LinkFieldTarget(field MetadataField) (string, error) {
	var options struct {
		Entity string `json:"entity"`
	}
	if err := json.Unmarshal(field.Options, &options); err != nil {
		return "", fmt.Errorf("link field options are invalid")
	}
	if strings.TrimSpace(options.Entity) == "" {
		return "", fmt.Errorf("link field target entity is required")
	}
	return options.Entity, nil
}

// ValidateRecordMatch verifies that match is backed by a unique metadata contract.
func ValidateRecordMatch(meta MetadataEntityMeta, match []string) error {
	if len(match) == 0 {
		return fmt.Errorf("record match is required")
	}
	fields := MetadataFieldsByName(meta)
	seen := map[string]bool{}
	for _, name := range match {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("record match contains an empty field")
		}
		if seen[name] {
			return fmt.Errorf("record match contains duplicate field %q", name)
		}
		seen[name] = true
		field, ok := MetadataFieldByName(fields, name)
		if !ok {
			return fmt.Errorf("record match field %q does not exist on Entity %q", name, meta.Name)
		}
		if !MetadataFieldStored(field) {
			return fmt.Errorf("record match field %q uses unsupported collection storage", name)
		}
	}
	if len(match) == 1 {
		field, _ := MetadataFieldByName(fields, match[0])
		if field.Unique || match[0] == "name" {
			return nil
		}
	}
	for _, constraint := range meta.Constraints {
		if constraint.Type != "unique" {
			continue
		}
		var uniqueFields []string
		if err := json.Unmarshal(constraint.Fields, &uniqueFields); err != nil {
			return fmt.Errorf("unique constraint %q fields are invalid", constraint.Name)
		}
		if sameStringSet(match, uniqueFields) {
			return nil
		}
	}
	return fmt.Errorf("record match %q is not backed by a unique field or constraint on Entity %q", strings.Join(match, ", "), meta.Name)
}

func sameStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftSorted := append([]string(nil), left...)
	rightSorted := append([]string(nil), right...)
	sort.Strings(leftSorted)
	sort.Strings(rightSorted)
	for index := range leftSorted {
		if leftSorted[index] != rightSorted[index] {
			return false
		}
	}
	return true
}

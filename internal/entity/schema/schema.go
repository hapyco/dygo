// Package schema loads and validates dygo Entity metadata.
package schema

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"gopkg.in/yaml.v3"
)

// Entity describes one dygo business object definition.
type Entity struct {
	Line        int     `yaml:"-"`
	Name        string  `yaml:"name"`
	Label       string  `yaml:"label"`
	Description string  `yaml:"description,omitempty"`
	Fields      []Field `yaml:"fields"`
}

// Field describes one field inside an Entity.
type Field struct {
	Line     int               `yaml:"-"`
	Name     string            `yaml:"name"`
	Label    string            `yaml:"label"`
	Type     string            `yaml:"type"`
	Required bool              `yaml:"required,omitempty"`
	Unique   bool              `yaml:"unique,omitempty"`
	Default  yaml.Node         `yaml:"default,omitempty"`
	Options  fieldtype.Options `yaml:"options,omitempty"`
}

// ValidationError reports one or more Entity validation problems.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "entity schema validation failed: " + strings.Join(e.Problems, "; ")
}

// LoadFile reads, decodes, and validates one Entity metadata file.
func LoadFile(path string, registry fieldtype.Registry) (Entity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Entity{}, fmt.Errorf("read entity schema %s: %w", path, err)
	}
	entity, err := Decode(data, registry)
	if err != nil {
		return Entity{}, fmt.Errorf("load entity schema %s: %w", path, err)
	}
	return entity, nil
}

// Decode decodes and validates one Entity metadata document.
func Decode(data []byte, registry fieldtype.Registry) (Entity, error) {
	source, err := inspectSource(data)
	if err != nil {
		return Entity{}, err
	}

	var entity Entity
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&entity); err != nil {
		return Entity{}, fmt.Errorf("decode entity schema: %w", err)
	}
	source.apply(&entity)
	if err := entity.Validate(registry); err != nil {
		return Entity{}, err
	}
	return entity, nil
}

// Validate validates an Entity against a field type registry.
func (e Entity) Validate(registry fieldtype.Registry) error {
	var problems []string

	if strings.TrimSpace(e.Name) == "" {
		problems = append(problems, withLine(e.Line, "name is required"))
	} else if !fieldtype.IsName(e.Name) {
		problems = append(problems, withLine(e.Line, fmt.Sprintf("name %q must be kebab-case", e.Name)))
	}
	if strings.TrimSpace(e.Label) == "" {
		problems = append(problems, withLine(e.Line, "label is required"))
	}
	if len(e.Fields) == 0 {
		problems = append(problems, withLine(e.Line, "at least one field is required"))
	}

	seenFields := map[string]struct{}{}
	for _, field := range e.Fields {
		validateField(field, registry, seenFields, &problems)
		if field.Name != "" {
			seenFields[field.Name] = struct{}{}
		}
	}

	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func validateField(field Field, registry fieldtype.Registry, seenFields map[string]struct{}, problems *[]string) {
	fieldLabel := field.Name
	if fieldLabel == "" {
		fieldLabel = "<missing>"
	}

	if strings.TrimSpace(field.Name) == "" {
		*problems = append(*problems, withLine(field.Line, "field name is required"))
	} else if !fieldtype.IsName(field.Name) {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q name must be kebab-case", field.Name)))
	}
	if _, ok := seenFields[field.Name]; ok {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("duplicate field %q", field.Name)))
	}
	if strings.TrimSpace(field.Label) == "" {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q label is required", fieldLabel)))
	}
	if strings.TrimSpace(field.Type) == "" {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type is required", fieldLabel)))
		return
	}
	if !fieldtype.IsName(field.Type) {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q must be kebab-case", fieldLabel, field.Type)))
		return
	}

	definition, ok := registry.Get(field.Type)
	if !ok {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q uses unknown type %q", fieldLabel, field.Type)))
		return
	}
	if field.Required && !definition.AllowRequired {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q cannot be required", fieldLabel, field.Type)))
	}
	if field.Unique && !definition.AllowUnique {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q cannot be unique", fieldLabel, field.Type)))
	}
	if field.Default.Kind != 0 && !definition.AllowDefault {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q does not support default values", fieldLabel, field.Type)))
	}
	if err := definition.Validate(field.Options); err != nil {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q options invalid for type %q: %v", fieldLabel, field.Type, err)))
	}
}

type sourceMap struct {
	entityLine int
	fieldLines []int
}

func (m sourceMap) apply(entity *Entity) {
	entity.Line = m.entityLine
	for i := range entity.Fields {
		if i >= len(m.fieldLines) {
			break
		}
		entity.Fields[i].Line = m.fieldLines[i]
	}
}

func inspectSource(data []byte) (sourceMap, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return sourceMap{}, fmt.Errorf("parse entity schema: %w", err)
	}
	if err := rejectDuplicateKeysNode(&root, "$"); err != nil {
		return sourceMap{}, err
	}
	return buildSourceMap(&root), nil
}

func buildSourceMap(root *yaml.Node) sourceMap {
	mapping := documentMapping(root)
	if mapping == nil {
		return sourceMap{}
	}

	source := sourceMap{entityLine: mapping.Line}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		value := mapping.Content[i+1]
		if key.Value != "fields" || value.Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range value.Content {
			source.fieldLines = append(source.fieldLines, item.Line)
		}
	}
	return source
}

func documentMapping(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return documentMapping(node.Content[0])
	}
	if node.Kind == yaml.MappingNode {
		return node
	}
	return nil
}

func rejectDuplicateKeysNode(node *yaml.Node, location string) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			if err := rejectDuplicateKeysNode(child, location); err != nil {
				return err
			}
		}
		return nil
	}
	if node.Kind == yaml.SequenceNode {
		for index, child := range node.Content {
			childLocation := fmt.Sprintf("%s[%d]", location, index)
			if err := rejectDuplicateKeysNode(child, childLocation); err != nil {
				return err
			}
		}
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}

	seen := map[string]struct{}{}
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]
		if _, ok := seen[key.Value]; ok {
			return fmt.Errorf("duplicate key %q at %s line %d", key.Value, location, key.Line)
		}
		seen[key.Value] = struct{}{}

		childLocation := location + "." + key.Value
		if err := rejectDuplicateKeysNode(value, childLocation); err != nil {
			return err
		}
	}

	return nil
}

func withLine(line int, message string) string {
	if line == 0 {
		return message
	}
	return fmt.Sprintf("line %d: %s", line, message)
}

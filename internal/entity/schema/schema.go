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
	Line        int          `yaml:"-"`
	Name        string       `yaml:"name"`
	Label       string       `yaml:"label"`
	Description string       `yaml:"description,omitempty"`
	Fields      []Field      `yaml:"fields"`
	Indexes     []Index      `yaml:"indexes,omitempty"`
	Constraints []Constraint `yaml:"constraints,omitempty"`
}

// Field describes one field inside an Entity.
type Field struct {
	Line     int               `yaml:"-"`
	Name     string            `yaml:"name"`
	Label    string            `yaml:"label"`
	Type     string            `yaml:"type"`
	Required bool              `yaml:"required,omitempty"`
	Unique   bool              `yaml:"unique,omitempty"`
	Index    bool              `yaml:"index,omitempty"`
	Default  yaml.Node         `yaml:"default,omitempty"`
	Options  fieldtype.Options `yaml:"options,omitempty"`
}

// Index describes a non-unique Entity-level database index.
type Index struct {
	Line   int      `yaml:"-"`
	Name   string   `yaml:"name,omitempty"`
	Fields []string `yaml:"fields"`
}

// EffectiveName returns the configured or deterministic metadata name for the index.
func (i Index) EffectiveName(entity Entity) string {
	if strings.TrimSpace(i.Name) != "" {
		return i.Name
	}
	parts := []string{entity.Name}
	parts = append(parts, i.Fields...)
	parts = append(parts, "idx")
	return strings.Join(parts, "-")
}

// Constraint describes an Entity-level database constraint.
type Constraint struct {
	Line     int       `yaml:"-"`
	Name     string    `yaml:"name,omitempty"`
	Type     string    `yaml:"type"`
	Fields   []string  `yaml:"fields,omitempty"`
	Field    string    `yaml:"field,omitempty"`
	Operator string    `yaml:"operator,omitempty"`
	Value    yaml.Node `yaml:"value,omitempty"`
}

// EffectiveName returns the configured or deterministic metadata name for the constraint.
func (c Constraint) EffectiveName(entity Entity) string {
	if strings.TrimSpace(c.Name) != "" {
		return c.Name
	}
	switch c.Type {
	case "unique":
		parts := []string{entity.Name}
		parts = append(parts, c.Fields...)
		parts = append(parts, "key")
		return strings.Join(parts, "-")
	case "check":
		return strings.Join([]string{entity.Name, c.Field, c.Operator, "check"}, "-")
	default:
		return strings.Join([]string{entity.Name, "constraint"}, "-")
	}
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
	fields := map[string]Field{}
	fieldTypes := map[string]fieldtype.Definition{}
	for _, field := range e.Fields {
		validateField(field, registry, seenFields, &problems)
		if field.Name != "" {
			seenFields[field.Name] = struct{}{}
			fields[field.Name] = field
			if definition, ok := registry.Get(field.Type); ok {
				fieldTypes[field.Name] = definition
			}
		}
	}
	validateIndexes(e, fields, fieldTypes, &problems)
	validateConstraints(e, fields, fieldTypes, &problems)

	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func validateIndexes(entity Entity, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, problems *[]string) {
	seenNames := map[string]struct{}{}
	seenDefinitions := map[string]struct{}{}
	for _, index := range entity.Indexes {
		if strings.TrimSpace(index.Name) != "" && !fieldtype.IsName(index.Name) {
			*problems = append(*problems, withLine(index.Line, fmt.Sprintf("index name %q must be kebab-case", index.Name)))
		}
		if len(index.Fields) == 0 {
			*problems = append(*problems, withLine(index.Line, "index fields are required"))
		}
		validateFieldReferences(index.Line, "index", index.Fields, fields, fieldTypes, func(definition fieldtype.Definition) bool {
			return definition.AllowIndex
		}, "indexed", problems)

		name := index.EffectiveName(entity)
		if strings.TrimSpace(name) != "" {
			if _, ok := seenNames[name]; ok {
				*problems = append(*problems, withLine(index.Line, fmt.Sprintf("duplicate index name %q", name)))
			}
			seenNames[name] = struct{}{}
		}
		definitionKey := strings.Join(index.Fields, "\x00")
		if definitionKey != "" {
			if _, ok := seenDefinitions[definitionKey]; ok {
				*problems = append(*problems, withLine(index.Line, fmt.Sprintf("duplicate index fields %q", strings.Join(index.Fields, ", "))))
			}
			seenDefinitions[definitionKey] = struct{}{}
		}
	}
}

func validateConstraints(entity Entity, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, problems *[]string) {
	seenNames := map[string]struct{}{}
	seenDefinitions := map[string]struct{}{}
	for _, constraint := range entity.Constraints {
		if strings.TrimSpace(constraint.Name) != "" && !fieldtype.IsName(constraint.Name) {
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("constraint name %q must be kebab-case", constraint.Name)))
		}
		if strings.TrimSpace(constraint.Type) == "" {
			*problems = append(*problems, withLine(constraint.Line, "constraint type is required"))
			continue
		}
		switch constraint.Type {
		case "unique":
			validateUniqueConstraint(constraint, fields, fieldTypes, seenDefinitions, problems)
		case "check":
			validateCheckConstraint(constraint, fields, fieldTypes, seenDefinitions, problems)
		default:
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("constraint type %q is not supported", constraint.Type)))
		}

		name := constraint.EffectiveName(entity)
		if strings.TrimSpace(name) != "" {
			if _, ok := seenNames[name]; ok {
				*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("duplicate constraint name %q", name)))
			}
			seenNames[name] = struct{}{}
		}
	}
}

func validateUniqueConstraint(constraint Constraint, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, seenDefinitions map[string]struct{}, problems *[]string) {
	if len(constraint.Fields) < 2 {
		*problems = append(*problems, withLine(constraint.Line, "unique constraint requires at least two fields"))
	}
	if strings.TrimSpace(constraint.Field) != "" {
		*problems = append(*problems, withLine(constraint.Line, "unique constraint uses fields, not field"))
	}
	if strings.TrimSpace(constraint.Operator) != "" || constraint.Value.Kind != 0 {
		*problems = append(*problems, withLine(constraint.Line, "unique constraint does not support operator or value"))
	}
	validateFieldReferences(constraint.Line, "unique constraint", constraint.Fields, fields, fieldTypes, func(definition fieldtype.Definition) bool {
		return definition.AllowUnique
	}, "unique", problems)

	key := constraint.Type + "\x00" + strings.Join(constraint.Fields, "\x00")
	if key != constraint.Type+"\x00" {
		if _, ok := seenDefinitions[key]; ok {
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("duplicate unique constraint fields %q", strings.Join(constraint.Fields, ", "))))
		}
		seenDefinitions[key] = struct{}{}
	}
}

func validateCheckConstraint(constraint Constraint, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, seenDefinitions map[string]struct{}, problems *[]string) {
	if len(constraint.Fields) > 0 {
		*problems = append(*problems, withLine(constraint.Line, "check constraint uses field, not fields"))
	}
	if strings.TrimSpace(constraint.Field) == "" {
		*problems = append(*problems, withLine(constraint.Line, "check constraint field is required"))
	} else if !fieldtype.IsName(constraint.Field) {
		*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("check constraint field %q must be kebab-case", constraint.Field)))
	} else {
		field, ok := fields[constraint.Field]
		if !ok {
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("check constraint references unknown field %q", constraint.Field)))
		} else if !isCheckFieldType(field.Type) {
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("check constraint field %q type %q is not supported", constraint.Field, field.Type)))
		} else if _, ok := fieldTypes[constraint.Field]; !ok {
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("check constraint field %q type %q is unknown", constraint.Field, field.Type)))
		}
	}
	if !isCheckOperator(constraint.Operator) {
		*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("check constraint operator %q is not supported", constraint.Operator)))
	}
	validateCheckValue(constraint, problems)

	key := strings.Join([]string{constraint.Type, constraint.Field, constraint.Operator, checkValueKey(constraint.Value)}, "\x00")
	if _, ok := seenDefinitions[key]; ok {
		*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("duplicate check constraint on field %q", constraint.Field)))
	}
	seenDefinitions[key] = struct{}{}
}

func validateFieldReferences(line int, owner string, refs []string, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, allowed func(fieldtype.Definition) bool, capability string, problems *[]string) {
	seen := map[string]struct{}{}
	for _, ref := range refs {
		if strings.TrimSpace(ref) == "" {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s field names must not be empty", owner)))
			continue
		}
		if !fieldtype.IsName(ref) {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s field %q must be kebab-case", owner, ref)))
			continue
		}
		if _, ok := seen[ref]; ok {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s has duplicate field %q", owner, ref)))
			continue
		}
		seen[ref] = struct{}{}
		field, ok := fields[ref]
		if !ok {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s references unknown field %q", owner, ref)))
			continue
		}
		definition, ok := fieldTypes[ref]
		if !ok {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s field %q type %q is unknown", owner, ref, field.Type)))
			continue
		}
		if !allowed(definition) {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s field %q type %q cannot be %s", owner, ref, field.Type, capability)))
		}
	}
}

func isCheckOperator(value string) bool {
	switch value {
	case "eq", "neq", "gt", "gte", "lt", "lte", "in", "not-in":
		return true
	default:
		return false
	}
}

func validateCheckValue(constraint Constraint, problems *[]string) {
	if constraint.Operator == "in" || constraint.Operator == "not-in" {
		if constraint.Value.Kind == 0 {
			*problems = append(*problems, withLine(constraint.Line, "check constraint value is required"))
			return
		}
		if constraint.Value.Kind != yaml.SequenceNode || len(constraint.Value.Content) == 0 {
			*problems = append(*problems, withLine(constraint.Line, "check constraint value must be a non-empty list for in and not-in"))
			return
		}
		for _, item := range constraint.Value.Content {
			if item.Kind != yaml.ScalarNode {
				*problems = append(*problems, withLine(constraint.Line, "check constraint list values must be scalar"))
				return
			}
			if item.Tag == "!!null" {
				*problems = append(*problems, withLine(constraint.Line, "check constraint list values must not be null"))
				return
			}
		}
		return
	}
	if constraint.Value.Kind == 0 {
		*problems = append(*problems, withLine(constraint.Line, "check constraint value is required"))
		return
	}
	if constraint.Value.Kind != yaml.ScalarNode {
		*problems = append(*problems, withLine(constraint.Line, "check constraint value must be scalar"))
		return
	}
	if constraint.Value.Tag == "!!null" {
		*problems = append(*problems, withLine(constraint.Line, "check constraint value must not be null"))
	}
}

func isCheckFieldType(fieldType string) bool {
	switch fieldType {
	case "text", "email", "phone", "long-text", "int", "decimal", "currency", "boolean", "date", "datetime", "time", "select":
		return true
	default:
		return false
	}
}

func checkValueKey(node yaml.Node) string {
	if node.Kind == 0 {
		return ""
	}
	if node.Kind == yaml.SequenceNode {
		values := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			values = append(values, item.Tag+"="+item.Value)
		}
		return strings.Join(values, ",")
	}
	return node.Tag + "=" + node.Value
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
	if field.Index && !definition.AllowIndex {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q cannot be indexed", fieldLabel, field.Type)))
	}
	if field.Default.Kind != 0 && !definition.AllowDefault {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q does not support default values", fieldLabel, field.Type)))
	}
	if err := definition.Validate(field.Options); err != nil {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q options invalid for type %q: %v", fieldLabel, field.Type, err)))
	}
}

type sourceMap struct {
	entityLine      int
	fieldLines      []int
	indexLines      []int
	constraintLines []int
}

func (m sourceMap) apply(entity *Entity) {
	entity.Line = m.entityLine
	for i := range entity.Fields {
		if i >= len(m.fieldLines) {
			break
		}
		entity.Fields[i].Line = m.fieldLines[i]
	}
	for i := range entity.Indexes {
		if i >= len(m.indexLines) {
			break
		}
		entity.Indexes[i].Line = m.indexLines[i]
	}
	for i := range entity.Constraints {
		if i >= len(m.constraintLines) {
			break
		}
		entity.Constraints[i].Line = m.constraintLines[i]
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
		if value.Kind != yaml.SequenceNode {
			continue
		}
		switch key.Value {
		case "fields":
			for _, item := range value.Content {
				source.fieldLines = append(source.fieldLines, item.Line)
			}
		case "indexes":
			for _, item := range value.Content {
				source.indexLines = append(source.indexLines, item.Line)
			}
		case "constraints":
			for _, item := range value.Content {
				source.constraintLines = append(source.constraintLines, item.Line)
			}
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

// Package fieldtype defines and validates dygo Entity field types.
package fieldtype

import (
	"fmt"
	"sort"
	"strings"
)

// Validator validates type-specific field options.
type Validator func(Options) error

const (
	ValueString   = "string"
	ValueInteger  = "integer"
	ValueNumber   = "number"
	ValueBoolean  = "boolean"
	ValueDate     = "date"
	ValueDatetime = "datetime"
	ValueTime     = "time"
	ValueJSON     = "json"
	ValuePassword = "password"
)

// Behavior describes cross-layer behavior for one field type.
type Behavior struct {
	Stored          bool
	ColumnSuffix    string
	SQLType         string
	PlaceholderCast string
	ValueKind       string
	WriteOnly       bool
	Listable        bool
	NameRenderable  bool
	SystemName      bool
	Checkable       bool
	StudioEditor    string
	StudioDisplay   string
}

// Definition describes one registered field type.
type Definition struct {
	Name          string
	Label         string
	AllowRequired bool
	AllowUnique   bool
	AllowDefault  bool
	AllowIndex    bool
	Behavior      Behavior
	Validate      Validator
}

// Registry stores field type definitions by name.
type Registry struct {
	definitions map[string]Definition
}

// NewRegistry returns an empty field type registry.
func NewRegistry() Registry {
	return Registry{definitions: map[string]Definition{}}
}

// DefaultRegistry returns dygo's built-in field type registry.
func DefaultRegistry() Registry {
	registry := NewRegistry()
	for _, definition := range builtIns() {
		if err := registry.Register(definition); err != nil {
			panic(fmt.Sprintf("register built-in field type %q: %v", definition.Name, err))
		}
	}
	return registry
}

// Register adds a field type definition to the registry.
func (r Registry) Register(definition Definition) error {
	if r.definitions == nil {
		return fmt.Errorf("field type registry is not initialized")
	}
	if strings.TrimSpace(definition.Name) == "" {
		return fmt.Errorf("field type name is required")
	}
	if !IsName(definition.Name) {
		return fmt.Errorf("field type name %q must be kebab-case", definition.Name)
	}
	if strings.TrimSpace(definition.Label) == "" {
		return fmt.Errorf("field type %q label is required", definition.Name)
	}
	if _, ok := r.definitions[definition.Name]; ok {
		return fmt.Errorf("duplicate field type %q", definition.Name)
	}
	if definition.Validate == nil {
		definition.Validate = NoOptions
	}
	r.definitions[definition.Name] = definition
	return nil
}

// Get returns one field type definition by name.
func (r Registry) Get(name string) (Definition, bool) {
	definition, ok := r.definitions[name]
	return definition, ok
}

// Has reports whether name is registered.
func (r Registry) Has(name string) bool {
	_, ok := r.Get(name)
	return ok
}

// Names returns registered field type names in stable order.
func (r Registry) Names() []string {
	names := make([]string, 0, len(r.definitions))
	for name := range r.definitions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultDefinition returns one built-in field type definition by name.
func DefaultDefinition(name string) (Definition, bool) {
	for _, definition := range builtIns() {
		if definition.Name == name {
			return definition, true
		}
	}
	return Definition{}, false
}

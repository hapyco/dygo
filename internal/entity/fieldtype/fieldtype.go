// Package fieldtype defines and validates dygo Entity field types.
package fieldtype

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Options contains type-specific field settings from Entity metadata.
type Options struct {
	Values []string `yaml:"values,omitempty"`
	Entity string   `yaml:"entity,omitempty"`
}

// Validator validates type-specific field options.
type Validator func(Options) error

// Definition describes one registered field type.
type Definition struct {
	Name          string
	Label         string
	AllowRequired bool
	AllowUnique   bool
	AllowDefault  bool
	AllowIndex    bool
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

// IsName reports whether value is a valid dygo metadata name.
func IsName(value string) bool {
	return namePattern.MatchString(value)
}

// NoOptions rejects type-specific field options.
func NoOptions(options Options) error {
	var problems []string
	if len(options.Values) > 0 {
		problems = append(problems, "values are not supported")
	}
	if options.Entity != "" {
		problems = append(problems, "entity is not supported")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// SelectOptions validates select field options.
func SelectOptions(options Options) error {
	var problems []string
	if options.Entity != "" {
		problems = append(problems, "entity is not supported")
	}
	if len(options.Values) == 0 {
		problems = append(problems, "values are required")
	}
	seen := map[string]struct{}{}
	for _, value := range options.Values {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, "values must not be empty")
			continue
		}
		if _, ok := seen[value]; ok {
			problems = append(problems, fmt.Sprintf("duplicate value %q", value))
		}
		seen[value] = struct{}{}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// EntityOptions validates link-style field options.
func EntityOptions(options Options) error {
	var problems []string
	if len(options.Values) > 0 {
		problems = append(problems, "values are not supported")
	}
	if strings.TrimSpace(options.Entity) == "" {
		problems = append(problems, "entity is required")
	} else if !IsName(options.Entity) {
		problems = append(problems, fmt.Sprintf("entity %q must be kebab-case", options.Entity))
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func builtIns() []Definition {
	return []Definition{
		scalar("text", "Text", true, true, true, true),
		scalar("email", "Email", true, true, true, true),
		scalar("phone", "Phone", true, true, true, true),
		scalar("password", "Password", true, false, false, false),
		scalar("long-text", "Long Text", true, false, true, false),
		scalar("int", "Integer", true, true, true, true),
		scalar("bigint", "Big Integer", true, true, true, true),
		scalar("decimal", "Decimal", true, true, true, true),
		scalar("currency", "Currency", true, true, true, true),
		scalar("boolean", "Boolean", true, true, true, true),
		scalar("date", "Date", true, true, true, true),
		scalar("datetime", "Datetime", true, true, true, true),
		scalar("time", "Time", true, true, true, true),
		{
			Name:          "select",
			Label:         "Select",
			AllowRequired: true,
			AllowUnique:   true,
			AllowDefault:  true,
			AllowIndex:    true,
			Validate:      SelectOptions,
		},
		{
			Name:          "link",
			Label:         "Link",
			AllowRequired: true,
			AllowUnique:   true,
			AllowDefault:  true,
			AllowIndex:    true,
			Validate:      EntityOptions,
		},
		{
			Name:          "child-table",
			Label:         "Child Table",
			AllowRequired: true,
			AllowUnique:   false,
			AllowDefault:  false,
			AllowIndex:    false,
			Validate:      EntityOptions,
		},
		scalar("attachment", "Attachment", true, false, false, false),
		scalar("json", "JSON", true, false, false, false),
	}
}

func scalar(name string, label string, allowRequired bool, allowUnique bool, allowDefault bool, allowIndex bool) Definition {
	return Definition{
		Name:          name,
		Label:         label,
		AllowRequired: allowRequired,
		AllowUnique:   allowUnique,
		AllowDefault:  allowDefault,
		AllowIndex:    allowIndex,
		Validate:      NoOptions,
	}
}

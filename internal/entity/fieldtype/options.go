package fieldtype

import (
	"errors"
	"fmt"
	"strings"
)

// Options contains type-specific field settings from Entity metadata.
type Options struct {
	Values []string `yaml:"values,omitempty"`
	App    string   `yaml:"app,omitempty"`
	Entity string   `yaml:"entity,omitempty"`
}

// NoOptions rejects type-specific field options.
func NoOptions(options Options) error {
	var problems []string
	if len(options.Values) > 0 {
		problems = append(problems, "values are not supported")
	}
	if options.App != "" {
		problems = append(problems, "app is not supported")
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
	if options.App != "" {
		problems = append(problems, "app is not supported")
	}
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
	if strings.TrimSpace(options.App) != "" && !IsName(options.App) {
		problems = append(problems, fmt.Sprintf("app %q must be kebab-case", options.App))
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

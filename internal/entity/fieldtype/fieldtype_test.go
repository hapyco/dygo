package fieldtype

import (
	"strings"
	"testing"
)

func TestDefaultRegistryContainsBuiltIns(t *testing.T) {
	t.Parallel()

	registry := DefaultRegistry()
	for _, name := range []string{
		"text",
		"email",
		"phone",
		"password",
		"long-text",
		"int",
		"decimal",
		"currency",
		"boolean",
		"date",
		"datetime",
		"time",
		"select",
		"link",
		"child-table",
		"attachment",
		"json",
	} {
		if !registry.Has(name) {
			t.Fatalf("DefaultRegistry().Has(%q) = false, want true", name)
		}
	}
}

func TestRegister(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	definition := Definition{
		Name:          "rating",
		Label:         "Rating",
		AllowRequired: true,
		AllowUnique:   false,
		AllowDefault:  true,
	}
	if err := registry.Register(definition); err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}
	if !registry.Has("rating") {
		t.Fatal("Registry.Has(\"rating\") = false, want true")
	}
	got, ok := registry.Get("rating")
	if !ok {
		t.Fatal("Registry.Get(\"rating\") ok = false, want true")
	}
	if got.Label != "Rating" {
		t.Fatalf("Registry.Get(\"rating\").Label = %q, want Rating", got.Label)
	}
}

func TestRegisterRejectsInvalidDefinitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		definition Definition
		wantError  string
	}{
		{
			name:       "missing name",
			definition: Definition{Label: "Missing"},
			wantError:  "name is required",
		},
		{
			name:       "invalid name",
			definition: Definition{Name: "bad_name", Label: "Bad"},
			wantError:  "must be kebab-case",
		},
		{
			name:       "missing label",
			definition: Definition{Name: "rating"},
			wantError:  "label is required",
		},
		{
			name:       "duplicate",
			definition: Definition{Name: "text", Label: "Text"},
			wantError:  "duplicate field type",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			registry := DefaultRegistry()
			err := registry.Register(tt.definition)
			if err == nil {
				t.Fatal("Register() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Register() error = %q, want substring %q", err.Error(), tt.wantError)
			}
		})
	}
}

func TestOptionValidators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		validate  Validator
		options   Options
		wantError string
	}{
		{
			name:     "select accepts values",
			validate: SelectOptions,
			options:  Options{Values: []string{"New", "Qualified"}},
		},
		{
			name:      "select requires values",
			validate:  SelectOptions,
			options:   Options{},
			wantError: "values are required",
		},
		{
			name:      "select rejects duplicate values",
			validate:  SelectOptions,
			options:   Options{Values: []string{"New", "New"}},
			wantError: "duplicate value",
		},
		{
			name:     "link accepts entity",
			validate: EntityOptions,
			options:  Options{Entity: "company"},
		},
		{
			name:      "link requires entity",
			validate:  EntityOptions,
			options:   Options{},
			wantError: "entity is required",
		},
		{
			name:      "link rejects invalid entity",
			validate:  EntityOptions,
			options:   Options{Entity: "Company"},
			wantError: "must be kebab-case",
		},
		{
			name:      "no options rejects values",
			validate:  NoOptions,
			options:   Options{Values: []string{"A"}},
			wantError: "values are not supported",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate(tt.options)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("validate() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("validate() error = %q, want substring %q", err.Error(), tt.wantError)
			}
		})
	}
}

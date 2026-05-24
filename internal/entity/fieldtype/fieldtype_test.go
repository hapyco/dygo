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
		"bigint",
		"decimal",
		"currency",
		"boolean",
		"date",
		"datetime",
		"time",
		"select",
		"link",
		"collection",
		"attachment",
		"json",
	} {
		if !registry.Has(name) {
			t.Fatalf("DefaultRegistry().Has(%q) = false, want true", name)
		}
	}
}

func TestBuiltInBehavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		wantSQL       string
		wantCast      string
		wantSuffix    string
		wantKind      string
		wantEditor    string
		wantWriteOnly bool
		wantNameable  bool
		wantCheckable bool
	}{
		{name: "text", wantSQL: "text", wantKind: ValueString, wantEditor: "text", wantNameable: true, wantCheckable: true},
		{name: "password", wantSQL: "text", wantSuffix: "_hash", wantKind: ValuePassword, wantEditor: "password", wantWriteOnly: true},
		{name: "link", wantSQL: "bigint", wantCast: "bigint", wantSuffix: "_id", wantKind: ValueInteger, wantEditor: "link", wantNameable: true},
		{name: "json", wantSQL: "jsonb", wantCast: "jsonb", wantKind: ValueJSON, wantEditor: "json"},
		{name: "collection", wantEditor: "collection"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			definition, ok := DefaultDefinition(tt.name)
			if !ok {
				t.Fatalf("DefaultDefinition(%q) ok = false, want true", tt.name)
			}
			if definition.Behavior.SQLType != tt.wantSQL {
				t.Fatalf("SQLType = %q, want %q", definition.Behavior.SQLType, tt.wantSQL)
			}
			if definition.Behavior.PlaceholderCast != tt.wantCast {
				t.Fatalf("PlaceholderCast = %q, want %q", definition.Behavior.PlaceholderCast, tt.wantCast)
			}
			if definition.Behavior.ColumnSuffix != tt.wantSuffix {
				t.Fatalf("ColumnSuffix = %q, want %q", definition.Behavior.ColumnSuffix, tt.wantSuffix)
			}
			if definition.Behavior.ValueKind != tt.wantKind {
				t.Fatalf("ValueKind = %q, want %q", definition.Behavior.ValueKind, tt.wantKind)
			}
			if definition.Behavior.StudioEditor != tt.wantEditor {
				t.Fatalf("StudioEditor = %q, want %q", definition.Behavior.StudioEditor, tt.wantEditor)
			}
			if definition.Behavior.WriteOnly != tt.wantWriteOnly {
				t.Fatalf("WriteOnly = %v, want %v", definition.Behavior.WriteOnly, tt.wantWriteOnly)
			}
			if definition.Behavior.NameRenderable != tt.wantNameable {
				t.Fatalf("NameRenderable = %v, want %v", definition.Behavior.NameRenderable, tt.wantNameable)
			}
			if definition.Behavior.Checkable != tt.wantCheckable {
				t.Fatalf("Checkable = %v, want %v", definition.Behavior.Checkable, tt.wantCheckable)
			}
		})
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
			validate: LinkOptions,
			options:  Options{Entity: "company"},
		},
		{
			name:     "link accepts disabled foreign key",
			validate: LinkOptions,
			options:  Options{Entity: "activity", ForeignKey: boolOption(false)},
		},
		{
			name:      "link requires entity",
			validate:  LinkOptions,
			options:   Options{},
			wantError: "entity is required",
		},
		{
			name:      "link rejects invalid entity",
			validate:  LinkOptions,
			options:   Options{Entity: "Company"},
			wantError: "must be kebab-case",
		},
		{
			name:      "collection rejects foreign key",
			validate:  EntityOptions,
			options:   Options{Entity: "invoice-item", ForeignKey: boolOption(false)},
			wantError: "foreign-key is not supported",
		},
		{
			name:      "no options rejects values",
			validate:  NoOptions,
			options:   Options{Values: []string{"A"}},
			wantError: "values are not supported",
		},
		{
			name:      "select rejects foreign key",
			validate:  SelectOptions,
			options:   Options{Values: []string{"New"}, ForeignKey: boolOption(false)},
			wantError: "foreign-key is not supported",
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

func boolOption(value bool) *bool {
	return &value
}

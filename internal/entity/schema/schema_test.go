package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	entity, err := Decode([]byte(validEntityYAML()), fieldtype.DefaultRegistry())
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if entity.Name != "lead" {
		t.Fatalf("Decode().Name = %q, want lead", entity.Name)
	}
	if entity.Line != 1 {
		t.Fatalf("Decode().Line = %d, want 1", entity.Line)
	}
	if len(entity.Fields) != 4 {
		t.Fatalf("Decode().Fields len = %d, want 4", len(entity.Fields))
	}
	if entity.Fields[0].Line == 0 {
		t.Fatal("Decode().Fields[0].Line = 0, want source line")
	}
	if entity.Fields[1].Options.Values[0] != "New" {
		t.Fatalf("Decode().Fields[1].Options.Values[0] = %q, want New", entity.Fields[1].Options.Values[0])
	}
	if entity.Fields[1].Default.Value != "New" {
		t.Fatalf("Decode().Fields[1].Default.Value = %q, want New", entity.Fields[1].Default.Value)
	}
}

func TestLoadFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "lead.yml")
	if err := os.WriteFile(path, []byte(validEntityYAML()), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	entity, err := LoadFile(path, fieldtype.DefaultRegistry())
	if err != nil {
		t.Fatalf("LoadFile() error = %v, want nil", err)
	}
	if entity.Name != "lead" {
		t.Fatalf("LoadFile().Name = %q, want lead", entity.Name)
	}
}

func TestDecodeWithCustomFieldType(t *testing.T) {
	t.Parallel()

	registry := fieldtype.DefaultRegistry()
	if err := registry.Register(fieldtype.Definition{
		Name:          "rating",
		Label:         "Rating",
		AllowRequired: true,
		AllowUnique:   false,
		AllowDefault:  true,
	}); err != nil {
		t.Fatalf("Register(rating) error = %v", err)
	}

	entity, err := Decode([]byte(`
name: review
label: Review
plural-name: reviews
plural-label: Reviews
fields:
  - name: score
    label: Score
    type: rating
`), registry)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if entity.Fields[0].Type != "rating" {
		t.Fatalf("Decode().Fields[0].Type = %q, want rating", entity.Fields[0].Type)
	}
}

func TestDecodeRejectsInvalidEntitySchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name: "missing entity fields",
			body: `
description: Missing required fields
`,
			wantError: "name is required",
		},
		{
			name: "missing plural name",
			body: `
name: lead
label: Lead
plural-label: Leads
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "plural-name is required",
		},
		{
			name: "invalid plural name",
			body: `
name: lead
label: Lead
plural-name: LeadRecords
plural-label: Leads
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "plural-name",
		},
		{
			name: "missing plural label",
			body: `
name: lead
label: Lead
plural-name: leads
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "plural-label is required",
		},
		{
			name: "invalid entity name",
			body: `
name: BadName
label: Bad
plural-name: bads
plural-label: Bads
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "must be kebab-case",
		},
		{
			name: "missing fields",
			body: `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
`,
			wantError: "at least one field",
		},
		{
			name: "duplicate field names",
			body: `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: title
    label: Title
    type: text
  - name: title
    label: Title Again
    type: text
`,
			wantError: "duplicate field",
		},
		{
			name: "field validation includes line",
			body: `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: title
    type: text
`,
			wantError: "line 6",
		},
		{
			name: "unknown field type",
			body: `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: title
    label: Title
    type: mystery
`,
			wantError: "unknown type",
		},
		{
			name: "unknown yaml field",
			body: `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
unknown: true
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "field unknown not found",
		},
		{
			name: "duplicate yaml key",
			body: `
name: lead
name: duplicate
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "duplicate key",
		},
		{
			name: "select missing values",
			body: `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: status
    label: Status
    type: select
`,
			wantError: "values are required",
		},
		{
			name: "link missing entity",
			body: `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: company
    label: Company
    type: link
`,
			wantError: "entity is required",
		},
		{
			name: "child table invalid entity",
			body: `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: contacts
    label: Contacts
    type: child-table
    options:
      entity: LeadContact
`,
			wantError: "must be kebab-case",
		},
		{
			name: "unique unsupported",
			body: `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: details
    label: Details
    type: long-text
    unique: true
`,
			wantError: "cannot be unique",
		},
		{
			name: "default unsupported",
			body: `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: payload
    label: Payload
    type: json
    default: {}
`,
			wantError: "does not support default",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Decode([]byte(strings.TrimSpace(tt.body)+"\n"), fieldtype.DefaultRegistry())
			if err == nil {
				t.Fatal("Decode() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Decode() error = %q, want substring %q", err.Error(), tt.wantError)
			}
		})
	}
}

func validEntityYAML() string {
	return strings.TrimSpace(`
name: lead
label: Lead
plural-name: leads
plural-label: Leads
description: Sales lead
fields:
  - name: full-name
    label: Full Name
    type: text
    required: true
  - name: status
    label: Status
    type: select
    default: New
    options:
      values:
        - New
        - Qualified
        - Lost
  - name: company
    label: Company
    type: link
    options:
      entity: company
  - name: contacts
    label: Contacts
    type: child-table
    options:
      entity: lead-contact
`) + "\n"
}

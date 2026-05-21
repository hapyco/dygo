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
	if entity.Icon != "contact" {
		t.Fatalf("Decode().Icon = %q, want contact", entity.Icon)
	}
	if got := entity.EffectiveNaming(); got.Strategy != NamingStrategyRandom || got.Length != DefaultRandomNameLength {
		t.Fatalf("Decode().EffectiveNaming() = %+v, want default random length", got)
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
	if entity.Fields[0].Check == nil || entity.Fields[0].Check.Operator != "neq" || entity.Fields[0].Check.Value.Value != "" {
		t.Fatalf("Decode().Fields[0].Check = %+v, want neq empty string", entity.Fields[0].Check)
	}
	if entity.Fields[1].Options.Values[0] != "New" {
		t.Fatalf("Decode().Fields[1].Options.Values[0] = %q, want New", entity.Fields[1].Options.Values[0])
	}
	if entity.Fields[1].Default.Value != "New" {
		t.Fatalf("Decode().Fields[1].Default.Value = %q, want New", entity.Fields[1].Default.Value)
	}
	if !entity.Fields[2].Index {
		t.Fatal("Decode().Fields[2].Index = false, want true")
	}
	if len(entity.Indexes) != 1 || entity.Indexes[0].EffectiveName(entity) != "by-company-status" {
		t.Fatalf("Decode().Indexes = %+v, want named index", entity.Indexes)
	}
	if len(entity.Constraints) != 2 {
		t.Fatalf("Decode().Constraints len = %d, want 2", len(entity.Constraints))
	}
	if entity.Constraints[0].EffectiveName(entity) != "lead-company-status-key" {
		t.Fatalf("unique constraint name = %q, want lead-company-status-key", entity.Constraints[0].EffectiveName(entity))
	}
	if entity.Constraints[1].Value.Value != "Lost" {
		t.Fatalf("check constraint value = %q, want Lost", entity.Constraints[1].Value.Value)
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

func TestDecodeNamingStrategies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		want      Naming
		wantError string
	}{
		{
			name: "random",
			body: `
name: ticket
label: Ticket
naming:
  strategy: random
  length: 24
fields:
  - name: title
    label: Title
    type: text
`,
			want: Naming{Strategy: NamingStrategyRandom, Length: 24},
		},
		{
			name: "field",
			body: `
name: user
label: User
naming:
  strategy: field
  field: email
fields:
  - name: email
    label: Email
    type: email
    required: true
    unique: true
`,
			want: Naming{Strategy: NamingStrategyField, Field: "email"},
		},
		{
			name: "series",
			body: `
name: sales-invoice
label: Sales Invoice
naming:
  strategy: series
  pattern: "SINV-{YYYY}-{MM}-{#####}"
fields:
  - name: status
    label: Status
    type: text
`,
			want: Naming{Strategy: NamingStrategySeries, Pattern: "SINV-{YYYY}-{MM}-{#####}"},
		},
		{
			name: "reserved name field",
			body: `
name: role
label: Role
fields:
  - name: name
    label: Name
    type: text
    required: true
    unique: true
`,
			wantError: `field "name" is reserved`,
		},
		{
			name: "name field allowed as naming source",
			body: `
name: role
label: Role
naming:
  strategy: field
  field: name
fields:
  - name: name
    label: Name
    type: text
    required: true
    unique: true
`,
			want: Naming{Strategy: NamingStrategyField, Field: "name"},
		},
		{
			name: "invalid random length",
			body: `
name: ticket
label: Ticket
naming:
  strategy: random
  length: 3
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "random naming length",
		},
		{
			name: "field source must be unique",
			body: `
name: user
label: User
naming:
  strategy: field
  field: email
fields:
  - name: email
    label: Email
    type: email
    required: true
`,
			wantError: "must be unique",
		},
		{
			name: "series requires counter",
			body: `
name: invoice
label: Invoice
naming:
  strategy: series
  pattern: "INV-{YYYY}"
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "exactly one hash counter",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entity, err := Decode([]byte(tt.body), fieldtype.DefaultRegistry())
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("Decode() error = %v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("Decode() error = %v, want nil", err)
			}
			got := entity.EffectiveNaming()
			if got.Strategy != tt.want.Strategy || got.Length != tt.want.Length || got.Field != tt.want.Field || got.Pattern != tt.want.Pattern {
				t.Fatalf("EffectiveNaming() = %+v, want %+v", got, tt.want)
			}
		})
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
			name: "invalid entity name",
			body: `
name: BadName
label: Bad
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
`,
			wantError: "at least one field",
		},
		{
			name: "invalid route slug",
			body: `
name: lead
label: Lead
route:
  slug: BadSlug
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "route slug",
		},
		{
			name: "duplicate field names",
			body: `
name: lead
label: Lead
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
fields:
  - name: title
    type: text
`,
			wantError: "line 4",
		},
		{
			name: "unknown field type",
			body: `
name: lead
label: Lead
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
fields:
  - name: details
    label: Details
    type: long-text
    unique: true
`,
			wantError: "cannot be unique",
		},
		{
			name: "password unique unsupported",
			body: `
name: user
label: User
fields:
  - name: password
    label: Password
    type: password
    unique: true
`,
			wantError: "cannot be unique",
		},
		{
			name: "default unsupported",
			body: `
name: lead
label: Lead
fields:
  - name: payload
    label: Payload
    type: json
    default: {}
`,
			wantError: "does not support default",
		},
		{
			name: "password default unsupported",
			body: `
name: user
label: User
fields:
  - name: password
    label: Password
    type: password
    default: secret
`,
			wantError: "does not support default",
		},
		{
			name: "index unsupported",
			body: `
name: lead
label: Lead
fields:
  - name: details
    label: Details
    type: long-text
    index: true
`,
			wantError: "cannot be indexed",
		},
		{
			name: "password index unsupported",
			body: `
name: user
label: User
fields:
  - name: password
    label: Password
    type: password
    index: true
`,
			wantError: "cannot be indexed",
		},
		{
			name: "password options unsupported",
			body: `
name: user
label: User
fields:
  - name: password
    label: Password
    type: password
    options:
      values: [A]
`,
			wantError: "values are not supported",
		},
		{
			name: "top-level index password unsupported",
			body: `
name: user
label: User
fields:
  - name: password
    label: Password
    type: password
indexes:
  - fields: [password]
`,
			wantError: "cannot be indexed",
		},
		{
			name: "top-level unique password unsupported",
			body: `
name: user
label: User
fields:
  - name: password
    label: Password
    type: password
  - name: email
    label: Email
    type: email
constraints:
  - type: unique
    fields: [password, email]
`,
			wantError: "cannot be unique",
		},
		{
			name: "invalid top-level index name",
			body: `
name: lead
label: Lead
fields:
  - name: status
    label: Status
    type: text
indexes:
  - name: BadName
    fields: [status]
`,
			wantError: "index name",
		},
		{
			name: "index missing fields",
			body: `
name: lead
label: Lead
fields:
  - name: status
    label: Status
    type: text
indexes:
  - name: by-status
`,
			wantError: "index fields are required",
		},
		{
			name: "index duplicate field references",
			body: `
name: lead
label: Lead
fields:
  - name: status
    label: Status
    type: text
indexes:
  - fields: [status, status]
`,
			wantError: "duplicate field",
		},
		{
			name: "index unknown field",
			body: `
name: lead
label: Lead
fields:
  - name: status
    label: Status
    type: text
indexes:
  - fields: [missing]
`,
			wantError: "references unknown field",
		},
		{
			name: "index unsupported field type",
			body: `
name: lead
label: Lead
fields:
  - name: details
    label: Details
    type: long-text
indexes:
  - fields: [details]
`,
			wantError: "cannot be indexed",
		},
		{
			name: "duplicate generated index names",
			body: `
name: lead
label: Lead
fields:
  - name: status
    label: Status
    type: text
indexes:
  - fields: [status]
  - fields: [status]
`,
			wantError: "duplicate index name",
		},
		{
			name: "unique constraint requires two fields",
			body: `
name: user-role
label: User Role
fields:
  - name: user
    label: User
    type: link
    options:
      entity: user
constraints:
  - type: unique
    fields: [user]
`,
			wantError: "at least two fields",
		},
		{
			name: "unique constraint unknown field",
			body: `
name: user-role
label: User Role
fields:
  - name: user
    label: User
    type: text
  - name: role
    label: Role
    type: text
constraints:
  - type: unique
    fields: [user, missing]
`,
			wantError: "references unknown field",
		},
		{
			name: "unique constraint unsupported field type",
			body: `
name: report
label: Report
fields:
  - name: payload
    label: Payload
    type: json
  - name: status
    label: Status
    type: text
constraints:
  - type: unique
    fields: [payload, status]
`,
			wantError: "cannot be unique",
		},
		{
			name: "check constraint unknown operator",
			body: `
name: invoice
label: Invoice
fields:
  - name: amount
    label: Amount
    type: currency
constraints:
  - type: check
    field: amount
    operator: between
    value: 0
`,
			wantError: "operator",
		},
		{
			name: "check constraint missing value",
			body: `
name: invoice
label: Invoice
fields:
  - name: amount
    label: Amount
    type: currency
constraints:
  - type: check
    field: amount
    operator: gte
`,
			wantError: "value is required",
		},
		{
			name: "check constraint in requires list",
			body: `
name: lead
label: Lead
fields:
  - name: status
    label: Status
    type: text
constraints:
  - type: check
    field: status
    operator: in
    value: New
`,
			wantError: "non-empty list",
		},
		{
			name: "check constraint unsupported field type",
			body: `
name: event
label: Event
fields:
  - name: payload
    label: Payload
    type: json
constraints:
  - type: check
    field: payload
    operator: eq
    value: {}
`,
			wantError: "not supported",
		},
		{
			name: "check constraint password unsupported",
			body: `
name: user
label: User
fields:
  - name: password
    label: Password
    type: password
constraints:
  - type: check
    field: password
    operator: neq
    value: ""
`,
			wantError: "not supported",
		},
		{
			name: "field check invalid operator",
			body: `
name: invoice
label: Invoice
fields:
  - name: amount
    label: Amount
    type: currency
    check:
      operator: between
      value: 0
`,
			wantError: "operator",
		},
		{
			name: "field check missing value",
			body: `
name: invoice
label: Invoice
fields:
  - name: amount
    label: Amount
    type: currency
    check:
      operator: gte
`,
			wantError: "value is required",
		},
		{
			name: "field check in requires list",
			body: `
name: lead
label: Lead
fields:
  - name: status
    label: Status
    type: text
    check:
      operator: in
      value: New
`,
			wantError: "non-empty list",
		},
		{
			name: "field check unsupported field type",
			body: `
name: user
label: User
fields:
  - name: password
    label: Password
    type: password
    check:
      operator: neq
      value: ""
`,
			wantError: "does not support checks",
		},
		{
			name: "invalid constraint name",
			body: `
name: lead
label: Lead
fields:
  - name: status
    label: Status
    type: text
  - name: source
    label: Source
    type: text
constraints:
  - name: BadName
    type: unique
    fields: [status, source]
`,
			wantError: "constraint name",
		},
		{
			name: "unknown constraint type",
			body: `
name: lead
label: Lead
fields:
  - name: status
    label: Status
    type: text
constraints:
  - type: foreign-key
    fields: [status]
`,
			wantError: "not supported",
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
description: Sales lead
icon: contact
fields:
  - name: full-name
    label: Full Name
    type: text
    required: true
    check:
      operator: neq
      value: ""
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
    index: true
    options:
      entity: company
  - name: contacts
    label: Contacts
    type: child-table
    options:
      entity: lead-contact
indexes:
  - name: by-company-status
    fields: [company, status]
constraints:
  - type: unique
    fields: [company, status]
  - type: check
    field: status
    operator: neq
    value: Lost
`) + "\n"
}

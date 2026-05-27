package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/entity/fieldtype"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	entity, err := Decode([]byte(validEntityYAML()), fieldtype.DefaultRegistry())
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if entity.Name != "" {
		t.Fatalf("Decode().Name = %q, want empty path-derived name before LoadFile", entity.Name)
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
	named := entity
	named.Name = "lead"
	if len(entity.Indexes) != 1 || entity.Indexes[0].EffectiveName(named) != "by-company-status" {
		t.Fatalf("Decode().Indexes = %+v, want named index", entity.Indexes)
	}
	if len(entity.Constraints) != 2 {
		t.Fatalf("Decode().Constraints len = %d, want 2", len(entity.Constraints))
	}
	if entity.Constraints[0].EffectiveName(named) != "lead-company-status-key" {
		t.Fatalf("unique constraint name = %q, want lead-company-status-key", entity.Constraints[0].EffectiveName(named))
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

func TestLoadFileDerivesCanonicalBundleNameFromParentFolder(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "lead", "entity.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
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

func TestDecodeCollectionEntityNaming(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		wantStrategy string
		wantLength   int
		wantError    string
	}{
		{
			name: "omitted name uses framework random naming",
			body: `
label: Invoice Item
fields:
  - name: item-code
    label: Item Code
    type: text
`,
			wantStrategy: NamingStrategyRandom,
			wantLength:   CollectionRowNameLength,
		},
		{
			name: "explicit random rejected",
			body: `
label: Invoice Item
name:
  strategy: random
fields:
  - name: item-code
    label: Item Code
    type: text
`,
			wantError: "collection Entities do not support explicit name configuration",
		},
		{
			name: "manual rejected",
			body: `
label: Invoice Item
name:
  strategy: manual
fields:
  - name: item-code
    label: Item Code
    type: text
`,
			wantError: "collection Entities do not support explicit name configuration",
		},
		{
			name: "series rejected",
			body: `
label: Invoice Item
name:
  strategy: series
  pattern: "ITEM-{####}"
fields:
  - name: item-code
    label: Item Code
    type: text
`,
			wantError: "collection Entities do not support explicit name configuration",
		},
		{
			name: "format rejected",
			body: `
label: Invoice Item
name:
  strategy: format
  format: "{item-code}"
fields:
  - name: item-code
    label: Item Code
    type: text
    required: true
`,
			wantError: "collection Entities do not support explicit name configuration",
		},
		{
			name: "nested collection rejected",
			body: `
label: Invoice Item
fields:
  - name: taxes
    label: Taxes
    type: collection
    options:
      entity: invoice-tax
`,
			wantError: "collection Entities cannot define collection fields in v1",
		},
		{
			name: "ordinal field rejected",
			body: `
label: Invoice Item
fields:
  - name: ordinal
    label: Ordinal
    type: bigint
`,
			wantError: `collection field "ordinal" is reserved for framework collection row storage`,
		},
		{
			name: "parent entity id field rejected",
			body: `
label: Invoice Item
fields:
  - name: parent-entity-id
    label: Parent Entity ID
    type: bigint
`,
			wantError: `collection field "parent-entity-id" is reserved for framework collection row storage`,
		},
		{
			name: "parent record id field rejected",
			body: `
label: Invoice Item
fields:
  - name: parent-record-id
    label: Parent Record ID
    type: bigint
`,
			wantError: `collection field "parent-record-id" is reserved for framework collection row storage`,
		},
		{
			name: "parent field id field rejected",
			body: `
label: Invoice Item
fields:
  - name: parent-field-id
    label: Parent Field ID
    type: bigint
`,
			wantError: `collection field "parent-field-id" is reserved for framework collection row storage`,
		},
		{
			name: "position field allowed",
			body: `
label: Invoice Item
fields:
  - name: position
    label: Position
    type: text
`,
			wantStrategy: NamingStrategyRandom,
			wantLength:   CollectionRowNameLength,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			entity, err := DecodeWithOptions([]byte(tt.body), fieldtype.DefaultRegistry(), DecodeOptions{IsCollection: true})
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("DecodeWithOptions() error = %v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeWithOptions() error = %v, want nil", err)
			}
			if got := entity.EffectiveNaming(); got.Strategy != tt.wantStrategy {
				t.Fatalf("EffectiveNaming().Strategy = %q, want %q", got.Strategy, tt.wantStrategy)
			}
			if got := entity.EffectiveNaming(); got.Length != tt.wantLength {
				t.Fatalf("EffectiveNaming().Length = %d, want %d", got.Length, tt.wantLength)
			}
		})
	}
}

func TestLoadFileRejectsInvalidEntityFilename(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "BadName.yml")
	if err := os.WriteFile(path, []byte(validEntityYAML()), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadFile(path, fieldtype.DefaultRegistry())
	if err == nil {
		t.Fatal("LoadFile() error = nil, want invalid filename error")
	}
	if !strings.Contains(err.Error(), `entity filename "BadName.yml" must be kebab-case`) {
		t.Fatalf("LoadFile() error = %q, want invalid filename context", err.Error())
	}
}

func TestDecodeSingleEntity(t *testing.T) {
	t.Parallel()

	entity, err := Decode([]byte(`
label: Invoice Settings
is-single: true
fields:
  - name: default-due-days
    label: Default Due Days
    type: int
    required: true
    default: 30
`), fieldtype.DefaultRegistry())
	if err != nil {
		t.Fatalf("Decode(single) error = %v, want nil", err)
	}
	if !entity.IsSingle {
		t.Fatal("Decode(single).IsSingle = false, want true")
	}
}

func TestDecodeSystemEntity(t *testing.T) {
	t.Parallel()

	entity, err := Decode([]byte(`
label: Session
is-system: true
name:
  strategy: random
fields:
  - name: token
    label: Token
    type: password
    required: true
`), fieldtype.DefaultRegistry())
	if err != nil {
		t.Fatalf("Decode(system) error = %v, want nil", err)
	}
	if !entity.IsSystem {
		t.Fatal("Decode(system).IsSystem = false, want true")
	}
}

func TestDecodeSingleEntityValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      string
		wantError string
	}{
		{
			name: "underscore key rejected",
			body: `
label: Invoice Settings
is_single: true
fields:
  - name: default-due-days
    label: Default Due Days
    type: int
`,
			wantError: "is_single",
		},
		{
			name: "underscore system key rejected",
			body: `
label: Session
is_system: true
fields:
  - name: token
    label: Token
    type: password
`,
			wantError: "is_system",
		},
		{
			name: "explicit name configuration rejected",
			body: `
label: Invoice Settings
is-single: true
name:
  strategy: random
fields:
  - name: default-due-days
    label: Default Due Days
    type: int
`,
			wantError: "single Entities do not support explicit name configuration",
		},
		{
			name: "required field needs default",
			body: `
label: Invoice Settings
is-single: true
fields:
  - name: default-due-days
    label: Default Due Days
    type: int
    required: true
`,
			wantError: `required field "default-due-days" must define a non-null default`,
		},
		{
			name: "required field rejects null default",
			body: `
label: Invoice Settings
is-single: true
fields:
  - name: default-due-days
    label: Default Due Days
    type: int
    required: true
    default: null
`,
			wantError: `required field "default-due-days" default must not be null`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Decode([]byte(tt.body), fieldtype.DefaultRegistry())
			if err == nil {
				t.Fatal("Decode(single invalid) error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("Decode(single invalid) error = %q, want substring %q", err.Error(), tt.wantError)
			}
		})
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
label: Review
name:
  strategy: random
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
label: Ticket
name:
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
			name: "strategy required",
			body: `
label: Ticket
name: {}
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "name strategy is required",
		},
		{
			name: "format",
			body: `
label: User
name:
  strategy: format
  format: "{email}"
fields:
  - name: email
    label: Email
    type: email
    required: true
    unique: true
`,
			want: Naming{Strategy: NamingStrategyFormat, Format: "{email}"},
		},
		{
			name: "series",
			body: `
label: Sales Invoice
name:
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
			name: "format with fields",
			body: `
label: Entity
name:
  strategy: format
  format: "{app}.{key}"
fields:
  - name: app
    label: App
    type: link
    required: true
    options:
      entity: app
  - name: key
    label: Key
    type: text
    required: true
`,
			want: Naming{Strategy: NamingStrategyFormat, Format: "{app}.{key}"},
		},
		{
			name: "reserved name field",
			body: `
label: Role
name:
  strategy: random
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
			name: "manual name",
			body: `
label: Role
name:
  strategy: manual
  label: Name
fields:
  - name: label
    label: Label
    type: text
    required: true
`,
			want: Naming{Strategy: NamingStrategyManual, Label: "Name"},
		},
		{
			name: "invalid random length",
			body: `
label: Ticket
name:
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
			name: "format source must be required",
			body: `
label: User
name:
  strategy: format
  format: "{email}"
fields:
  - name: email
    label: Email
    type: email
`,
			wantError: "must be required",
		},
		{
			name: "series requires counter",
			body: `
label: Invoice
name:
  strategy: series
  pattern: "INV-{YYYY}"
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "exactly one hash counter",
		},
		{
			name: "format source must exist",
			body: `
label: Entity
name:
  strategy: format
  format: "{app}.{missing}"
fields:
  - name: app
    label: App
    type: link
    required: true
    options:
      entity: app
`,
			wantError: `unknown field "missing"`,
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
			if got.Strategy != tt.want.Strategy || got.Label != tt.want.Label || got.Length != tt.want.Length || got.Pattern != tt.want.Pattern || got.Format != tt.want.Format {
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
			wantError: "label is required",
		},
		{
			name: "invalid name config",
			body: `
name: lead
label: Lead
fields:
  - name: title
    label: Title
    type: text
`,
			wantError: "cannot unmarshal",
		},
		{
			name: "missing fields",
			body: `
label: Lead
`,
			wantError: "at least one field",
		},
		{
			name: "invalid route slug",
			body: `
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
			name: "reserved field name",
			body: `
label: Lead
fields:
  - name: search
    label: Search
    type: text
`,
			wantError: `field "search" is reserved`,
		},
		{
			name: "field validation includes line",
			body: `
label: Lead
fields:
  - name: title
    type: text
`,
			wantError: "line 3",
		},
		{
			name: "unknown field type",
			body: `
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
label: Lead
label: Lead Again
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
label: Lead
fields:
  - name: company
    label: Company
    type: link
`,
			wantError: "entity is required",
		},
		{
			name: "child table field type rejected",
			body: `
label: Lead
fields:
  - name: contacts
    label: Contacts
    type: child-table
    options:
      entity: lead-contact
`,
			wantError: "unknown type",
		},
		{
			name: "collection invalid entity",
			body: `
label: Lead
fields:
  - name: contacts
    label: Contacts
    type: collection
    options:
      entity: LeadContact
`,
			wantError: "must be kebab-case",
		},
		{
			name: "unique unsupported",
			body: `
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
label: Lead
description: Sales lead
icon: contact
name:
  strategy: random
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
    type: collection
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

package schema

import (
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/entity/fieldtype"
)

func TestNormalizeFormLayoutFlattensTabs(t *testing.T) {
	entity := Entity{
		Label:  "Currency",
		Naming: Naming{Strategy: NamingStrategyFormat, Format: "{code}"},
		Tabs: []Tab{
			{
				Label: "Identity",
				Name:  "identity",
				Icon:  "badge-check",
				Fields: []Field{
					{Name: "code", Label: "Code", Type: "text", Required: true},
					{Name: "split", Label: "Split", Type: "column"},
					{Name: "symbol", Label: "Symbol", Type: "text"},
				},
			},
			{
				Label: "Formatting",
				Name:  "formatting",
				Fields: []Field{
					{Name: "rounding", Label: "Rounding", Type: "section", Description: "Cash rounding"},
					{Name: "enabled", Label: "Enabled", Type: "boolean"},
				},
			},
		},
	}
	if err := normalizeFormLayout(&entity); err != nil {
		t.Fatalf("normalizeFormLayout() error = %v", err)
	}
	if len(entity.Fields) != 3 {
		t.Fatalf("Fields len = %d, want 3", len(entity.Fields))
	}
	if entity.Form == nil || len(entity.Form.Tabs) != 2 {
		t.Fatalf("Form tabs = %#v, want 2", entity.Form)
	}
	if entity.Form.Tabs[0].Key != "identity" || entity.Form.Tabs[0].Icon != "badge-check" {
		t.Fatalf("Identity tab = %#v", entity.Form.Tabs[0])
	}
	if got := entity.Form.Tabs[0].Items; len(got) != 3 || got[1].Kind != FormItemKindColumn {
		t.Fatalf("Identity items = %#v", got)
	}
	if got := entity.Form.Tabs[1].Items; len(got) != 2 || got[0].Kind != FormItemKindSection || got[0].Description != "Cash rounding" {
		t.Fatalf("Formatting items = %#v", got)
	}
	if err := entity.Validate(fieldtype.DefaultRegistry()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDecodeTabsEntity(t *testing.T) {
	const source = `
label: Currency
name:
  strategy: format
  format: "{code}"
tabs:
  - tab: Identity
    name: identity
    icon: badge-check
    fields:
      - name: code
        label: Code
        type: text
        required: true
      - name: split
        label: Split
        type: column
      - name: symbol
        label: Symbol
        type: text
`
	entity, err := Decode([]byte(source), fieldtype.DefaultRegistry())
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(entity.Fields) != 2 {
		t.Fatalf("Fields = %#v, want code and symbol", entity.Fields)
	}
	if entity.Form == nil || len(entity.Form.Tabs) != 1 || entity.Form.Tabs[0].Key != "identity" || entity.Form.Tabs[0].Icon != "badge-check" {
		t.Fatalf("Form = %#v", entity.Form)
	}
}

func TestNormalizeFormLayoutRequiresTabName(t *testing.T) {
	err := normalizeFormLayout(&Entity{
		Label: "Currency",
		Tabs: []Tab{{
			Label:  "Identity",
			Fields: []Field{{Name: "code", Label: "Code", Type: "text"}},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "requires name") {
		t.Fatalf("normalizeFormLayout() error = %v, want requires name", err)
	}
}

func TestFormLayoutSourceLines(t *testing.T) {
	source := `label: Contact
name:
  strategy: random
tabs:
  - tab: Details
    name: details
    fields:
      - name: email
        label: Email
        type: email
      - type: column
      - type: section
        label: Notes
        description: Additional information
      - name: notes
        label: Notes
        type: text
`
	entity, err := Decode([]byte(source), fieldtype.DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if entity.Fields[0].Line != 8 || entity.Fields[1].Line != 15 {
		t.Fatalf("field source lines = %d, %d", entity.Fields[0].Line, entity.Fields[1].Line)
	}
}

func TestDecodeRejectsInvalidFormLayouts(t *testing.T) {
	const prefix = "label: Contact\nname: {strategy: random}\n"
	for _, tc := range []struct{ name, body, want string }{
		{"mixed empty fields", "fields: []\ntabs:\n  - tab: Details\n    name: details\n    fields: [{name: email, label: Email, type: email}]\n", "both fields and tabs"},
		{"empty tab", "tabs: [{tab: Details, name: details, fields: []}]\n", "requires fields"},
		{"marker storage setting", "tabs:\n  - tab: Details\n    name: details\n    fields:\n      - {type: column, required: true}\n      - {name: email, label: Email, type: email}\n", "line 7: layout markers"},
		{"invalid nested field", "tabs:\n  - tab: Details\n    name: details\n    fields:\n      - {name: email, label: Email, type: missing}\n", "line 7:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(prefix+tc.body), fieldtype.DefaultRegistry())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

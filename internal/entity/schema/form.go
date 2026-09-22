package schema

import (
	"fmt"
	"strings"

	"github.com/hapyco/dygo/internal/entity/fieldtype"
)

const (
	FormItemKindField   = "field"
	FormItemKindColumn  = "column"
	FormItemKindSection = "section"

	layoutFieldTypeColumn  = "column"
	layoutFieldTypeSection = "section"
)

// Tab is one authored form tab containing storage fields and layout markers.
type Tab struct {
	Line   int     `yaml:"-"`
	Label  string  `yaml:"tab"`
	Name   string  `yaml:"name"`
	Icon   string  `yaml:"icon,omitempty"`
	Fields []Field `yaml:"fields"`
}

// FormLayout is the Studio form presentation tree derived from tabs.
type FormLayout struct {
	Tabs []FormTab `json:"tabs"`
}

// FormTab is one form tab.
type FormTab struct {
	Key   string     `json:"key"`
	Label string     `json:"label"`
	Icon  string     `json:"icon,omitempty"`
	Items []FormItem `json:"items"`
}

// FormItem is one ordered layout node inside a tab.
type FormItem struct {
	Kind        string `json:"kind"`
	Name        string `json:"name,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

func isLayoutFieldType(fieldType string) bool {
	switch strings.TrimSpace(fieldType) {
	case layoutFieldTypeColumn, layoutFieldTypeSection:
		return true
	default:
		return false
	}
}

// normalizeFormLayout flattens authored tabs into Fields and Form.
func normalizeFormLayout(entity *Entity) error {
	if entity == nil {
		return nil
	}
	if len(entity.Tabs) == 0 {
		return nil
	}
	if len(entity.Fields) > 0 {
		return fmt.Errorf("entity cannot define both fields and tabs")
	}

	seenFields := map[string]struct{}{}
	seenTabs := map[string]struct{}{}
	form := FormLayout{Tabs: make([]FormTab, 0, len(entity.Tabs))}
	fields := make([]Field, 0)

	for _, tab := range entity.Tabs {
		label := strings.TrimSpace(tab.Label)
		if label == "" {
			return fmt.Errorf("%s", withLine(tab.Line, "tab label is required"))
		}
		key := strings.TrimSpace(tab.Name)
		if key == "" {
			return fmt.Errorf("%s", withLine(tab.Line, fmt.Sprintf("tab %q requires name", label)))
		}
		if !fieldtype.IsName(key) {
			return fmt.Errorf("%s", withLine(tab.Line, fmt.Sprintf("tab name %q must be kebab-case", key)))
		}
		if _, exists := seenTabs[key]; exists {
			return fmt.Errorf("%s", withLine(tab.Line, fmt.Sprintf("duplicate tab name %q", key)))
		}
		seenTabs[key] = struct{}{}

		if len(tab.Fields) == 0 {
			return fmt.Errorf("%s", withLine(tab.Line, fmt.Sprintf("tab %q requires fields", label)))
		}
		items := make([]FormItem, 0, len(tab.Fields))
		for _, field := range tab.Fields {
			if isLayoutFieldType(field.Type) && (field.Required || field.Unique || field.Index || field.Default.Kind != 0 || field.Check != nil || field.Fetch != nil || fieldtype.NoOptions(field.Options) != nil) {
				return fmt.Errorf("%s", withLine(field.Line, "layout markers cannot define storage field settings"))
			}
			switch strings.TrimSpace(field.Type) {
			case layoutFieldTypeColumn:
				items = append(items, FormItem{Kind: FormItemKindColumn})
			case layoutFieldTypeSection:
				sectionLabel := strings.TrimSpace(field.Label)
				if sectionLabel == "" {
					return fmt.Errorf("%s", withLine(field.Line, fmt.Sprintf("tab %q section requires label", label)))
				}
				items = append(items, FormItem{
					Kind:        FormItemKindSection,
					Name:        strings.TrimSpace(field.Name),
					Label:       sectionLabel,
					Description: strings.TrimSpace(field.Description),
				})
			default:
				name := strings.TrimSpace(field.Name)
				if name == "" {
					return fmt.Errorf("%s", withLine(field.Line, fmt.Sprintf("tab %q field is missing name", label)))
				}
				if _, exists := seenFields[name]; exists {
					return fmt.Errorf("%s", withLine(field.Line, fmt.Sprintf("duplicate field %q across tabs", name)))
				}
				seenFields[name] = struct{}{}
				fields = append(fields, field)
				items = append(items, FormItem{Kind: FormItemKindField, Name: name})
			}
		}
		form.Tabs = append(form.Tabs, FormTab{
			Key:   key,
			Label: label,
			Icon:  strings.TrimSpace(tab.Icon),
			Items: items,
		})
	}

	if len(fields) == 0 {
		return fmt.Errorf("tabs must include at least one storage field")
	}
	entity.Fields = fields
	entity.Form = &form
	entity.Tabs = nil
	return nil
}

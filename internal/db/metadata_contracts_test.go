package db

import (
	"strings"
	"testing"
)

func TestValidateRecordMatch(t *testing.T) {
	meta := MetadataEntityMeta{
		MetadataEntity: MetadataEntity{Name: "core.user"},
		Fields: []MetadataField{
			{Name: "email", Type: "email", Unique: true, Stored: true},
			{Name: "status", Type: "select", Stored: true},
			{Name: "role", Type: "link", Stored: true},
			{Name: "contacts", Type: "collection"},
		},
		Constraints: []MetadataConstraint{
			{Name: "user_status_role_key", Type: "unique", Fields: []byte(`["status","role"]`)},
		},
	}
	tests := []struct {
		name  string
		match []string
		want  string
	}{
		{name: "system name", match: []string{"name"}},
		{name: "system id", match: []string{"id"}, want: "does not exist"},
		{name: "system created at", match: []string{"created-at"}, want: "does not exist"},
		{name: "system updated at", match: []string{"updated-at"}, want: "does not exist"},
		{name: "unique field", match: []string{"email"}},
		{name: "unique constraint", match: []string{"role", "status"}},
		{name: "unknown", match: []string{"missing"}, want: "does not exist"},
		{name: "non unique", match: []string{"status"}, want: "not backed by a unique field or constraint"},
		{name: "collection", match: []string{"contacts"}, want: "unsupported collection storage"},
		{name: "duplicate", match: []string{"email", "email"}, want: "duplicate field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRecordMatch(meta, tt.match)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("ValidateRecordMatch() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateRecordMatch() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestRecordAddressableFieldByName(t *testing.T) {
	fields := map[string]MetadataField{
		"email": {Name: "email", Type: "email", Unique: true, Stored: true},
	}

	tests := []struct {
		name string
		want bool
	}{
		{name: "email", want: true},
		{name: "name", want: true},
		{name: "id"},
		{name: "created-at"},
		{name: "updated-at"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := RecordAddressableFieldByName(fields, tt.name)
			if ok != tt.want {
				t.Fatalf("RecordAddressableFieldByName() ok = %t, want %t", ok, tt.want)
			}
			if ok && field.Name != tt.name {
				t.Fatalf("RecordAddressableFieldByName() field = %q, want %q", field.Name, tt.name)
			}
		})
	}
}

func TestLinkFieldTarget(t *testing.T) {
	target, err := LinkFieldTarget(MetadataField{Options: []byte(`{"entity":"user"}`)})
	if err != nil || target != "user" {
		t.Fatalf("LinkFieldTarget() = %q, %v; want user, nil", target, err)
	}
	_, err = LinkFieldTarget(MetadataField{Options: []byte(`{}`)})
	if err == nil || !strings.Contains(err.Error(), "target entity is required") {
		t.Fatalf("LinkFieldTarget() error = %v, want target error", err)
	}
}

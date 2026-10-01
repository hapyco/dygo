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

func TestLinkFieldTargetIdentity(t *testing.T) {
	_, err := LinkFieldTargetIdentity(MetadataField{Options: []byte(`{}`)}, "sales")
	if err == nil || !strings.Contains(err.Error(), "target entity is required") {
		t.Fatalf("LinkFieldTargetIdentity() error = %v, want target error", err)
	}
	targetIdentity, err := LinkFieldTargetIdentity(MetadataField{Options: []byte(`{"app":"core","entity":"user"}`)}, "sales")
	if err != nil || targetIdentity.App != "core" || targetIdentity.Entity != "user" {
		t.Fatalf("LinkFieldTargetIdentity() = %+v, %v; want core/user, nil", targetIdentity, err)
	}
	targetIdentity, err = LinkFieldTargetIdentity(MetadataField{Options: []byte(`{"entity":"company"}`)}, "sales")
	if err != nil || targetIdentity.App != "sales" || targetIdentity.Entity != "company" {
		t.Fatalf("LinkFieldTargetIdentity() = %+v, %v; want sales/company, nil", targetIdentity, err)
	}
}

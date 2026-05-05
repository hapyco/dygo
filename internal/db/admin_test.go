package db

import (
	"strings"
	"testing"
)

func TestParseDatabaseTarget(t *testing.T) {
	target, err := ParseDatabaseTarget("postgres://user:secret@127.0.0.1:5432/dygo_development?sslmode=disable")
	if err != nil {
		t.Fatalf("ParseDatabaseTarget() error = %v, want nil", err)
	}
	if target.Name != "dygo_development" {
		t.Fatalf("target name = %q, want dygo_development", target.Name)
	}
	if strings.Contains(target.MaintenanceURL, "/dygo_development") {
		t.Fatalf("maintenance URL = %q, should not contain target database", target.MaintenanceURL)
	}
	if !strings.Contains(target.MaintenanceURL, "/postgres") {
		t.Fatalf("maintenance URL = %q, want postgres database", target.MaintenanceURL)
	}
}

func TestParseDatabaseTargetFailures(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "empty",
			url:  "",
			want: "database url is required",
		},
		{
			name: "unsupported scheme",
			url:  "mysql://user@localhost:3306/dygo",
			want: "must use postgres",
		},
		{
			name: "missing database name",
			url:  "postgres://user@localhost:5432",
			want: "must include a database name",
		},
		{
			name: "reserved database",
			url:  "postgres://user@localhost:5432/postgres",
			want: "reserved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDatabaseTarget(tt.url)
			if err == nil {
				t.Fatal("ParseDatabaseTarget() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ParseDatabaseTarget() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

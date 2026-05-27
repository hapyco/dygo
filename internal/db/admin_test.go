package db

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func TestManagerExistsReportsDatabaseStatus(t *testing.T) {
	tests := []struct {
		name   string
		exists bool
	}{
		{name: "existing database", exists: true},
		{name: "missing database", exists: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &fakeMaintenanceConn{exists: tt.exists}
			var maintenanceURL string
			restoreConnectMaintenance(t, func(_ context.Context, databaseURL string) (maintenanceConn, error) {
				maintenanceURL = databaseURL
				return conn, nil
			})

			status, err := Manager{}.Exists(context.Background(), "postgres://user:secret@127.0.0.1:5432/dygo_development?sslmode=disable")
			if err != nil {
				t.Fatalf("Exists() error = %v, want nil", err)
			}
			if status.Name != "dygo_development" || status.Exists != tt.exists {
				t.Fatalf("Exists() = %+v, want name dygo_development exists %v", status, tt.exists)
			}
			if strings.Contains(maintenanceURL, "/dygo_development") || !strings.Contains(maintenanceURL, "/postgres") {
				t.Fatalf("maintenance URL = %q, want postgres maintenance database", maintenanceURL)
			}
			if conn.queryName != "dygo_development" {
				t.Fatalf("queried database name = %q, want dygo_development", conn.queryName)
			}
			if !conn.closed {
				t.Fatal("maintenance connection was not closed")
			}
		})
	}
}

func TestManagerExistsRejectsInvalidTargetsBeforeConnecting(t *testing.T) {
	called := false
	restoreConnectMaintenance(t, func(context.Context, string) (maintenanceConn, error) {
		called = true
		return &fakeMaintenanceConn{}, nil
	})

	_, err := Manager{}.Exists(context.Background(), "postgres://user@localhost:5432/postgres")
	if err == nil {
		t.Fatal("Exists() error = nil, want reserved database error")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("Exists() error = %q, want reserved database", err.Error())
	}
	if called {
		t.Fatal("Exists() connected before rejecting reserved database")
	}
}

func restoreConnectMaintenance(t *testing.T, fn func(context.Context, string) (maintenanceConn, error)) {
	t.Helper()
	old := connectMaintenance
	connectMaintenance = fn
	t.Cleanup(func() {
		connectMaintenance = old
	})
}

type fakeMaintenanceConn struct {
	exists    bool
	queryName string
	closed    bool
}

func (c *fakeMaintenanceConn) QueryRow(_ context.Context, _ string, args ...any) pgx.Row {
	if len(args) > 0 {
		if name, ok := args[0].(string); ok {
			c.queryName = name
		}
	}
	return fakeBoolRow{value: c.exists}
}

func (c *fakeMaintenanceConn) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (c *fakeMaintenanceConn) Close(context.Context) error {
	c.closed = true
	return nil
}

type fakeBoolRow struct {
	value bool
}

func (r fakeBoolRow) Scan(dest ...any) error {
	if len(dest) > 0 {
		if target, ok := dest[0].(*bool); ok {
			*target = r.value
		}
	}
	return nil
}

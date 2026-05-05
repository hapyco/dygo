package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiscoverMigrations(t *testing.T) {
	root := t.TempDir()
	writeMigrationPair(t, root, "20260505190000_create_project_table")

	migrations, err := DiscoverMigrations(root)
	if err != nil {
		t.Fatalf("DiscoverMigrations() error = %v, want nil", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("DiscoverMigrations() len = %d, want 2", len(migrations))
	}
	if migrations[0].Scope != MigrationScopeFramework || migrations[0].Name != "create_core_tables" {
		t.Fatalf("first migration = %#v, want framework core migration", migrations[0])
	}
	if migrations[1].Scope != MigrationScopeProject || migrations[1].Name != "create_project_table" {
		t.Fatalf("second migration = %#v, want project migration", migrations[1])
	}
	if migrations[0].UpChecksum == "" || migrations[0].DownChecksum == "" {
		t.Fatal("framework migration checksums are empty")
	}
}

func TestDiscoverMigrationsAllowsMissingProjectDirectory(t *testing.T) {
	migrations, err := DiscoverMigrations(t.TempDir())
	if err != nil {
		t.Fatalf("DiscoverMigrations() error = %v, want nil", err)
	}
	if len(migrations) != 1 || migrations[0].Scope != MigrationScopeFramework {
		t.Fatalf("DiscoverMigrations() = %#v, want only framework migration", migrations)
	}
}

func TestDiscoverMigrationsRejectsInvalidProjectFiles(t *testing.T) {
	tests := []struct {
		name string
		file string
		body string
		want string
	}{
		{
			name: "malformed name",
			file: "bad.sql",
			body: "SELECT 1;",
			want: "invalid project migration filename",
		},
		{
			name: "missing pair",
			file: "20260505190000_missing_down.up.sql",
			body: "SELECT 1;",
			want: "must have paired up and down files",
		},
		{
			name: "mismatched names",
			file: "20260505190000_name_one.up.sql",
			body: "SELECT 1;",
			want: "mismatched names",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeProjectMigrationFile(t, root, tt.file, tt.body)
			if tt.name == "mismatched names" {
				writeProjectMigrationFile(t, root, "20260505190000_name_two.down.sql", "SELECT 1;")
			}

			_, err := DiscoverMigrations(root)
			if err == nil {
				t.Fatal("DiscoverMigrations() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("DiscoverMigrations() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestBuildMigrationStatus(t *testing.T) {
	migration := testMigration("framework", "20260505180000", "create_core_tables")
	appliedAt := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

	statuses, err := BuildMigrationStatus([]Migration{migration}, []AppliedMigration{{
		Scope:        migration.Scope,
		Version:      migration.Version,
		Name:         migration.Name,
		UpChecksum:   migration.UpChecksum,
		DownChecksum: migration.DownChecksum,
		AppliedAt:    appliedAt,
	}})
	if err != nil {
		t.Fatalf("BuildMigrationStatus() error = %v, want nil", err)
	}
	if len(statuses) != 1 || !statuses[0].Applied || !statuses[0].AppliedAt.Equal(appliedAt) {
		t.Fatalf("BuildMigrationStatus() = %#v, want one applied status", statuses)
	}
}

func TestBuildMigrationStatusRejectsChecksumDrift(t *testing.T) {
	migration := testMigration("framework", "20260505180000", "create_core_tables")

	_, err := BuildMigrationStatus([]Migration{migration}, []AppliedMigration{{
		Scope:        migration.Scope,
		Version:      migration.Version,
		Name:         migration.Name,
		UpChecksum:   "changed",
		DownChecksum: migration.DownChecksum,
	}})
	if err == nil {
		t.Fatal("BuildMigrationStatus() error = nil, want checksum error")
	}
	if !strings.Contains(err.Error(), "checksum changed") {
		t.Fatalf("BuildMigrationStatus() error = %q, want checksum context", err.Error())
	}
}

func TestApplyPendingMigrations(t *testing.T) {
	ctx := context.Background()
	first := testMigration("framework", "20260505180000", "create_core_tables")
	second := testMigration("project", "20260505190000", "create_project_table")
	store := &fakeMigrationStore{
		applied: []AppliedMigration{appliedFor(first)},
	}

	result, err := ApplyPendingMigrations(ctx, []Migration{first, second}, store)
	if err != nil {
		t.Fatalf("ApplyPendingMigrations() error = %v, want nil", err)
	}
	if len(result.Applied) != 1 || result.Applied[0].Name != second.Name {
		t.Fatalf("ApplyPendingMigrations() result = %#v, want second migration applied", result)
	}
	if len(store.appliedCalls) != 1 || store.appliedCalls[0].Name != second.Name {
		t.Fatalf("fake store applied calls = %#v, want second migration", store.appliedCalls)
	}
}

func TestRollbackMigrations(t *testing.T) {
	ctx := context.Background()
	first := testMigration("framework", "20260505180000", "create_core_tables")
	second := testMigration("project", "20260505190000", "create_project_table")
	store := &fakeMigrationStore{
		applied: []AppliedMigration{appliedFor(first), appliedFor(second)},
	}

	result, err := RollbackMigrations(ctx, []Migration{first, second}, store, 1)
	if err != nil {
		t.Fatalf("RollbackMigrations() error = %v, want nil", err)
	}
	if len(result.RolledBack) != 1 || result.RolledBack[0].Name != second.Name {
		t.Fatalf("RollbackMigrations() result = %#v, want latest migration rolled back", result)
	}
	if len(store.rolledBackCalls) != 1 || store.rolledBackCalls[0].Name != second.Name {
		t.Fatalf("fake store rolled back calls = %#v, want second migration", store.rolledBackCalls)
	}
}

func TestRollbackMigrationsRejectsInvalidSteps(t *testing.T) {
	_, err := RollbackMigrations(context.Background(), nil, &fakeMigrationStore{}, 0)
	if err == nil {
		t.Fatal("RollbackMigrations() error = nil, want steps error")
	}
}

func TestPGDumpConnectionRemovesPassword(t *testing.T) {
	got, password := pgDumpConnection("postgres://user:secret@127.0.0.1:5432/dygo?sslmode=disable")
	if password != "secret" {
		t.Fatalf("password = %q, want secret", password)
	}
	if strings.Contains(got, "secret") {
		t.Fatalf("connection string %q leaked password", got)
	}
	if !strings.Contains(got, "user@127.0.0.1") {
		t.Fatalf("connection string = %q, want user without password", got)
	}
}

type fakeMigrationStore struct {
	applied         []AppliedMigration
	appliedCalls    []Migration
	rolledBackCalls []Migration
}

func (s *fakeMigrationStore) Applied(context.Context) ([]AppliedMigration, error) {
	return append([]AppliedMigration(nil), s.applied...), nil
}

func (s *fakeMigrationStore) Apply(_ context.Context, migration Migration) error {
	s.appliedCalls = append(s.appliedCalls, migration)
	return nil
}

func (s *fakeMigrationStore) Rollback(_ context.Context, migration Migration) error {
	s.rolledBackCalls = append(s.rolledBackCalls, migration)
	return nil
}

func testMigration(scope string, version string, name string) Migration {
	upSQL := "CREATE TABLE " + name + " (id bigint);"
	downSQL := "DROP TABLE " + name + ";"
	return Migration{
		Scope:        scope,
		Version:      version,
		Name:         name,
		UpPath:       version + "_" + name + ".up.sql",
		DownPath:     version + "_" + name + ".down.sql",
		UpSQL:        upSQL,
		DownSQL:      downSQL,
		UpChecksum:   checksum([]byte(upSQL)),
		DownChecksum: checksum([]byte(downSQL)),
	}
}

func appliedFor(migration Migration) AppliedMigration {
	return AppliedMigration{
		Scope:        migration.Scope,
		Version:      migration.Version,
		Name:         migration.Name,
		UpChecksum:   migration.UpChecksum,
		DownChecksum: migration.DownChecksum,
		AppliedAt:    time.Now().UTC(),
	}
}

func writeMigrationPair(t *testing.T, root string, name string) {
	t.Helper()

	writeProjectMigrationFile(t, root, name+".up.sql", "CREATE TABLE project_table (id bigint);")
	writeProjectMigrationFile(t, root, name+".down.sql", "DROP TABLE project_table;")
}

func writeProjectMigrationFile(t *testing.T, root string, name string, body string) {
	t.Helper()

	path := filepath.Join(root, projectMigrationsDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

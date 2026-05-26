package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestMigratorCheckSchemaSnapshotCurrent(t *testing.T) {
	root := t.TempDir()
	writeTestSchemaSnapshot(t, root, "current schema\n")
	snapshotter := &fakeSchemaCheckSnapshotter{content: "current schema\n"}

	err := (Migrator{Snapshotter: snapshotter}).CheckSchemaSnapshot(context.Background(), root, "postgres://user:secret@localhost/dygo")
	if err != nil {
		t.Fatalf("CheckSchemaSnapshot() error = %v, want nil", err)
	}
	if snapshotter.calls != 1 {
		t.Fatalf("snapshotter calls = %d, want 1", snapshotter.calls)
	}
	if snapshotter.root == root {
		t.Fatalf("snapshotter root = repo root %q, want temporary root", root)
	}
	if got := readTestSchemaSnapshot(t, root); got != "current schema\n" {
		t.Fatalf("repo schema snapshot = %q, want unchanged current schema", got)
	}
}

func TestMigratorCheckSchemaSnapshotMissing(t *testing.T) {
	root := t.TempDir()
	snapshotter := &fakeSchemaCheckSnapshotter{content: "current schema\n"}

	err := (Migrator{Snapshotter: snapshotter}).CheckSchemaSnapshot(context.Background(), root, "postgres://user:secret@localhost/dygo")
	if !errors.Is(err, ErrSchemaSnapshotMissing) {
		t.Fatalf("CheckSchemaSnapshot() error = %v, want ErrSchemaSnapshotMissing", err)
	}
	if !strings.Contains(err.Error(), "schema snapshot is missing: db/schema.sql; run dygo doctor") {
		t.Fatalf("CheckSchemaSnapshot() error = %q, want missing snapshot command", err.Error())
	}
	if snapshotter.calls != 0 {
		t.Fatalf("snapshotter calls = %d, want 0", snapshotter.calls)
	}
}

func TestMigratorCheckSchemaSnapshotOutOfDate(t *testing.T) {
	root := t.TempDir()
	writeTestSchemaSnapshot(t, root, "old schema\n")
	snapshotter := &fakeSchemaCheckSnapshotter{content: "new schema\n"}

	err := (Migrator{Snapshotter: snapshotter}).CheckSchemaSnapshot(context.Background(), root, "postgres://user:secret@localhost/dygo")
	if !errors.Is(err, ErrSchemaSnapshotOutOfDate) {
		t.Fatalf("CheckSchemaSnapshot() error = %v, want ErrSchemaSnapshotOutOfDate", err)
	}
	if !strings.Contains(err.Error(), "schema snapshot is out of date: db/schema.sql; run dygo doctor") {
		t.Fatalf("CheckSchemaSnapshot() error = %q, want out-of-date command", err.Error())
	}
	if got := readTestSchemaSnapshot(t, root); got != "old schema\n" {
		t.Fatalf("repo schema snapshot = %q, want unchanged old schema", got)
	}
}

func TestMigratorCheckSchemaSnapshotRedactsSnapshotterError(t *testing.T) {
	root := t.TempDir()
	writeTestSchemaSnapshot(t, root, "current schema\n")
	databaseURL := "postgres://user:secret@localhost/dygo"
	snapshotter := &fakeSchemaCheckSnapshotter{err: fmt.Errorf("pg_dump failed for %s", databaseURL)}

	err := (Migrator{Snapshotter: snapshotter}).CheckSchemaSnapshot(context.Background(), root, databaseURL)
	if err == nil {
		t.Fatal("CheckSchemaSnapshot() error = nil, want snapshotter error")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), databaseURL) {
		t.Fatalf("CheckSchemaSnapshot() error = %q, want redacted database URL", err.Error())
	}
}

type fakeSchemaCheckSnapshotter struct {
	content     string
	err         error
	calls       int
	root        string
	databaseURL string
}

func (s *fakeSchemaCheckSnapshotter) Dump(_ context.Context, root string, databaseURL string) error {
	s.calls++
	s.root = root
	s.databaseURL = databaseURL
	if s.err != nil {
		return s.err
	}
	path := filepath.Join(root, filepath.FromSlash(SchemaPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create schema dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(s.content), 0o644); err != nil {
		return fmt.Errorf("write schema snapshot: %w", err)
	}
	return nil
}

func writeTestSchemaSnapshot(t interface {
	Helper()
	Fatalf(string, ...any)
}, root string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(SchemaPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(schema dir) error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(schema snapshot) error = %v", err)
	}
}

func readTestSchemaSnapshot(t *testing.T, root string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(SchemaPath)))
	if err != nil {
		t.Fatalf("ReadFile(schema snapshot) error = %v", err)
	}
	return string(content)
}

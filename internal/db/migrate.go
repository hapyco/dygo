package db

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const SchemaPath = "db/schema.sql"

var (
	// ErrSchemaSnapshotMissing reports that db/schema.sql has not been generated yet.
	ErrSchemaSnapshotMissing = errors.New("schema snapshot is missing")
	// ErrSchemaSnapshotOutOfDate reports that db/schema.sql does not match the live schema dump.
	ErrSchemaSnapshotOutOfDate = errors.New("schema snapshot is out of date")
)

// SchemaSyncResult reports the metadata schema synced by an operation.
type SchemaSyncResult struct {
	Apps       int
	Entities   int
	Fields     int
	Operations int
}

// Migrator syncs dygo metadata into PostgreSQL and writes schema snapshots.
type Migrator struct {
	Snapshotter Snapshotter
}

// Snapshotter writes a schema snapshot for a database.
type Snapshotter interface {
	Dump(ctx context.Context, root string, databaseURL string) error
}

// NewMigrator returns the default dygo migrator.
func NewMigrator() Migrator {
	return Migrator{Snapshotter: PgdumpSnapshotter{}}
}

// Sync applies app Entity metadata to PostgreSQL and regenerates the schema snapshot.
func (m Migrator) Sync(ctx context.Context, root string, databaseURL string) (SchemaSyncResult, error) {
	pool, err := connectMetadataPool(ctx, databaseURL)
	if err != nil {
		return SchemaSyncResult{}, err
	}
	defer pool.Close()

	result, err := SyncMetadataSchema(ctx, pool, root)
	if err != nil {
		return SchemaSyncResult{}, fmt.Errorf("sync metadata schema: %w", err)
	}
	if err := m.dumpSchema(ctx, root, databaseURL); err != nil {
		return SchemaSyncResult{}, err
	}
	return result, nil
}

// Plan compares app Entity metadata with the live database without applying changes.
func (m Migrator) Plan(ctx context.Context, root string, databaseURL string) (SchemaPlan, error) {
	pool, err := connectMetadataPool(ctx, databaseURL)
	if err != nil {
		return SchemaPlan{}, err
	}
	defer pool.Close()

	plan, err := PlanMetadataSchema(ctx, pool, root)
	if err != nil {
		return SchemaPlan{}, fmt.Errorf("plan metadata schema: %w", err)
	}
	return plan, nil
}

// PrunePlan compares app Entity metadata with the live database for explicit destructive cleanup.
func (m Migrator) PrunePlan(ctx context.Context, root string, databaseURL string) (SchemaPrunePlan, error) {
	pool, err := connectMetadataPool(ctx, databaseURL)
	if err != nil {
		return SchemaPrunePlan{}, err
	}
	defer pool.Close()

	plan, err := PlanSchemaPrune(ctx, pool, root)
	if err != nil {
		return SchemaPrunePlan{}, fmt.Errorf("plan schema prune: %w", err)
	}
	return plan, nil
}

// Prune applies explicit destructive cleanup and regenerates the schema snapshot when changed.
func (m Migrator) Prune(ctx context.Context, root string, databaseURL string) (SchemaPruneResult, error) {
	pool, err := connectMetadataPool(ctx, databaseURL)
	if err != nil {
		return SchemaPruneResult{}, err
	}
	defer pool.Close()

	result, err := PruneMetadataSchema(ctx, pool, root)
	if err != nil {
		return SchemaPruneResult{}, fmt.Errorf("prune schema: %w", err)
	}
	if result.Operations == 0 {
		return result, nil
	}
	if err := m.dumpSchema(ctx, root, databaseURL); err != nil {
		return SchemaPruneResult{}, err
	}
	return result, nil
}

// DumpSchema writes db/schema.sql using the configured snapshotter.
func (m Migrator) DumpSchema(ctx context.Context, root string, databaseURL string) error {
	return m.dumpSchema(ctx, root, databaseURL)
}

// CheckSchemaSnapshot verifies db/schema.sql matches a fresh live schema dump.
func (m Migrator) CheckSchemaSnapshot(ctx context.Context, root string, databaseURL string) error {
	schemaPath := filepath.Join(root, filepath.FromSlash(SchemaPath))
	current, err := os.ReadFile(schemaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s; run dygo db schema dump", ErrSchemaSnapshotMissing, SchemaPath)
		}
		return fmt.Errorf("read schema snapshot: %w", err)
	}

	tmpRoot, err := os.MkdirTemp("", "dygo-schema-check-*")
	if err != nil {
		return fmt.Errorf("create temporary schema check root: %w", err)
	}
	defer os.RemoveAll(tmpRoot)

	if err := m.dumpSchema(ctx, tmpRoot, databaseURL); err != nil {
		return sanitizeError("dump schema for snapshot check", databaseURL, err)
	}
	fresh, err := os.ReadFile(filepath.Join(tmpRoot, filepath.FromSlash(SchemaPath)))
	if err != nil {
		return fmt.Errorf("read generated schema snapshot: %w", err)
	}
	if !bytes.Equal(current, fresh) {
		return fmt.Errorf("%w: %s; run dygo db schema dump", ErrSchemaSnapshotOutOfDate, SchemaPath)
	}
	return nil
}

func (m Migrator) dumpSchema(ctx context.Context, root string, databaseURL string) error {
	if m.Snapshotter == nil {
		return nil
	}
	return m.Snapshotter.Dump(ctx, root, databaseURL)
}

// PgdumpSnapshotter writes db/schema.sql using pg_dump.
type PgdumpSnapshotter struct{}

// Dump writes db/schema.sql from the live database schema.
func (s PgdumpSnapshotter) Dump(ctx context.Context, root string, databaseURL string) error {
	pgDump, err := findPGDump()
	if err != nil {
		return err
	}
	schemaPath := filepath.Join(root, filepath.FromSlash(SchemaPath))
	if err := os.MkdirAll(filepath.Dir(schemaPath), 0o755); err != nil {
		return fmt.Errorf("create schema directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(schemaPath), ".schema.*.sql")
	if err != nil {
		return fmt.Errorf("create temporary schema file: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temporary schema file: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmpPath)
		}
	}()

	connectionString, password := pgDumpConnection(databaseURL)
	output, err := runPGDump(ctx, pgDump, tmpPath, connectionString, password, true)
	if err != nil && pgDumpRestrictKeyUnsupported(output) {
		output, err = runPGDump(ctx, pgDump, tmpPath, connectionString, password, false)
	}
	if err != nil {
		return fmt.Errorf("dump schema with pg_dump: %w: %s", err, strings.TrimSpace(string(output)))
	}
	dumped, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("read schema snapshot: %w", err)
	}
	if err := os.WriteFile(tmpPath, normalizeSchemaSnapshot(dumped), 0o644); err != nil {
		return fmt.Errorf("normalize schema snapshot: %w", err)
	}
	if err := os.Rename(tmpPath, schemaPath); err != nil {
		return fmt.Errorf("write schema snapshot: %w", err)
	}
	cleanup = false
	return nil
}

func normalizeSchemaSnapshot(data []byte) []byte {
	trimmed := bytes.TrimRight(data, "\n")
	if len(trimmed) == 0 {
		return []byte{}
	}
	return append(trimmed, '\n')
}

func runPGDump(ctx context.Context, pgDump string, tmpPath string, connectionString string, password string, useRestrictKey bool) ([]byte, error) {
	args := []string{"--schema-only", "--no-owner", "--no-privileges"}
	if useRestrictKey {
		args = append(args, "--restrict-key", "dygoschemasnapshot")
	}
	args = append(args, "--file", tmpPath, connectionString)

	cmd := exec.CommandContext(ctx, pgDump, args...)
	cmd.Env = os.Environ()
	if password != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+password)
	}
	return cmd.CombinedOutput()
}

func pgDumpRestrictKeyUnsupported(output []byte) bool {
	text := string(output)
	return strings.Contains(text, "unrecognized option") && strings.Contains(text, "restrict-key")
}

func connectMetadataPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, fmt.Errorf("database url is required")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid postgres database url")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, sanitizeError("connect to postgres", databaseURL, err)
	}
	return pool, nil
}

// OpenRuntimePool opens and pings the PostgreSQL pool used by runtime services.
func OpenRuntimePool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	connectCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()

	pool, err := connectMetadataPool(connectCtx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, sanitizeError("ping postgres", databaseURL, err)
	}
	return pool, nil
}

func findPGDump() (string, error) {
	if path, err := exec.LookPath("pg_dump"); err == nil {
		return path, nil
	}
	const postgresAppPGDump = "/Applications/Postgres.app/Contents/Versions/latest/bin/pg_dump"
	if info, err := os.Stat(postgresAppPGDump); err == nil && !info.IsDir() {
		return postgresAppPGDump, nil
	}
	return "", fmt.Errorf("pg_dump not found in PATH or Postgres.app")
}

func pgDumpConnection(databaseURL string) (string, string) {
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.User == nil {
		return databaseURL, ""
	}
	password, ok := parsed.User.Password()
	if !ok || password == "" {
		return databaseURL, ""
	}
	username := parsed.User.Username()
	parsed.User = url.User(username)
	return parsed.String(), password
}

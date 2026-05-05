package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MigrationScopeFramework = "framework"
	MigrationScopeProject   = "project"

	projectMigrationsDir = "db/migrations"
	SchemaPath           = "db/schema.sql"

	migrationTableName       = "migrations"
	legacyMigrationTableName = "dygo_migrations"
)

var migrationFilenamePattern = regexp.MustCompile(`^([0-9]{14})_([a-z0-9][a-z0-9_-]*)\.(up|down)\.sql$`)

//go:embed migrations/*.sql
var frameworkMigrationFS embed.FS

// Migration is one paired up/down SQL migration.
type Migration struct {
	Scope        string
	Version      string
	Name         string
	UpPath       string
	DownPath     string
	UpSQL        string
	DownSQL      string
	UpChecksum   string
	DownChecksum string
}

// AppliedMigration is a row from the migrations tracking table.
type AppliedMigration struct {
	Scope        string
	Version      string
	Name         string
	UpChecksum   string
	DownChecksum string
	AppliedAt    time.Time
}

// MigrationStatus reports whether a discovered migration is applied.
type MigrationStatus struct {
	Migration
	Applied   bool
	AppliedAt time.Time
}

// MigrationResult reports migrations changed by an operation.
type MigrationResult struct {
	Applied    []Migration
	RolledBack []Migration
}

// Migrator runs dygo database migrations and schema snapshots.
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

// DiscoverMigrations loads framework and project migrations for root.
func DiscoverMigrations(root string) ([]Migration, error) {
	framework, err := discoverMigrationFS(frameworkMigrationFS, "migrations", MigrationScopeFramework, true)
	if err != nil {
		return nil, err
	}

	projectRoot := os.DirFS(root)
	project, err := discoverMigrationFS(projectRoot, filepath.ToSlash(projectMigrationsDir), MigrationScopeProject, false)
	if err != nil {
		return nil, err
	}

	migrations := append(framework, project...)
	sortMigrations(migrations)
	return migrations, nil
}

// Status returns discovered migrations with applied state.
func (m Migrator) Status(ctx context.Context, root string, databaseURL string) ([]MigrationStatus, error) {
	migrations, err := DiscoverMigrations(root)
	if err != nil {
		return nil, err
	}
	pool, err := connectMigrationPool(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	defer pool.Close()

	store := postgresMigrationStore{pool: pool}
	applied, err := store.Applied(ctx)
	if err != nil {
		return nil, err
	}
	return BuildMigrationStatus(migrations, applied)
}

// Up applies all pending migrations and regenerates the schema snapshot.
func (m Migrator) Up(ctx context.Context, root string, databaseURL string) (MigrationResult, error) {
	migrations, err := DiscoverMigrations(root)
	if err != nil {
		return MigrationResult{}, err
	}
	pool, err := connectMigrationPool(ctx, databaseURL)
	if err != nil {
		return MigrationResult{}, err
	}
	defer pool.Close()

	store := postgresMigrationStore{pool: pool}
	if err := store.Ensure(ctx); err != nil {
		return MigrationResult{}, err
	}
	result, err := ApplyPendingMigrations(ctx, migrations, store)
	if err != nil {
		return MigrationResult{}, err
	}
	if err := m.dumpSchema(ctx, root, databaseURL); err != nil {
		return MigrationResult{}, err
	}
	return result, nil
}

// Down rolls back the most recent applied migrations and regenerates the schema snapshot.
func (m Migrator) Down(ctx context.Context, root string, databaseURL string, steps int) (MigrationResult, error) {
	migrations, err := DiscoverMigrations(root)
	if err != nil {
		return MigrationResult{}, err
	}
	pool, err := connectMigrationPool(ctx, databaseURL)
	if err != nil {
		return MigrationResult{}, err
	}
	defer pool.Close()

	store := postgresMigrationStore{pool: pool}
	if err := store.Ensure(ctx); err != nil {
		return MigrationResult{}, err
	}
	result, err := RollbackMigrations(ctx, migrations, store, steps)
	if err != nil {
		return MigrationResult{}, err
	}
	if err := m.dumpSchema(ctx, root, databaseURL); err != nil {
		return MigrationResult{}, err
	}
	return result, nil
}

// Redo rolls back applied migrations and applies pending migrations again.
func (m Migrator) Redo(ctx context.Context, root string, databaseURL string, steps int) (MigrationResult, error) {
	if steps < 1 {
		return MigrationResult{}, fmt.Errorf("steps must be at least 1")
	}
	down, err := m.Down(ctx, root, databaseURL, steps)
	if err != nil {
		return MigrationResult{}, err
	}
	up, err := m.Up(ctx, root, databaseURL)
	if err != nil {
		return MigrationResult{}, err
	}
	return MigrationResult{
		Applied:    up.Applied,
		RolledBack: down.RolledBack,
	}, nil
}

// DumpSchema writes db/schema.sql using the configured snapshotter.
func (m Migrator) DumpSchema(ctx context.Context, root string, databaseURL string) error {
	return m.dumpSchema(ctx, root, databaseURL)
}

func (m Migrator) dumpSchema(ctx context.Context, root string, databaseURL string) error {
	if m.Snapshotter == nil {
		return nil
	}
	return m.Snapshotter.Dump(ctx, root, databaseURL)
}

type migrationStore interface {
	Applied(context.Context) ([]AppliedMigration, error)
	Apply(context.Context, Migration) error
	Rollback(context.Context, Migration) error
}

// BuildMigrationStatus joins discovered and applied migrations.
func BuildMigrationStatus(migrations []Migration, applied []AppliedMigration) ([]MigrationStatus, error) {
	migrationByKey := map[string]Migration{}
	for _, migration := range migrations {
		migrationByKey[migrationKey(migration.Scope, migration.Version)] = migration
	}

	appliedByKey := map[string]AppliedMigration{}
	for _, appliedMigration := range applied {
		key := migrationKey(appliedMigration.Scope, appliedMigration.Version)
		migration, ok := migrationByKey[key]
		if !ok {
			return nil, fmt.Errorf("applied migration %s/%s is not present on disk", appliedMigration.Scope, appliedMigration.Version)
		}
		if err := validateAppliedChecksum(migration, appliedMigration); err != nil {
			return nil, err
		}
		appliedByKey[key] = appliedMigration
	}

	statuses := make([]MigrationStatus, 0, len(migrations))
	for _, migration := range migrations {
		status := MigrationStatus{Migration: migration}
		if appliedMigration, ok := appliedByKey[migrationKey(migration.Scope, migration.Version)]; ok {
			status.Applied = true
			status.AppliedAt = appliedMigration.AppliedAt
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// ApplyPendingMigrations applies all migrations not present in the store.
func ApplyPendingMigrations(ctx context.Context, migrations []Migration, store migrationStore) (MigrationResult, error) {
	applied, err := store.Applied(ctx)
	if err != nil {
		return MigrationResult{}, err
	}
	statuses, err := BuildMigrationStatus(migrations, applied)
	if err != nil {
		return MigrationResult{}, err
	}

	var result MigrationResult
	for _, status := range statuses {
		if status.Applied {
			continue
		}
		if err := store.Apply(ctx, status.Migration); err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, status.Migration)
	}
	return result, nil
}

// RollbackMigrations rolls back applied migrations in reverse order.
func RollbackMigrations(ctx context.Context, migrations []Migration, store migrationStore, steps int) (MigrationResult, error) {
	if steps < 1 {
		return MigrationResult{}, fmt.Errorf("steps must be at least 1")
	}
	applied, err := store.Applied(ctx)
	if err != nil {
		return MigrationResult{}, err
	}
	statuses, err := BuildMigrationStatus(migrations, applied)
	if err != nil {
		return MigrationResult{}, err
	}

	var result MigrationResult
	for i := len(statuses) - 1; i >= 0 && len(result.RolledBack) < steps; i-- {
		status := statuses[i]
		if !status.Applied {
			continue
		}
		if err := store.Rollback(ctx, status.Migration); err != nil {
			return result, err
		}
		result.RolledBack = append(result.RolledBack, status.Migration)
	}
	return result, nil
}

type postgresMigrationStore struct {
	pool *pgxpool.Pool
}

func (s postgresMigrationStore) Ensure(ctx context.Context) error {
	if err := s.renameLegacyTable(ctx); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS migrations (
	id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	scope text NOT NULL,
	version text NOT NULL,
	name text NOT NULL,
	up_checksum text NOT NULL,
	down_checksum text NOT NULL,
	applied_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (scope, version)
)`)
	return err
}

func (s postgresMigrationStore) Applied(ctx context.Context) ([]AppliedMigration, error) {
	if err := s.renameLegacyTable(ctx); err != nil {
		return nil, err
	}
	tableExists, err := s.tableExists(ctx, migrationTableName)
	if err != nil {
		return nil, err
	}
	if !tableExists {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `
SELECT scope, version, name, up_checksum, down_checksum, applied_at
FROM migrations
ORDER BY scope, version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applied []AppliedMigration
	for rows.Next() {
		var migration AppliedMigration
		if err := rows.Scan(&migration.Scope, &migration.Version, &migration.Name, &migration.UpChecksum, &migration.DownChecksum, &migration.AppliedAt); err != nil {
			return nil, err
		}
		applied = append(applied, migration)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return applied, nil
}

func (s postgresMigrationStore) Apply(ctx context.Context, migration Migration) error {
	return s.inTransaction(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, migration.UpSQL); err != nil {
			return fmt.Errorf("apply migration %s/%s: %w", migration.Scope, migration.Version, err)
		}
		_, err := tx.Exec(ctx, `
INSERT INTO migrations (scope, version, name, up_checksum, down_checksum)
VALUES ($1, $2, $3, $4, $5)`,
			migration.Scope,
			migration.Version,
			migration.Name,
			migration.UpChecksum,
			migration.DownChecksum,
		)
		if err != nil {
			return fmt.Errorf("record migration %s/%s: %w", migration.Scope, migration.Version, err)
		}
		return nil
	})
}

func (s postgresMigrationStore) Rollback(ctx context.Context, migration Migration) error {
	return s.inTransaction(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, migration.DownSQL); err != nil {
			return fmt.Errorf("roll back migration %s/%s: %w", migration.Scope, migration.Version, err)
		}
		tag, err := tx.Exec(ctx, "DELETE FROM migrations WHERE scope = $1 AND version = $2", migration.Scope, migration.Version)
		if err != nil {
			return fmt.Errorf("delete migration record %s/%s: %w", migration.Scope, migration.Version, err)
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("delete migration record %s/%s: expected 1 row, got %d", migration.Scope, migration.Version, tag.RowsAffected())
		}
		return nil
	})
}

func (s postgresMigrationStore) renameLegacyTable(ctx context.Context) error {
	legacyExists, err := s.tableExists(ctx, legacyMigrationTableName)
	if err != nil {
		return err
	}
	currentExists, err := s.tableExists(ctx, migrationTableName)
	if err != nil {
		return err
	}
	if legacyExists && !currentExists {
		if _, err := s.pool.Exec(ctx, fmt.Sprintf("ALTER TABLE public.%s RENAME TO %s", legacyMigrationTableName, migrationTableName)); err != nil {
			return fmt.Errorf("rename legacy migration table: %w", err)
		}
		currentExists = true
	}
	if currentExists {
		if _, err := s.pool.Exec(ctx, `
DO $$
DECLARE
	constraint_record record;
BEGIN
	IF to_regclass('public.dygo_migrations_id_seq') IS NOT NULL
		AND to_regclass('public.migrations_id_seq') IS NULL
	THEN
		ALTER SEQUENCE public.dygo_migrations_id_seq RENAME TO migrations_id_seq;
	END IF;

	IF EXISTS (
		SELECT 1 FROM pg_constraint
		WHERE conrelid = 'public.migrations'::regclass
			AND conname = 'dygo_migrations_pkey'
	) THEN
		ALTER TABLE public.migrations RENAME CONSTRAINT dygo_migrations_pkey TO migrations_pkey;
	END IF;
	IF EXISTS (
		SELECT 1 FROM pg_constraint
		WHERE conrelid = 'public.migrations'::regclass
			AND conname = 'dygo_migrations_scope_version_key'
	) THEN
		ALTER TABLE public.migrations RENAME CONSTRAINT dygo_migrations_scope_version_key TO migrations_scope_version_key;
	END IF;

	FOR constraint_record IN
		SELECT conname
		FROM pg_constraint
		WHERE conrelid = 'public.migrations'::regclass
			AND conname LIKE 'dygo_migrations_%'
	LOOP
		EXECUTE format(
			'ALTER TABLE public.migrations RENAME CONSTRAINT %I TO %I',
			constraint_record.conname,
			replace(constraint_record.conname, 'dygo_migrations_', 'migrations_')
		);
	END LOOP;
END $$`); err != nil {
			return fmt.Errorf("rename legacy migration constraints: %w", err)
		}
	}
	return nil
}

func (s postgresMigrationStore) tableExists(ctx context.Context, name string) (bool, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", "public."+name).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s postgresMigrationStore) inTransaction(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
	if err := os.Rename(tmpPath, schemaPath); err != nil {
		return fmt.Errorf("write schema snapshot: %w", err)
	}
	cleanup = false
	return nil
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

func discoverMigrationFS(fsys fs.FS, dir string, scope string, required bool) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if !required && errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s migrations: %w", scope, err)
	}

	pairs := map[string]*migrationPair{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if filepath.Ext(filename) != ".sql" {
			continue
		}
		matches := migrationFilenamePattern.FindStringSubmatch(filename)
		if matches == nil {
			return nil, fmt.Errorf("invalid %s migration filename %q", scope, filename)
		}
		version, name, direction := matches[1], matches[2], matches[3]
		pair := pairs[version]
		if pair == nil {
			pair = &migrationPair{Version: version, Name: name}
			pairs[version] = pair
		}
		if pair.Name != name {
			return nil, fmt.Errorf("migration version %s has mismatched names %q and %q", version, pair.Name, name)
		}
		path := filepath.ToSlash(filepath.Join(dir, filename))
		sql, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("read %s migration %s: %w", scope, filename, err)
		}
		switch direction {
		case "up":
			if pair.UpPath != "" {
				return nil, fmt.Errorf("duplicate up migration for %s/%s", scope, version)
			}
			pair.UpPath = path
			pair.UpSQL = string(sql)
			pair.UpChecksum = checksum(sql)
		case "down":
			if pair.DownPath != "" {
				return nil, fmt.Errorf("duplicate down migration for %s/%s", scope, version)
			}
			pair.DownPath = path
			pair.DownSQL = string(sql)
			pair.DownChecksum = checksum(sql)
		}
	}

	migrations := make([]Migration, 0, len(pairs))
	for _, pair := range pairs {
		if pair.UpPath == "" || pair.DownPath == "" {
			return nil, fmt.Errorf("migration %s/%s must have paired up and down files", scope, pair.Version)
		}
		migrations = append(migrations, Migration{
			Scope:        scope,
			Version:      pair.Version,
			Name:         pair.Name,
			UpPath:       pair.UpPath,
			DownPath:     pair.DownPath,
			UpSQL:        pair.UpSQL,
			DownSQL:      pair.DownSQL,
			UpChecksum:   pair.UpChecksum,
			DownChecksum: pair.DownChecksum,
		})
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

type migrationPair struct {
	Version      string
	Name         string
	UpPath       string
	DownPath     string
	UpSQL        string
	DownSQL      string
	UpChecksum   string
	DownChecksum string
}

func sortMigrations(migrations []Migration) {
	sort.Slice(migrations, func(i, j int) bool {
		if migrations[i].Scope != migrations[j].Scope {
			return scopeOrder(migrations[i].Scope) < scopeOrder(migrations[j].Scope)
		}
		return migrations[i].Version < migrations[j].Version
	})
}

func scopeOrder(scope string) int {
	switch scope {
	case MigrationScopeFramework:
		return 0
	case MigrationScopeProject:
		return 1
	default:
		return 2
	}
}

func validateAppliedChecksum(migration Migration, applied AppliedMigration) error {
	if migration.Name != applied.Name {
		return fmt.Errorf("applied migration %s/%s name changed from %q to %q", migration.Scope, migration.Version, applied.Name, migration.Name)
	}
	if migration.UpChecksum != applied.UpChecksum {
		return fmt.Errorf("applied migration %s/%s up checksum changed", migration.Scope, migration.Version)
	}
	if migration.DownChecksum != applied.DownChecksum {
		return fmt.Errorf("applied migration %s/%s down checksum changed", migration.Scope, migration.Version)
	}
	return nil
}

func migrationKey(scope string, version string) string {
	return scope + "/" + version
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func connectMigrationPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
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

package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaLoadPrelude = `
DROP SCHEMA IF EXISTS public CASCADE;
CREATE SCHEMA public;
`

// DatabaseResult reports whether a database lifecycle operation changed state.
type DatabaseResult struct {
	Name    string
	Changed bool
}

// DatabaseVersion reports the latest applied migration version.
type DatabaseVersion struct {
	Version string
	Found   bool
}

// Manager runs database lifecycle and schema commands.
type Manager struct {
	Migrator     Migrator
	SchemaLoader SchemaLoader
}

// SchemaLoader loads a schema snapshot into a database.
type SchemaLoader interface {
	Load(ctx context.Context, root string, databaseURL string) error
}

// NewManager returns the default database manager.
func NewManager(migrator Migrator) Manager {
	return Manager{
		Migrator:     migrator,
		SchemaLoader: PsqlSchemaLoader{},
	}
}

// Check verifies database connectivity.
func (m Manager) Check(ctx context.Context, databaseURL string) error {
	return Check(ctx, databaseURL)
}

// Create creates the configured database if it is missing.
func (m Manager) Create(ctx context.Context, databaseURL string) (DatabaseResult, error) {
	target, err := ParseDatabaseTarget(databaseURL)
	if err != nil {
		return DatabaseResult{}, err
	}
	conn, err := connectMaintenance(ctx, target.MaintenanceURL)
	if err != nil {
		return DatabaseResult{}, sanitizeError("connect to postgres", databaseURL, err)
	}
	defer conn.Close(ctx)

	exists, err := databaseExists(ctx, conn, target.Name)
	if err != nil {
		return DatabaseResult{}, sanitizeError("check database", databaseURL, err)
	}
	if exists {
		return DatabaseResult{Name: target.Name, Changed: false}, nil
	}
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{target.Name}.Sanitize()); err != nil {
		return DatabaseResult{}, sanitizeError("create database", databaseURL, err)
	}
	return DatabaseResult{Name: target.Name, Changed: true}, nil
}

// Drop drops the configured database if it exists.
func (m Manager) Drop(ctx context.Context, databaseURL string) (DatabaseResult, error) {
	target, err := ParseDatabaseTarget(databaseURL)
	if err != nil {
		return DatabaseResult{}, err
	}
	conn, err := connectMaintenance(ctx, target.MaintenanceURL)
	if err != nil {
		return DatabaseResult{}, sanitizeError("connect to postgres", databaseURL, err)
	}
	defer conn.Close(ctx)

	exists, err := databaseExists(ctx, conn, target.Name)
	if err != nil {
		return DatabaseResult{}, sanitizeError("check database", databaseURL, err)
	}
	if !exists {
		return DatabaseResult{Name: target.Name, Changed: false}, nil
	}
	if _, err := conn.Exec(ctx, "DROP DATABASE "+pgx.Identifier{target.Name}.Sanitize()+" WITH (FORCE)"); err != nil {
		return DatabaseResult{}, sanitizeError("drop database", databaseURL, err)
	}
	return DatabaseResult{Name: target.Name, Changed: true}, nil
}

// Prepare creates the database if needed and applies pending migrations.
func (m Manager) Prepare(ctx context.Context, root string, databaseURL string) (MigrationResult, error) {
	if _, err := m.Create(ctx, databaseURL); err != nil {
		return MigrationResult{}, err
	}
	return m.Migrator.Up(ctx, root, databaseURL)
}

// Reset drops, recreates, and migrates the configured database.
func (m Manager) Reset(ctx context.Context, root string, databaseURL string) (MigrationResult, error) {
	if _, err := m.Drop(ctx, databaseURL); err != nil {
		return MigrationResult{}, err
	}
	if _, err := m.Create(ctx, databaseURL); err != nil {
		return MigrationResult{}, err
	}
	return m.Migrator.Up(ctx, root, databaseURL)
}

// SchemaDump writes db/schema.sql from the live database.
func (m Manager) SchemaDump(ctx context.Context, root string, databaseURL string) error {
	return m.Migrator.DumpSchema(ctx, root, databaseURL)
}

// SchemaLoad replaces the public schema with db/schema.sql.
func (m Manager) SchemaLoad(ctx context.Context, root string, databaseURL string) error {
	if m.SchemaLoader == nil {
		return fmt.Errorf("schema loader is required")
	}
	return m.SchemaLoader.Load(ctx, root, databaseURL)
}

// Version returns the latest applied migration version.
func (m Manager) Version(ctx context.Context, root string, databaseURL string) (DatabaseVersion, error) {
	statuses, err := m.Migrator.Status(ctx, root, databaseURL)
	if err != nil {
		return DatabaseVersion{}, err
	}
	var version DatabaseVersion
	for _, status := range statuses {
		if !status.Applied {
			continue
		}
		version.Version = status.Version
		version.Found = true
	}
	return version, nil
}

// PsqlSchemaLoader loads db/schema.sql through psql.
type PsqlSchemaLoader struct{}

// Load replaces the public schema and loads db/schema.sql.
func (l PsqlSchemaLoader) Load(ctx context.Context, root string, databaseURL string) error {
	schemaPath := filepath.Join(root, filepath.FromSlash(SchemaPath))
	if _, err := os.Stat(schemaPath); err != nil {
		return fmt.Errorf("read schema snapshot %s: %w", schemaPath, err)
	}

	pool, err := connectMigrationPool(ctx, databaseURL)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, schemaLoadPrelude); err != nil {
		pool.Close()
		return sanitizeError("reset public schema", databaseURL, err)
	}
	pool.Close()

	psql, err := findPSQL()
	if err != nil {
		return err
	}
	connectionString, password := pgDumpConnection(databaseURL)
	args := []string{"--set", "ON_ERROR_STOP=1", "--file", schemaPath, connectionString}
	cmd := exec.CommandContext(ctx, psql, args...)
	cmd.Env = os.Environ()
	if password != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+password)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("load schema with psql: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// DatabaseTarget is the database named by a PostgreSQL URL plus a maintenance URL.
type DatabaseTarget struct {
	Name           string
	MaintenanceURL string
}

// ParseDatabaseTarget extracts the target database and maintenance URL.
func ParseDatabaseTarget(databaseURL string) (DatabaseTarget, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return DatabaseTarget{}, fmt.Errorf("database url is required")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return DatabaseTarget{}, fmt.Errorf("invalid postgres database url")
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return DatabaseTarget{}, fmt.Errorf("database url must use postgres")
	}
	name, err := url.PathUnescape(strings.TrimPrefix(parsed.Path, "/"))
	if err != nil {
		return DatabaseTarget{}, fmt.Errorf("invalid postgres database name")
	}
	if strings.TrimSpace(name) == "" {
		return DatabaseTarget{}, fmt.Errorf("database url must include a database name")
	}
	if reservedDatabaseName(name) {
		return DatabaseTarget{}, fmt.Errorf("database %q is reserved and cannot be managed by dygo", name)
	}
	maintenance := *parsed
	maintenance.Path = "/postgres"
	return DatabaseTarget{Name: name, MaintenanceURL: maintenance.String()}, nil
}

func reservedDatabaseName(name string) bool {
	switch name {
	case "postgres", "template0", "template1":
		return true
	default:
		return false
	}
}

func connectMaintenance(ctx context.Context, databaseURL string) (*pgx.Conn, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid postgres database url")
	}
	return pgx.ConnectConfig(ctx, cfg.ConnConfig)
}

func databaseExists(ctx context.Context, conn *pgx.Conn, name string) (bool, error) {
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func findPSQL() (string, error) {
	if path, err := exec.LookPath("psql"); err == nil {
		return path, nil
	}
	const postgresAppPSQL = "/Applications/Postgres.app/Contents/Versions/latest/bin/psql"
	if info, err := os.Stat(postgresAppPSQL); err == nil && !info.IsDir() {
		return postgresAppPSQL, nil
	}
	return "", fmt.Errorf("psql not found in PATH or Postgres.app")
}

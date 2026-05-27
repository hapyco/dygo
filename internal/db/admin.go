package db

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseResult reports whether a database lifecycle operation changed state.
type DatabaseResult struct {
	Name    string
	Changed bool
}

// DatabaseStatus reports whether a configured database exists.
type DatabaseStatus struct {
	Name   string
	Exists bool
}

// Manager runs database lifecycle and schema commands.
type Manager struct {
	Migrator Migrator
}

// NewManager returns the default database manager.
func NewManager(migrator Migrator) Manager {
	return Manager{Migrator: migrator}
}

// Check verifies database connectivity.
func (m Manager) Check(ctx context.Context, databaseURL string) error {
	return Check(ctx, databaseURL)
}

// Exists reports whether the configured database exists.
func (m Manager) Exists(ctx context.Context, databaseURL string) (DatabaseStatus, error) {
	target, err := ParseDatabaseTarget(databaseURL)
	if err != nil {
		return DatabaseStatus{}, err
	}
	conn, err := connectMaintenance(ctx, target.MaintenanceURL)
	if err != nil {
		return DatabaseStatus{}, sanitizeError("connect to postgres", databaseURL, err)
	}
	defer conn.Close(ctx)

	exists, err := databaseExists(ctx, conn, target.Name)
	if err != nil {
		return DatabaseStatus{}, sanitizeError("check database", databaseURL, err)
	}
	return DatabaseStatus{Name: target.Name, Exists: exists}, nil
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

type maintenanceConn interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Close(context.Context) error
}

var connectMaintenance = func(ctx context.Context, databaseURL string) (maintenanceConn, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid postgres database url")
	}
	return pgx.ConnectConfig(ctx, cfg.ConnConfig)
}

func databaseExists(ctx context.Context, conn maintenanceConn, name string) (bool, error) {
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

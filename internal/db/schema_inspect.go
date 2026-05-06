package db

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// LiveSchema is the inspected PostgreSQL public schema.
type LiveSchema struct {
	Tables map[string]liveTable
}

type liveTable struct {
	Name        string
	Columns     map[string]liveColumn
	Indexes     map[string]liveIndex
	Constraints map[string]liveConstraint
}

func (t liveTable) HasIndex(name string) bool {
	_, ok := t.Indexes[name]
	return ok
}

type liveColumn struct {
	Name     string
	Type     string
	Nullable bool
}

type liveIndex struct {
	Name       string
	Definition string
}

type liveConstraint struct {
	Name       string
	Type       string
	Definition string
}

// InspectLiveSchema reads the PostgreSQL public schema.
func InspectLiveSchema(ctx context.Context, pool *pgxpool.Pool) (LiveSchema, error) {
	live := LiveSchema{Tables: map[string]liveTable{}}
	if err := inspectTables(ctx, pool, &live); err != nil {
		return LiveSchema{}, err
	}
	if err := inspectColumns(ctx, pool, &live); err != nil {
		return LiveSchema{}, err
	}
	if err := inspectIndexes(ctx, pool, &live); err != nil {
		return LiveSchema{}, err
	}
	if err := inspectConstraints(ctx, pool, &live); err != nil {
		return LiveSchema{}, err
	}
	return live, nil
}

func inspectTables(ctx context.Context, pool *pgxpool.Pool, live *LiveSchema) error {
	rows, err := pool.Query(ctx, `
SELECT c.relname
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
	AND c.relkind IN ('r', 'p')
ORDER BY c.relname`)
	if err != nil {
		return fmt.Errorf("inspect schema tables: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan schema table: %w", err)
		}
		ensureLiveTable(live, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect schema tables: %w", err)
	}
	return nil
}

func inspectColumns(ctx context.Context, pool *pgxpool.Pool, live *LiveSchema) error {
	rows, err := pool.Query(ctx, `
SELECT c.relname, a.attname, format_type(a.atttypid, a.atttypmod), a.attnotnull
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid
WHERE n.nspname = 'public'
	AND c.relkind IN ('r', 'p')
	AND a.attnum > 0
	AND NOT a.attisdropped
ORDER BY c.relname, a.attnum`)
	if err != nil {
		return fmt.Errorf("inspect schema columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		var columnName string
		var dataType string
		var notNull bool
		if err := rows.Scan(&tableName, &columnName, &dataType, &notNull); err != nil {
			return fmt.Errorf("scan schema column: %w", err)
		}
		table := ensureLiveTable(live, tableName)
		table.Columns[columnName] = liveColumn{Name: columnName, Type: normalizeSQLType(dataType), Nullable: !notNull}
		live.Tables[tableName] = table
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect schema columns: %w", err)
	}
	return nil
}

func inspectIndexes(ctx context.Context, pool *pgxpool.Pool, live *LiveSchema) error {
	rows, err := pool.Query(ctx, `
SELECT tablename, indexname, indexdef
FROM pg_indexes
WHERE schemaname = 'public'
ORDER BY tablename, indexname`)
	if err != nil {
		return fmt.Errorf("inspect schema indexes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		var indexName string
		var definition string
		if err := rows.Scan(&tableName, &indexName, &definition); err != nil {
			return fmt.Errorf("scan schema index: %w", err)
		}
		table := ensureLiveTable(live, tableName)
		table.Indexes[indexName] = liveIndex{Name: indexName, Definition: definition}
		live.Tables[tableName] = table
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect schema indexes: %w", err)
	}
	return nil
}

func inspectConstraints(ctx context.Context, pool *pgxpool.Pool, live *LiveSchema) error {
	rows, err := pool.Query(ctx, `
SELECT c.relname, con.conname, con.contype::text, pg_get_constraintdef(con.oid)
FROM pg_constraint con
JOIN pg_class c ON c.oid = con.conrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public'
	AND c.relkind IN ('r', 'p')
ORDER BY c.relname, con.conname`)
	if err != nil {
		return fmt.Errorf("inspect schema constraints: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var tableName string
		var constraintName string
		var constraintType string
		var definition string
		if err := rows.Scan(&tableName, &constraintName, &constraintType, &definition); err != nil {
			return fmt.Errorf("scan schema constraint: %w", err)
		}
		table := ensureLiveTable(live, tableName)
		table.Constraints[constraintName] = liveConstraint{Name: constraintName, Type: normalizeConstraintType(constraintType), Definition: definition}
		live.Tables[tableName] = table
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect schema constraints: %w", err)
	}
	return nil
}

func ensureLiveTable(live *LiveSchema, name string) liveTable {
	if live.Tables == nil {
		live.Tables = map[string]liveTable{}
	}
	if table, ok := live.Tables[name]; ok {
		if table.Columns == nil {
			table.Columns = map[string]liveColumn{}
		}
		if table.Indexes == nil {
			table.Indexes = map[string]liveIndex{}
		}
		if table.Constraints == nil {
			table.Constraints = map[string]liveConstraint{}
		}
		return table
	}
	table := liveTable{
		Name:        name,
		Columns:     map[string]liveColumn{},
		Indexes:     map[string]liveIndex{},
		Constraints: map[string]liveConstraint{},
	}
	live.Tables[name] = table
	return table
}

func normalizeSQLType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "timestamp with time zone":
		return "timestamptz"
	case "time without time zone":
		return "time"
	default:
		return value
	}
}

func normalizeConstraintType(value string) string {
	switch value {
	case "p":
		return "primary-key"
	case "u":
		return "unique"
	case "c":
		return "check"
	case "f":
		return "foreign-key"
	case "n":
		return "not-null"
	default:
		return value
	}
}

func sortedTableNames(tables map[string]liveTable) []string {
	names := make([]string, 0, len(tables))
	for name := range tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedColumnNames(columns map[string]liveColumn) []string {
	names := make([]string, 0, len(columns))
	for name := range columns {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedConstraintNames(constraints map[string]liveConstraint) []string {
	names := make([]string, 0, len(constraints))
	for name := range constraints {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

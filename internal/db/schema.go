package db

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dygo-dev/dygo/internal/app/registry"
	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/dygo-dev/dygo/internal/entity/schema"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"
)

// SyncMetadataSchema creates PostgreSQL tables from discovered app Entity metadata.
func SyncMetadataSchema(ctx context.Context, pool *pgxpool.Pool, root string) (SchemaSyncResult, error) {
	apps, err := registry.New(root).Validate()
	if err != nil {
		return SchemaSyncResult{}, fmt.Errorf("validate apps for metadata schema: %w", err)
	}
	entities, err := catalog.New(apps, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		return SchemaSyncResult{}, fmt.Errorf("validate entities for metadata schema: %w", err)
	}
	return ApplyMetadataSchema(ctx, pool, entities)
}

// ApplyMetadataSchema applies Entity metadata tables to PostgreSQL.
func ApplyMetadataSchema(ctx context.Context, pool *pgxpool.Pool, entities []catalog.LoadedEntity) (SchemaSyncResult, error) {
	statements, err := BuildMetadataSchemaStatements(entities)
	if err != nil {
		return SchemaSyncResult{}, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return SchemaSyncResult{}, fmt.Errorf("begin metadata schema transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return SchemaSyncResult{}, fmt.Errorf("apply metadata schema statement: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return SchemaSyncResult{}, fmt.Errorf("commit metadata schema transaction: %w", err)
	}
	return schemaSyncResult(entities), nil
}

// BuildMetadataSchemaStatements converts Entity metadata into idempotent PostgreSQL DDL.
func BuildMetadataSchemaStatements(entities []catalog.LoadedEntity) ([]string, error) {
	targets := map[string]catalog.LoadedEntity{}
	for _, entity := range entities {
		targets[entity.Entity.Name] = entity
	}

	ordered := append([]catalog.LoadedEntity(nil), entities...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].AppName != ordered[j].AppName {
			return ordered[i].AppName < ordered[j].AppName
		}
		if ordered[i].Entity.PluralName != ordered[j].Entity.PluralName {
			return ordered[i].Entity.PluralName < ordered[j].Entity.PluralName
		}
		return ordered[i].Path < ordered[j].Path
	})

	var creates []string
	var columns []string
	var constraints []string
	seenTables := map[string]catalog.LoadedEntity{}
	for _, loaded := range ordered {
		table, err := tableName(loaded.Entity)
		if err != nil {
			return nil, entitySchemaError(loaded, err)
		}
		if previous, ok := seenTables[table]; ok {
			return nil, entitySchemaError(loaded, fmt.Errorf("duplicate table name %q also used by %s/%s at %s", table, previous.AppName, previous.Entity.Name, previous.Path))
		}
		seenTables[table] = loaded

		creates = append(creates, fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	%s bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	%s timestamptz NOT NULL DEFAULT now(),
	%s timestamptz NOT NULL DEFAULT now()
)`, quoteIdent(table), quoteIdent("id"), quoteIdent("created_at"), quoteIdent("updated_at")))

		for _, field := range loaded.Entity.Fields {
			column, err := columnForField(field)
			if err != nil {
				return nil, fieldSchemaError(loaded, field, err)
			}
			sqlType, err := columnType(field, targets)
			if err != nil {
				return nil, fieldSchemaError(loaded, field, err)
			}
			definition := quoteIdent(column) + " " + sqlType
			if defaultSQL, ok, err := defaultClause(field.Default); err != nil {
				return nil, fieldSchemaError(loaded, field, err)
			} else if ok {
				definition += " DEFAULT " + defaultSQL
			}
			if field.Required {
				definition += " NOT NULL"
			}
			columns = append(columns, fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s", quoteIdent(table), definition))

			if field.Unique {
				constraint := constraintName(table, column, "key")
				constraints = append(constraints, addConstraintStatement(table, constraint, fmt.Sprintf("UNIQUE (%s)", quoteIdent(column))))
			}
			if field.Type == "select" && len(field.Options.Values) > 0 {
				constraint := constraintName(table, column, "check")
				constraints = append(constraints, addConstraintStatement(table, constraint, selectCheck(column, field.Options.Values)))
			}
			if field.Type == "link" {
				target, ok := targets[field.Options.Entity]
				if !ok {
					return nil, fieldSchemaError(loaded, field, fmt.Errorf("link target %q is not loaded", field.Options.Entity))
				}
				targetTable, err := tableName(target.Entity)
				if err != nil {
					return nil, fieldSchemaError(loaded, field, fmt.Errorf("invalid link target table: %w", err))
				}
				constraint := constraintName(table, column, "fkey")
				constraints = append(constraints, addConstraintStatement(table, constraint, fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s) ON DELETE CASCADE", quoteIdent(column), quoteIdent(targetTable), quoteIdent("id"))))
			}
		}
	}

	statements := make([]string, 0, len(creates)+len(columns)+len(constraints))
	statements = append(statements, creates...)
	statements = append(statements, columns...)
	statements = append(statements, constraints...)
	return statements, nil
}

func schemaSyncResult(entities []catalog.LoadedEntity) SchemaSyncResult {
	result := SchemaSyncResult{Entities: len(entities)}
	for _, entity := range entities {
		result.Fields += len(entity.Entity.Fields)
	}
	return result
}

func tableName(entity schema.Entity) (string, error) {
	if strings.TrimSpace(entity.PluralName) == "" {
		return "", fmt.Errorf("plural-name is required")
	}
	return strings.ReplaceAll(entity.PluralName, "-", "_"), nil
}

func columnForField(field schema.Field) (string, error) {
	if field.Type == "child-table" {
		return "", fmt.Errorf("child-table storage is not supported by metadata schema sync yet")
	}
	name := strings.ReplaceAll(field.Name, "-", "_")
	if field.Type == "link" {
		name += "_id"
	}
	return name, nil
}

func columnType(field schema.Field, targets map[string]catalog.LoadedEntity) (string, error) {
	switch field.Type {
	case "text", "long-text", "select", "attachment":
		return "text", nil
	case "int":
		return "integer", nil
	case "decimal", "currency":
		return "numeric", nil
	case "boolean":
		return "boolean", nil
	case "date":
		return "date", nil
	case "datetime":
		return "timestamptz", nil
	case "time":
		return "time", nil
	case "json":
		return "jsonb", nil
	case "link":
		if _, ok := targets[field.Options.Entity]; !ok {
			return "", fmt.Errorf("link target %q is not loaded", field.Options.Entity)
		}
		return "bigint", nil
	case "child-table":
		return "", fmt.Errorf("child-table storage is not supported by metadata schema sync yet")
	default:
		return "", fmt.Errorf("unsupported field type %q", field.Type)
	}
}

func defaultClause(node yaml.Node) (string, bool, error) {
	if node.Kind == 0 {
		return "", false, nil
	}
	if node.Kind != yaml.ScalarNode {
		return "", false, fmt.Errorf("default must be a scalar value")
	}
	switch node.Tag {
	case "!!bool":
		value, err := strconv.ParseBool(node.Value)
		if err != nil {
			return "", false, fmt.Errorf("invalid boolean default %q", node.Value)
		}
		if value {
			return "true", true, nil
		}
		return "false", true, nil
	case "!!int", "!!float":
		return node.Value, true, nil
	case "!!null":
		return "NULL", true, nil
	default:
		return quoteLiteral(node.Value), true, nil
	}
}

func selectCheck(column string, values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, quoteLiteral(value))
	}
	return fmt.Sprintf("CHECK (%s IN (%s))", quoteIdent(column), strings.Join(quoted, ", "))
}

func addConstraintStatement(table string, constraint string, definition string) string {
	return fmt.Sprintf(`DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conrelid = 'public.%s'::regclass
			AND conname = %s
	) THEN
		ALTER TABLE %s ADD CONSTRAINT %s %s;
	END IF;
END $$`, table, quoteLiteral(constraint), quoteIdent(table), quoteIdent(constraint), definition)
}

func constraintName(table string, column string, suffix string) string {
	return table + "_" + column + "_" + suffix
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func entitySchemaError(entity catalog.LoadedEntity, err error) error {
	return fmt.Errorf("entity %s/%s at %s: %w", entity.AppName, entity.Entity.Name, entity.Path, err)
}

func fieldSchemaError(entity catalog.LoadedEntity, field schema.Field, err error) error {
	return fmt.Errorf("entity %s/%s field %q at %s: %w", entity.AppName, entity.Entity.Name, field.Name, entity.Path, err)
}

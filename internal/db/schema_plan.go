package db

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/schema"
	"gopkg.in/yaml.v3"
)

const (
	SchemaOperationSafe = "safe"

	SchemaDiagnosticUnsafe      = "unsafe"
	SchemaDiagnosticUnsupported = "unsupported"
)

// SchemaPlan describes the safe operations and blocking diagnostics for metadata schema sync.
type SchemaPlan struct {
	Entities    int
	Fields      int
	Operations  []SchemaOperation
	Diagnostics []SchemaDiagnostic
}

// SchemaOperation is one safe schema operation generated from metadata.
type SchemaOperation struct {
	Classification string
	Kind           string
	Table          string
	Column         string
	Name           string
	Description    string
	Source         string
	SQL            string
}

// SchemaDiagnostic is an unsafe or unsupported schema difference.
type SchemaDiagnostic struct {
	Classification string
	Kind           string
	Table          string
	Column         string
	Name           string
	Message        string
	Source         string
}

// HasBlockers reports whether a plan has diagnostics that should stop sync.
func (p SchemaPlan) HasBlockers() bool {
	return len(p.Diagnostics) > 0
}

// BlockerError returns an error when unsafe or unsupported diagnostics exist.
func (p SchemaPlan) BlockerError() error {
	if !p.HasBlockers() {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "schema plan has %d blocker", len(p.Diagnostics))
	if len(p.Diagnostics) != 1 {
		b.WriteString("s")
	}
	for _, diagnostic := range p.Diagnostics {
		b.WriteString("\n")
		b.WriteString(diagnostic.String())
	}
	return errors.New(b.String())
}

// Result converts a successful plan into a sync result.
func (p SchemaPlan) Result() SchemaSyncResult {
	return SchemaSyncResult{Entities: p.Entities, Fields: p.Fields, Operations: len(p.Operations)}
}

func (d SchemaDiagnostic) String() string {
	target := d.Table
	if d.Column != "" {
		target += "." + d.Column
	}
	if target == "" && d.Name != "" {
		target = d.Name
	}
	if target == "" {
		target = "schema"
	}
	if d.Source != "" {
		return fmt.Sprintf("%s: %s: %s (%s)", d.Classification, target, d.Message, d.Source)
	}
	return fmt.Sprintf("%s: %s: %s", d.Classification, target, d.Message)
}

// BuildMetadataSchemaPlan compares Entity metadata with an inspected live schema.
func BuildMetadataSchemaPlan(entities []catalog.LoadedEntity, live LiveSchema) (SchemaPlan, error) {
	desired, err := buildDesiredSchema(entities)
	if err != nil {
		return SchemaPlan{}, err
	}

	plan := SchemaPlan{Entities: len(entities)}
	for _, entity := range entities {
		plan.Fields += len(entity.Entity.Fields)
	}

	var creates []SchemaOperation
	var columns []SchemaOperation
	var indexes []SchemaOperation
	var constraints []SchemaOperation

	desiredTables := map[string]desiredTable{}
	for _, table := range desired.Tables {
		desiredTables[table.Name] = table
		liveTable, tableExists := live.Tables[table.Name]
		if !tableExists {
			creates = append(creates, SchemaOperation{
				Classification: SchemaOperationSafe,
				Kind:           "create-table",
				Table:          table.Name,
				Description:    "create table " + table.Name,
				Source:         table.Source,
				SQL:            table.CreateSQL,
			})
		}

		availableColumns := map[string]bool{}
		if tableExists {
			for _, column := range table.SystemColumns {
				liveColumn, ok := liveTable.Columns[column.Name]
				if !ok {
					plan.Diagnostics = append(plan.Diagnostics, unsupportedDiagnostic("missing-system-column", table.Name, column.Name, "", "system column is missing from existing table", table.Source))
					continue
				}
				availableColumns[column.Name] = true
				compareColumn(&plan, table.Name, column, liveColumn)
			}
		} else {
			for _, column := range table.SystemColumns {
				availableColumns[column.Name] = true
			}
		}

		for _, column := range table.Columns {
			liveColumn, columnExists := liveTable.Columns[column.Name]
			if !tableExists || !columnExists {
				if tableExists && column.Required && !column.HasSafeDefault {
					plan.Diagnostics = append(plan.Diagnostics, unsafeDiagnostic("missing-required-column", table.Name, column.Name, "", "required column is missing and has no safe default", column.Source))
					continue
				}
				availableColumns[column.Name] = true
				columns = append(columns, SchemaOperation{
					Classification: SchemaOperationSafe,
					Kind:           "add-column",
					Table:          table.Name,
					Column:         column.Name,
					Description:    "add column " + table.Name + "." + column.Name,
					Source:         column.Source,
					SQL:            fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", quoteIdent(table.Name), column.Definition),
				})
				continue
			}
			availableColumns[column.Name] = true
			compareColumn(&plan, table.Name, column, liveColumn)
		}

		if tableExists {
			reportExtraColumns(&plan, table, liveTable)
		}

		for _, index := range table.Indexes {
			if !availableColumns[index.Column] {
				continue
			}
			if tableExists && liveTable.HasIndex(index.Name) {
				continue
			}
			indexes = append(indexes, SchemaOperation{
				Classification: SchemaOperationSafe,
				Kind:           "create-index",
				Table:          table.Name,
				Column:         index.Column,
				Name:           index.Name,
				Description:    "create index " + index.Name + " on " + table.Name + "." + index.Column,
				Source:         index.Source,
				SQL:            fmt.Sprintf("CREATE INDEX %s ON %s (%s)", quoteIdent(index.Name), quoteIdent(table.Name), quoteIdent(index.Column)),
			})
		}

		for _, constraint := range table.Constraints {
			if strings.HasPrefix(constraint.Name, "unsupported:") {
				plan.Diagnostics = append(plan.Diagnostics, unsupportedDiagnostic("unsupported-field-storage", table.Name, constraint.Column, "", constraint.Definition, constraint.Source))
				continue
			}
			if !availableColumns[constraint.Column] {
				continue
			}
			if tableExists {
				if liveConstraint, ok := liveTable.Constraints[constraint.Name]; ok {
					if liveConstraint.Type != constraint.Type {
						plan.Diagnostics = append(plan.Diagnostics, unsafeDiagnostic("constraint-type-drift", table.Name, constraint.Column, constraint.Name, fmt.Sprintf("constraint %q is %s in database but metadata expects %s", constraint.Name, liveConstraint.Type, constraint.Type), constraint.Source))
					}
					continue
				}
			}
			constraints = append(constraints, SchemaOperation{
				Classification: SchemaOperationSafe,
				Kind:           "add-constraint",
				Table:          table.Name,
				Column:         constraint.Column,
				Name:           constraint.Name,
				Description:    "add " + constraint.Type + " constraint " + constraint.Name + " on " + table.Name + "." + constraint.Column,
				Source:         constraint.Source,
				SQL:            fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s %s", quoteIdent(table.Name), quoteIdent(constraint.Name), constraint.Definition),
			})
		}

		if tableExists {
			reportExtraConstraints(&plan, table, liveTable)
		}
	}

	reportExtraTables(&plan, desiredTables, live)

	plan.Operations = append(plan.Operations, creates...)
	plan.Operations = append(plan.Operations, columns...)
	plan.Operations = append(plan.Operations, indexes...)
	plan.Operations = append(plan.Operations, constraints...)
	return plan, nil
}

type desiredSchema struct {
	Tables []desiredTable
}

type desiredTable struct {
	Name          string
	Source        string
	CreateSQL     string
	SystemColumns []desiredColumn
	Columns       []desiredColumn
	Indexes       []desiredIndex
	Constraints   []desiredConstraint
}

type desiredColumn struct {
	Name           string
	Type           string
	Required       bool
	HasSafeDefault bool
	Definition     string
	Source         string
}

type desiredIndex struct {
	Name   string
	Column string
	Source string
}

type desiredConstraint struct {
	Name       string
	Type       string
	Column     string
	Definition string
	Source     string
}

func buildDesiredSchema(entities []catalog.LoadedEntity) (desiredSchema, error) {
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

	var desired desiredSchema
	seenTables := map[string]catalog.LoadedEntity{}
	for _, loaded := range ordered {
		table, err := tableName(loaded.Entity)
		if err != nil {
			return desiredSchema{}, entitySchemaError(loaded, err)
		}
		if previous, ok := seenTables[table]; ok {
			return desiredSchema{}, entitySchemaError(loaded, fmt.Errorf("duplicate table name %q also used by %s/%s at %s", table, previous.AppName, previous.Entity.Name, previous.Path))
		}
		seenTables[table] = loaded

		desiredTable := desiredTable{
			Name:      table,
			Source:    entitySource(loaded),
			CreateSQL: createTableSQL(table),
			SystemColumns: []desiredColumn{
				{Name: "id", Type: "bigint", Required: true, Source: entitySource(loaded)},
				{Name: "created_at", Type: "timestamptz", Required: true, Source: entitySource(loaded)},
				{Name: "updated_at", Type: "timestamptz", Required: true, Source: entitySource(loaded)},
			},
		}

		for _, field := range loaded.Entity.Fields {
			column, err := columnForField(field)
			if err != nil {
				desiredTable.Constraints = append(desiredTable.Constraints, desiredConstraint{
					Name:       "unsupported:" + field.Name,
					Type:       SchemaDiagnosticUnsupported,
					Column:     field.Name,
					Definition: err.Error(),
					Source:     fieldSource(loaded, field),
				})
				continue
			}
			sqlType, err := columnType(field, targets)
			if err != nil {
				return desiredSchema{}, fieldSchemaError(loaded, field, err)
			}
			definition := quoteIdent(column) + " " + sqlType
			hasDefault := false
			hasSafeDefault := false
			if defaultSQL, ok, err := defaultClause(field.Default); err != nil {
				return desiredSchema{}, fieldSchemaError(loaded, field, err)
			} else if ok {
				hasDefault = true
				hasSafeDefault = defaultSQL != "NULL"
				definition += " DEFAULT " + defaultSQL
			}
			if field.Required {
				definition += " NOT NULL"
			}
			desiredTable.Columns = append(desiredTable.Columns, desiredColumn{
				Name:           column,
				Type:           sqlType,
				Required:       field.Required,
				HasSafeDefault: !field.Required || hasDefault && hasSafeDefault,
				Definition:     definition,
				Source:         fieldSource(loaded, field),
			})

			if field.Unique {
				constraint := constraintName(table, column, "key")
				desiredTable.Constraints = append(desiredTable.Constraints, desiredConstraint{
					Name:       constraint,
					Type:       "unique",
					Column:     column,
					Definition: fmt.Sprintf("UNIQUE (%s)", quoteIdent(column)),
					Source:     fieldSource(loaded, field),
				})
			}
			if field.Index {
				index := constraintName(table, column, "idx")
				desiredTable.Indexes = append(desiredTable.Indexes, desiredIndex{
					Name:   index,
					Column: column,
					Source: fieldSource(loaded, field),
				})
			}
			if field.Type == "select" && len(field.Options.Values) > 0 {
				constraint := constraintName(table, column, "check")
				desiredTable.Constraints = append(desiredTable.Constraints, desiredConstraint{
					Name:       constraint,
					Type:       "check",
					Column:     column,
					Definition: selectCheck(column, field.Options.Values),
					Source:     fieldSource(loaded, field),
				})
			}
			if field.Type == "link" {
				target, ok := targets[field.Options.Entity]
				if !ok {
					return desiredSchema{}, fieldSchemaError(loaded, field, fmt.Errorf("link target %q is not loaded", field.Options.Entity))
				}
				targetTable, err := tableName(target.Entity)
				if err != nil {
					return desiredSchema{}, fieldSchemaError(loaded, field, fmt.Errorf("invalid link target table: %w", err))
				}
				constraint := constraintName(table, column, "fkey")
				desiredTable.Constraints = append(desiredTable.Constraints, desiredConstraint{
					Name:       constraint,
					Type:       "foreign-key",
					Column:     column,
					Definition: fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s) ON DELETE CASCADE", quoteIdent(column), quoteIdent(targetTable), quoteIdent("id")),
					Source:     fieldSource(loaded, field),
				})
			}
		}
		desired.Tables = append(desired.Tables, desiredTable)
	}
	return desired, nil
}

func compareColumn(plan *SchemaPlan, table string, desired desiredColumn, live liveColumn) {
	if normalizeSQLType(live.Type) != normalizeSQLType(desired.Type) {
		plan.Diagnostics = append(plan.Diagnostics, unsafeDiagnostic("column-type-drift", table, desired.Name, "", fmt.Sprintf("column type is %s in database but metadata expects %s", live.Type, desired.Type), desired.Source))
	}
	if desired.Required && live.Nullable {
		plan.Diagnostics = append(plan.Diagnostics, unsafeDiagnostic("column-required-drift", table, desired.Name, "", "column is nullable in database but required in metadata", desired.Source))
	}
	if !desired.Required && !live.Nullable {
		plan.Diagnostics = append(plan.Diagnostics, unsafeDiagnostic("column-required-drift", table, desired.Name, "", "column is NOT NULL in database but not required in metadata", desired.Source))
	}
}

func reportExtraColumns(plan *SchemaPlan, desired desiredTable, live liveTable) {
	expected := map[string]bool{}
	for _, column := range desired.SystemColumns {
		expected[column.Name] = true
	}
	for _, column := range desired.Columns {
		expected[column.Name] = true
	}
	names := sortedColumnNames(live.Columns)
	for _, name := range names {
		if expected[name] {
			continue
		}
		plan.Diagnostics = append(plan.Diagnostics, unsafeDiagnostic("extra-column", desired.Name, name, "", "column exists in database but not metadata", desired.Source))
	}
}

func reportExtraConstraints(plan *SchemaPlan, desired desiredTable, live liveTable) {
	expected := map[string]bool{}
	for _, constraint := range desired.Constraints {
		expected[constraint.Name] = true
	}
	names := sortedConstraintNames(live.Constraints)
	for _, name := range names {
		constraint := live.Constraints[name]
		if constraint.Type == "primary-key" || constraint.Type == "not-null" || expected[name] {
			continue
		}
		plan.Diagnostics = append(plan.Diagnostics, unsafeDiagnostic("extra-constraint", desired.Name, "", name, fmt.Sprintf("constraint %q exists in database but not metadata", name), desired.Source))
	}
}

func reportExtraTables(plan *SchemaPlan, desired map[string]desiredTable, live LiveSchema) {
	names := sortedTableNames(live.Tables)
	for _, name := range names {
		if _, ok := desired[name]; ok {
			continue
		}
		plan.Diagnostics = append(plan.Diagnostics, unsafeDiagnostic("extra-table", name, "", "", "table exists in database but not metadata", "database public schema"))
	}
}

func unsafeDiagnostic(kind string, table string, column string, name string, message string, source string) SchemaDiagnostic {
	return SchemaDiagnostic{Classification: SchemaDiagnosticUnsafe, Kind: kind, Table: table, Column: column, Name: name, Message: message, Source: source}
}

func unsupportedDiagnostic(kind string, table string, column string, name string, message string, source string) SchemaDiagnostic {
	return SchemaDiagnostic{Classification: SchemaDiagnosticUnsupported, Kind: kind, Table: table, Column: column, Name: name, Message: message, Source: source}
}

func createTableSQL(table string) string {
	return fmt.Sprintf(`CREATE TABLE %s (
	%s bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	%s timestamptz NOT NULL DEFAULT now(),
	%s timestamptz NOT NULL DEFAULT now()
)`, quoteIdent(table), quoteIdent("id"), quoteIdent("created_at"), quoteIdent("updated_at"))
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
	case "text", "email", "phone", "long-text", "select", "attachment":
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

func constraintName(table string, column string, suffix string) string {
	return table + "_" + column + "_" + suffix
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func entitySource(entity catalog.LoadedEntity) string {
	return fmt.Sprintf("app %s entity %s at %s", entity.AppName, entity.Entity.Name, entity.Path)
}

func fieldSource(entity catalog.LoadedEntity, field schema.Field) string {
	return fmt.Sprintf("app %s entity %s field %s at %s", entity.AppName, entity.Entity.Name, field.Name, entity.Path)
}

func entitySchemaError(entity catalog.LoadedEntity, err error) error {
	return fmt.Errorf("entity %s/%s at %s: %w", entity.AppName, entity.Entity.Name, entity.Path, err)
}

func fieldSchemaError(entity catalog.LoadedEntity, field schema.Field, err error) error {
	return fmt.Errorf("entity %s/%s field %q at %s: %w", entity.AppName, entity.Entity.Name, field.Name, entity.Path, err)
}

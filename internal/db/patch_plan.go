package db

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/schema"
	"github.com/dygo-dev/dygo/internal/patches"
	"gopkg.in/yaml.v3"
)

const (
	PatchOperationRenameField     = patches.OperationRenameField
	PatchOperationRenameEntity    = patches.OperationRenameEntity
	PatchOperationCopyField       = patches.OperationCopyField
	PatchOperationBackfillField   = patches.OperationBackfillField
	PatchOperationDropField       = patches.OperationDropField
	PatchOperationChangeFieldType = patches.OperationChangeFieldType
	PatchOperationSQL             = patches.OperationSQL
)

// PatchOperationPlan is a read-only plan for patch operations.
type PatchOperationPlan struct {
	Operations []PatchOperation
}

// PatchOperation is one planned patch operation with exact SQL for review.
type PatchOperation struct {
	AppName         string
	PatchID         string
	Phase           string
	Path            string
	AppRelativePath string
	Checksum        string
	OperationIndex  int
	Type            string
	Table           string
	Column          string
	Name            string
	Description     string
	Source          string
	SQL             string
}

// BuildPatchOperationPlan validates and plans loaded patch operations without executing them.
func BuildPatchOperationPlan(loaded []patches.LoadedPatch, entities []catalog.LoadedEntity, live LiveSchema) (PatchOperationPlan, error) {
	planner, err := newPatchOperationPlanner(entities, live)
	if err != nil {
		return PatchOperationPlan{}, err
	}

	var plan PatchOperationPlan
	for _, patch := range loaded {
		for index, operation := range patch.Patch.Operations {
			reader := patchOperationReader{patch: patch, index: index, operation: operation}
			planned, err := planner.plan(reader)
			if err != nil {
				return PatchOperationPlan{}, fmt.Errorf("%s: %w", reader.source(), err)
			}
			plan.Operations = append(plan.Operations, planned)
		}
	}
	return plan, nil
}

type patchOperationPlanner struct {
	entities map[string]catalog.LoadedEntity
	targets  schemaTargetIndex
	live     LiveSchema
}

func newPatchOperationPlanner(entities []catalog.LoadedEntity, live LiveSchema) (*patchOperationPlanner, error) {
	byIdentity := map[string]catalog.LoadedEntity{}
	for _, entity := range entities {
		key := catalog.EntityKey(entity.AppName, entity.Entity.Name)
		if previous, ok := byIdentity[key]; ok {
			return nil, fmt.Errorf("duplicate entity %s/%s in %s and %s", entity.AppName, entity.Entity.Name, previous.Path, entity.Path)
		}
		byIdentity[key] = entity
	}
	return &patchOperationPlanner{
		entities: byIdentity,
		targets:  newSchemaTargetIndex(entities),
		live:     cloneLiveSchema(live),
	}, nil
}

func (p *patchOperationPlanner) plan(reader patchOperationReader) (PatchOperation, error) {
	switch reader.operation.Type {
	case PatchOperationRenameField:
		return p.planRenameField(reader)
	case PatchOperationRenameEntity:
		return p.planRenameEntity(reader)
	case PatchOperationCopyField:
		return p.planCopyField(reader)
	case PatchOperationBackfillField:
		return p.planBackfillField(reader)
	case PatchOperationDropField:
		return p.planDropField(reader)
	case PatchOperationChangeFieldType:
		return p.planChangeFieldType(reader)
	case PatchOperationSQL:
		return p.planSQL(reader)
	default:
		return PatchOperation{}, fmt.Errorf("unsupported patch operation type %q", reader.operation.Type)
	}
}

func (p *patchOperationPlanner) planRenameField(reader patchOperationReader) (PatchOperation, error) {
	if err := reader.requireOperationFields(); err != nil {
		return PatchOperation{}, err
	}
	entityName, err := reader.requiredString("entity")
	if err != nil {
		return PatchOperation{}, err
	}
	fromField, err := reader.requiredString("from")
	if err != nil {
		return PatchOperation{}, err
	}
	toField, err := reader.requiredString("to")
	if err != nil {
		return PatchOperation{}, err
	}
	entity, err := p.entity(reader.patch.AppName, entityName)
	if err != nil {
		return PatchOperation{}, err
	}
	table, err := tableName(entity)
	if err != nil {
		return PatchOperation{}, err
	}
	fromColumn, err := p.patchFieldColumn(entity, table, fromField)
	if err != nil {
		return PatchOperation{}, err
	}
	if isSystemColumn(fromColumn) {
		return PatchOperation{}, fmt.Errorf("rename-field cannot rename system column %q", fromColumn)
	}
	toColumn, _, err := p.metadataFieldColumn(entity, toField)
	if err != nil {
		return PatchOperation{}, err
	}
	if err := p.requireLiveColumn(table, fromColumn); err != nil {
		return PatchOperation{}, err
	}
	if p.liveColumnExists(table, toColumn) {
		return PatchOperation{}, fmt.Errorf("target column %s.%s already exists", table, toColumn)
	}

	p.renameLiveColumn(table, fromColumn, toColumn)
	operation := reader.base(PatchOperation{
		Table:       table,
		Column:      fromColumn,
		Name:        toColumn,
		Description: "rename column " + table + "." + fromColumn + " to " + toColumn,
		SQL:         fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", quoteIdent(table), quoteIdent(fromColumn), quoteIdent(toColumn)),
	})
	return operation, nil
}

func (p *patchOperationPlanner) planRenameEntity(reader patchOperationReader) (PatchOperation, error) {
	if err := reader.requireOperationFields(); err != nil {
		return PatchOperation{}, err
	}
	fromEntity, err := reader.requiredString("from")
	if err != nil {
		return PatchOperation{}, err
	}
	toEntity, err := reader.requiredString("to")
	if err != nil {
		return PatchOperation{}, err
	}
	toLoaded, err := p.entity(reader.patch.AppName, toEntity)
	if err != nil {
		return PatchOperation{}, err
	}
	fromTable := entityTableName(reader.patch.AppName, fromEntity)
	toTable, err := tableName(toLoaded)
	if err != nil {
		return PatchOperation{}, err
	}
	if err := p.requireLiveTable(fromTable); err != nil {
		return PatchOperation{}, err
	}
	if p.liveTableExists(toTable) {
		return PatchOperation{}, fmt.Errorf("target table %s already exists", toTable)
	}

	p.renameLiveTable(fromTable, toTable)
	operation := reader.base(PatchOperation{
		Table:       fromTable,
		Name:        toTable,
		Description: "rename table " + fromTable + " to " + toTable,
		SQL:         fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quoteIdent(fromTable), quoteIdent(toTable)),
	})
	return operation, nil
}

func (p *patchOperationPlanner) planCopyField(reader patchOperationReader) (PatchOperation, error) {
	if err := reader.requireOperationFields(); err != nil {
		return PatchOperation{}, err
	}
	entityName, err := reader.requiredString("entity")
	if err != nil {
		return PatchOperation{}, err
	}
	fromField, err := reader.requiredString("from")
	if err != nil {
		return PatchOperation{}, err
	}
	toField, err := reader.requiredString("to")
	if err != nil {
		return PatchOperation{}, err
	}
	whenToIsNull, err := reader.whenBool("to-is-null")
	if err != nil {
		return PatchOperation{}, err
	}
	entity, err := p.entity(reader.patch.AppName, entityName)
	if err != nil {
		return PatchOperation{}, err
	}
	table, err := tableName(entity)
	if err != nil {
		return PatchOperation{}, err
	}
	fromColumn, err := p.patchFieldColumn(entity, table, fromField)
	if err != nil {
		return PatchOperation{}, err
	}
	toColumn, _, err := p.metadataFieldColumn(entity, toField)
	if err != nil {
		return PatchOperation{}, err
	}
	if err := p.requireLiveColumn(table, fromColumn); err != nil {
		return PatchOperation{}, err
	}
	if err := p.requireLiveColumn(table, toColumn); err != nil {
		return PatchOperation{}, err
	}

	sql := fmt.Sprintf("UPDATE %s SET %s = %s", quoteIdent(table), quoteIdent(toColumn), quoteIdent(fromColumn))
	description := "copy column " + table + "." + fromColumn + " to " + toColumn
	if whenToIsNull {
		sql += fmt.Sprintf(" WHERE %s IS NULL", quoteIdent(toColumn))
		description += " where " + toColumn + " is null"
	}
	operation := reader.base(PatchOperation{
		Table:       table,
		Column:      toColumn,
		Name:        fromColumn,
		Description: description,
		SQL:         sql,
	})
	return operation, nil
}

func (p *patchOperationPlanner) planBackfillField(reader patchOperationReader) (PatchOperation, error) {
	if err := reader.requireOperationFields(); err != nil {
		return PatchOperation{}, err
	}
	entityName, err := reader.requiredString("entity")
	if err != nil {
		return PatchOperation{}, err
	}
	fieldName, err := reader.requiredString("field")
	if err != nil {
		return PatchOperation{}, err
	}
	valueSQL, err := reader.requiredScalarSQL("value")
	if err != nil {
		return PatchOperation{}, err
	}
	whenFieldIsNull, err := reader.whenBool("field-is-null")
	if err != nil {
		return PatchOperation{}, err
	}
	entity, err := p.entity(reader.patch.AppName, entityName)
	if err != nil {
		return PatchOperation{}, err
	}
	table, err := tableName(entity)
	if err != nil {
		return PatchOperation{}, err
	}
	column, _, err := p.metadataFieldColumn(entity, fieldName)
	if err != nil {
		return PatchOperation{}, err
	}
	if err := p.requireLiveColumn(table, column); err != nil {
		return PatchOperation{}, err
	}

	sql := fmt.Sprintf("UPDATE %s SET %s = %s", quoteIdent(table), quoteIdent(column), valueSQL)
	description := "backfill column " + table + "." + column
	if whenFieldIsNull {
		sql += fmt.Sprintf(" WHERE %s IS NULL", quoteIdent(column))
		description += " where " + column + " is null"
	}
	operation := reader.base(PatchOperation{
		Table:       table,
		Column:      column,
		Description: description,
		SQL:         sql,
	})
	return operation, nil
}

func (p *patchOperationPlanner) planDropField(reader patchOperationReader) (PatchOperation, error) {
	if err := reader.requireOperationFields(); err != nil {
		return PatchOperation{}, err
	}
	entityName, err := reader.requiredString("entity")
	if err != nil {
		return PatchOperation{}, err
	}
	fieldName, err := reader.requiredString("field")
	if err != nil {
		return PatchOperation{}, err
	}
	entity, err := p.entity(reader.patch.AppName, entityName)
	if err != nil {
		return PatchOperation{}, err
	}
	table, err := tableName(entity)
	if err != nil {
		return PatchOperation{}, err
	}
	column, err := p.patchFieldColumn(entity, table, fieldName)
	if err != nil {
		return PatchOperation{}, err
	}
	if isSystemColumn(column) {
		return PatchOperation{}, fmt.Errorf("drop-field cannot drop system column %q", column)
	}
	if err := p.requireLiveColumn(table, column); err != nil {
		return PatchOperation{}, err
	}

	p.dropLiveColumn(table, column)
	operation := reader.base(PatchOperation{
		Table:       table,
		Column:      column,
		Description: "drop column " + table + "." + column,
		SQL:         fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", quoteIdent(table), quoteIdent(column)),
	})
	return operation, nil
}

func (p *patchOperationPlanner) planChangeFieldType(reader patchOperationReader) (PatchOperation, error) {
	if err := reader.requireOperationFields(); err != nil {
		return PatchOperation{}, err
	}
	entityName, err := reader.requiredString("entity")
	if err != nil {
		return PatchOperation{}, err
	}
	fieldName, err := reader.requiredString("field")
	if err != nil {
		return PatchOperation{}, err
	}
	targetType, err := reader.requiredString("to")
	if err != nil {
		return PatchOperation{}, err
	}
	using, err := reader.requiredString("using")
	if err != nil {
		return PatchOperation{}, err
	}
	entity, err := p.entity(reader.patch.AppName, entityName)
	if err != nil {
		return PatchOperation{}, err
	}
	table, err := tableName(entity)
	if err != nil {
		return PatchOperation{}, err
	}
	column, field, err := p.metadataFieldColumn(entity, fieldName)
	if err != nil {
		return PatchOperation{}, err
	}
	if field.Type != targetType {
		return PatchOperation{}, fmt.Errorf("field %q metadata type is %q, patch requested %q", fieldName, field.Type, targetType)
	}
	sqlType, err := columnType(entity, field, p.targets)
	if err != nil {
		return PatchOperation{}, err
	}
	if err := p.requireLiveColumn(table, column); err != nil {
		return PatchOperation{}, err
	}

	p.changeLiveColumnType(table, column, sqlType)
	operation := reader.base(PatchOperation{
		Table:       table,
		Column:      column,
		Name:        targetType,
		Description: "change type of " + table + "." + column + " to " + sqlType,
		SQL:         fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s USING %s", quoteIdent(table), quoteIdent(column), sqlType, using),
	})
	return operation, nil
}

func (p *patchOperationPlanner) planSQL(reader patchOperationReader) (PatchOperation, error) {
	if err := reader.requireOperationFields(); err != nil {
		return PatchOperation{}, err
	}
	name, err := reader.requiredString("name")
	if err != nil {
		return PatchOperation{}, err
	}
	if _, err := reader.requiredString("reason"); err != nil {
		return PatchOperation{}, err
	}
	statement, err := reader.requiredStatement("statement")
	if err != nil {
		return PatchOperation{}, err
	}
	if err := validatePatchSQL(statement); err != nil {
		return PatchOperation{}, err
	}
	operation := reader.base(PatchOperation{
		Name:        name,
		Description: "run SQL " + name,
		SQL:         statement,
	})
	return operation, nil
}

func (p *patchOperationPlanner) entity(appName string, entityName string) (catalog.LoadedEntity, error) {
	entity, ok := p.entities[catalog.EntityKey(appName, entityName)]
	if !ok {
		return catalog.LoadedEntity{}, fmt.Errorf("entity %s/%s is not loaded", appName, entityName)
	}
	return entity, nil
}

func (p *patchOperationPlanner) metadataFieldColumn(entity catalog.LoadedEntity, fieldName string) (string, schema.Field, error) {
	field, ok := findEntityField(entity, fieldName)
	if !ok {
		return "", schema.Field{}, fmt.Errorf("field %s/%s.%s is not loaded", entity.AppName, entity.Entity.Name, fieldName)
	}
	column, err := columnForField(field)
	if err != nil {
		return "", schema.Field{}, fmt.Errorf("field %s/%s.%s has unsupported storage: %w", entity.AppName, entity.Entity.Name, fieldName, err)
	}
	return column, field, nil
}

func (p *patchOperationPlanner) patchFieldColumn(entity catalog.LoadedEntity, table string, fieldName string) (string, error) {
	if field, ok := findEntityField(entity, fieldName); ok {
		column, err := columnForField(field)
		if err != nil {
			return "", fmt.Errorf("field %s/%s.%s has unsupported storage: %w", entity.AppName, entity.Entity.Name, fieldName, err)
		}
		return column, nil
	}
	return p.inferLiveFieldColumn(table, fieldName)
}

func (p *patchOperationPlanner) inferLiveFieldColumn(table string, fieldName string) (string, error) {
	liveTable, err := p.liveTable(table)
	if err != nil {
		return "", err
	}
	base := storageName(fieldName)
	candidates := []string{base, base + "_id", base + "_hash"}
	matches := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, ok := liveTable.Columns[candidate]; ok {
			matches = append(matches, candidate)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("field %q is not in metadata and no matching live column exists on %s", fieldName, table)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("field %q is not in metadata and live column inference is ambiguous on %s: %s", fieldName, table, strings.Join(matches, ", "))
	}
}

func (p *patchOperationPlanner) requireLiveTable(table string) error {
	_, err := p.liveTable(table)
	return err
}

func (p *patchOperationPlanner) liveTable(table string) (liveTable, error) {
	tableState, ok := p.live.Tables[table]
	if !ok {
		return liveTable{}, fmt.Errorf("table %s does not exist", table)
	}
	return tableState, nil
}

func (p *patchOperationPlanner) liveTableExists(table string) bool {
	_, ok := p.live.Tables[table]
	return ok
}

func (p *patchOperationPlanner) requireLiveColumn(table string, column string) error {
	liveTable, err := p.liveTable(table)
	if err != nil {
		return err
	}
	if _, ok := liveTable.Columns[column]; !ok {
		return fmt.Errorf("column %s.%s does not exist", table, column)
	}
	return nil
}

func (p *patchOperationPlanner) liveColumnExists(table string, column string) bool {
	liveTable, ok := p.live.Tables[table]
	if !ok {
		return false
	}
	_, ok = liveTable.Columns[column]
	return ok
}

func (p *patchOperationPlanner) renameLiveColumn(table string, from string, to string) {
	liveTable := p.live.Tables[table]
	column := liveTable.Columns[from]
	delete(liveTable.Columns, from)
	column.Name = to
	liveTable.Columns[to] = column
	p.live.Tables[table] = liveTable
}

func (p *patchOperationPlanner) renameLiveTable(from string, to string) {
	liveTable := p.live.Tables[from]
	delete(p.live.Tables, from)
	liveTable.Name = to
	p.live.Tables[to] = liveTable
}

func (p *patchOperationPlanner) dropLiveColumn(table string, column string) {
	liveTable := p.live.Tables[table]
	delete(liveTable.Columns, column)
	p.live.Tables[table] = liveTable
}

func (p *patchOperationPlanner) changeLiveColumnType(table string, column string, sqlType string) {
	liveTable := p.live.Tables[table]
	liveColumn := liveTable.Columns[column]
	liveColumn.Type = sqlType
	liveTable.Columns[column] = liveColumn
	p.live.Tables[table] = liveTable
}

type patchOperationReader struct {
	patch     patches.LoadedPatch
	index     int
	operation patches.Operation
}

func (r patchOperationReader) base(operation PatchOperation) PatchOperation {
	operation.AppName = r.patch.AppName
	operation.PatchID = r.patch.Patch.ID
	operation.Phase = r.patch.Patch.Phase
	operation.Path = r.patch.Path
	operation.AppRelativePath = r.patch.AppRelativePath
	operation.Checksum = r.patch.Checksum
	operation.OperationIndex = r.index
	operation.Type = r.operation.Type
	operation.Source = r.source()
	return operation
}

func (r patchOperationReader) source() string {
	path := r.patch.AppRelativePath
	if path == "" {
		path = r.patch.Path
	}
	if path == "" {
		path = r.patch.Patch.ID
	}
	return fmt.Sprintf("patch %s/%s operation %d at %s", r.patch.AppName, r.patch.Patch.ID, r.index, path)
}

func (r patchOperationReader) requireOperationFields() error {
	spec, ok := patches.OperationSpecFor(r.operation.Type)
	if !ok {
		return fmt.Errorf("unsupported patch operation type %q", r.operation.Type)
	}
	return r.requireFields(spec.AllowedFields()...)
}

func (r patchOperationReader) requireFields(allowed ...string) error {
	allowedSet := map[string]struct{}{}
	for _, field := range allowed {
		allowedSet[field] = struct{}{}
	}
	names := make([]string, 0, len(r.operation.Fields))
	for name := range r.operation.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, ok := allowedSet[name]; !ok {
			return fmt.Errorf("unknown field %q for %s operation", name, r.operation.Type)
		}
	}
	return nil
}

func (r patchOperationReader) requiredString(name string) (string, error) {
	node, ok := r.operation.Fields[name]
	if !ok {
		return "", fmt.Errorf("%s is required", name)
	}
	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s must be a scalar string", name)
	}
	value := strings.TrimSpace(node.Value)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func (r patchOperationReader) requiredStatement(name string) (string, error) {
	node, ok := r.operation.Fields[name]
	if !ok {
		return "", fmt.Errorf("%s is required", name)
	}
	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s must be a scalar string", name)
	}
	if strings.TrimSpace(node.Value) == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return node.Value, nil
}

func (r patchOperationReader) requiredScalarSQL(name string) (string, error) {
	node, ok := r.operation.Fields[name]
	if !ok {
		return "", fmt.Errorf("%s is required", name)
	}
	value, err := scalarSQL(node)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return value, nil
}

func (r patchOperationReader) whenBool(name string) (bool, error) {
	node, ok := r.operation.Fields["when"]
	if !ok {
		return false, nil
	}
	if node.Kind != yaml.MappingNode {
		return false, fmt.Errorf("when must be a mapping")
	}
	if len(node.Content) == 0 {
		return false, fmt.Errorf("when must not be empty")
	}
	seen := false
	for i := 0; i < len(node.Content); i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			return false, fmt.Errorf("when keys must be scalar")
		}
		if key.Value != name {
			return false, fmt.Errorf("unknown when field %q", key.Value)
		}
		seen = true
		if value.Kind != yaml.ScalarNode || value.Tag != "!!bool" {
			return false, fmt.Errorf("when.%s must be a boolean", name)
		}
		enabled, err := strconv.ParseBool(value.Value)
		if err != nil {
			return false, fmt.Errorf("when.%s must be a boolean", name)
		}
		if !enabled {
			return false, fmt.Errorf("when.%s must be true when set", name)
		}
	}
	if !seen {
		return false, fmt.Errorf("when.%s is required when when is set", name)
	}
	return true, nil
}

func validatePatchSQL(statement string) error {
	tokens := sqlValidationTokens(statement)
	for index, token := range tokens {
		switch token {
		case "BEGIN", "COMMIT", "ROLLBACK", "END":
			return fmt.Errorf("sql operation cannot use transaction control %q", token)
		case "START":
			if tokenAt(tokens, index+1) == "TRANSACTION" {
				return fmt.Errorf("sql operation cannot use transaction control %q", "START TRANSACTION")
			}
		case "CREATE":
			if tokenAt(tokens, index+1) == "DATABASE" {
				return fmt.Errorf("sql operation cannot use database-level operation %q", "CREATE DATABASE")
			}
		case "DROP":
			if tokenAt(tokens, index+1) == "DATABASE" {
				return fmt.Errorf("sql operation cannot use database-level operation %q", "DROP DATABASE")
			}
		case "ALTER":
			if tokenAt(tokens, index+1) == "SYSTEM" {
				return fmt.Errorf("sql operation cannot use database-level operation %q", "ALTER SYSTEM")
			}
		}
	}
	return nil
}

func sqlValidationTokens(statement string) []string {
	stripped := stripSQLLiteralsAndComments(statement)
	fields := strings.FieldsFunc(stripped, func(r rune) bool {
		return !unicode.IsLetter(r)
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		tokens = append(tokens, strings.ToUpper(field))
	}
	return tokens
}

func stripSQLLiteralsAndComments(statement string) string {
	var b strings.Builder
	for i := 0; i < len(statement); {
		switch {
		case statement[i] == '\'':
			b.WriteByte(' ')
			i++
			for i < len(statement) {
				if statement[i] == '\'' {
					if i+1 < len(statement) && statement[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case statement[i] == '"':
			b.WriteByte(' ')
			i++
			for i < len(statement) {
				if statement[i] == '"' {
					if i+1 < len(statement) && statement[i+1] == '"' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case statement[i] == '-' && i+1 < len(statement) && statement[i+1] == '-':
			for i < len(statement) && statement[i] != '\n' {
				i++
			}
			b.WriteByte('\n')
		case statement[i] == '/' && i+1 < len(statement) && statement[i+1] == '*':
			i += 2
			for i+1 < len(statement) && !(statement[i] == '*' && statement[i+1] == '/') {
				i++
			}
			if i+1 < len(statement) {
				i += 2
			}
			b.WriteByte(' ')
		case statement[i] == '$':
			if next, ok := skipDollarQuotedLiteral(statement, i); ok {
				b.WriteByte(' ')
				i = next
				continue
			}
			b.WriteByte(statement[i])
			i++
		default:
			b.WriteByte(statement[i])
			i++
		}
	}
	return b.String()
}

func skipDollarQuotedLiteral(statement string, start int) (int, bool) {
	end := start + 1
	for end < len(statement) && isDollarQuoteTagChar(statement[end]) {
		end++
	}
	if end >= len(statement) || statement[end] != '$' {
		return start, false
	}
	delimiter := statement[start : end+1]
	closeIndex := strings.Index(statement[end+1:], delimiter)
	if closeIndex < 0 {
		return start, false
	}
	return end + 1 + closeIndex + len(delimiter), true
}

func isDollarQuoteTagChar(value byte) bool {
	return value == '_' || value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func tokenAt(tokens []string, index int) string {
	if index < 0 || index >= len(tokens) {
		return ""
	}
	return tokens[index]
}

func findEntityField(entity catalog.LoadedEntity, name string) (schema.Field, bool) {
	for _, field := range entity.Entity.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return schema.Field{}, false
}

func cloneLiveSchema(live LiveSchema) LiveSchema {
	cloned := LiveSchema{Tables: map[string]liveTable{}}
	for name, table := range live.Tables {
		clonedTable := liveTable{
			Name:          table.Name,
			RowStateKnown: table.RowStateKnown,
			HasRows:       table.HasRows,
			Columns:       map[string]liveColumn{},
			Indexes:       map[string]liveIndex{},
			Constraints:   map[string]liveConstraint{},
		}
		for columnName, column := range table.Columns {
			clonedTable.Columns[columnName] = column
		}
		for indexName, index := range table.Indexes {
			clonedTable.Indexes[indexName] = index
		}
		for constraintName, constraint := range table.Constraints {
			clonedTable.Constraints[constraintName] = constraint
		}
		cloned.Tables[name] = clonedTable
	}
	return cloned
}

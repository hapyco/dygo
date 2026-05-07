package db

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SchemaPruneBlockerHelp = "resolve blockers before pruning schema objects"

// SchemaPrunePlan describes destructive operations that remove metadata-orphaned schema objects.
type SchemaPrunePlan struct {
	Operations  []SchemaPruneOperation
	Diagnostics []SchemaDiagnostic
}

// SchemaPruneOperation is one destructive schema operation generated from live drift.
type SchemaPruneOperation struct {
	Kind        string
	Table       string
	Column      string
	Name        string
	Description string
	Source      string
	SQL         string
}

// SchemaPruneResult reports destructive prune work that was applied.
type SchemaPruneResult struct {
	Operations int
}

// HasBlockers reports whether a prune plan has non-prunable diagnostics.
func (p SchemaPrunePlan) HasBlockers() bool {
	return len(p.Diagnostics) > 0
}

// BlockerError returns an error when non-prunable diagnostics exist.
func (p SchemaPrunePlan) BlockerError() error {
	if !p.HasBlockers() {
		return nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "schema prune plan has %d blocker", len(p.Diagnostics))
	if len(p.Diagnostics) != 1 {
		b.WriteString("s")
	}
	for _, diagnostic := range p.Diagnostics {
		b.WriteString("\n")
		b.WriteString(diagnostic.String())
	}
	b.WriteString("\n")
	b.WriteString(SchemaPruneBlockerHelp)
	return errors.New(b.String())
}

// Result converts a successful prune plan into an applied result.
func (p SchemaPrunePlan) Result() SchemaPruneResult {
	return SchemaPruneResult{Operations: len(p.Operations)}
}

// PlanSchemaPrune compares discovered Entity metadata with the live database for destructive cleanup.
func PlanSchemaPrune(ctx context.Context, pool *pgxpool.Pool, root string) (SchemaPrunePlan, error) {
	metadata, err := loadMetadataCatalog(root)
	if err != nil {
		return SchemaPrunePlan{}, err
	}
	live, err := InspectLiveSchema(ctx, pool)
	if err != nil {
		return SchemaPrunePlan{}, err
	}
	return BuildSchemaPrunePlan(metadata.Entities, live)
}

// PruneMetadataSchema removes PostgreSQL schema objects that are no longer represented by Entity metadata.
func PruneMetadataSchema(ctx context.Context, pool *pgxpool.Pool, root string) (SchemaPruneResult, error) {
	plan, err := PlanSchemaPrune(ctx, pool, root)
	if err != nil {
		return SchemaPruneResult{}, err
	}
	return ApplySchemaPrunePlan(ctx, pool, plan)
}

// ApplySchemaPrunePlan applies destructive prune operations in one transaction.
func ApplySchemaPrunePlan(ctx context.Context, pool *pgxpool.Pool, plan SchemaPrunePlan) (SchemaPruneResult, error) {
	if err := plan.BlockerError(); err != nil {
		return SchemaPruneResult{}, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return SchemaPruneResult{}, fmt.Errorf("begin schema prune transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := executeSchemaPrunePlan(ctx, tx, plan); err != nil {
		return SchemaPruneResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SchemaPruneResult{}, fmt.Errorf("commit schema prune transaction: %w", err)
	}
	return plan.Result(), nil
}

// BuildSchemaPrunePlan converts metadata-orphaned live schema objects into destructive operations.
func BuildSchemaPrunePlan(entities []catalog.LoadedEntity, live LiveSchema) (SchemaPrunePlan, error) {
	desired, err := buildDesiredSchema(entities)
	if err != nil {
		return SchemaPrunePlan{}, err
	}
	metadataPlan, err := BuildMetadataSchemaPlan(entities, live)
	if err != nil {
		return SchemaPrunePlan{}, err
	}

	plan := SchemaPrunePlan{}
	for _, diagnostic := range metadataPlan.Diagnostics {
		if !isPrunableDiagnostic(diagnostic.Kind) {
			plan.Diagnostics = append(plan.Diagnostics, diagnostic)
		}
	}

	desiredTables := mapDesiredTables(desired)
	var constraints []SchemaPruneOperation
	var indexes []SchemaPruneOperation
	var columns []SchemaPruneOperation
	var tables []SchemaPruneOperation

	for _, name := range sortedDesiredTableNames(desiredTables) {
		desiredTable := desiredTables[name]
		liveTable, ok := live.Tables[name]
		if !ok {
			continue
		}
		constraints = append(constraints, pruneExtraConstraints(desiredTable, liveTable)...)
		indexes = append(indexes, pruneExtraIndexes(desiredTable, liveTable)...)
		columns = append(columns, pruneExtraColumns(desiredTable, liveTable)...)
	}

	for _, name := range sortedTableNames(live.Tables) {
		if _, ok := desiredTables[name]; ok {
			continue
		}
		tables = append(tables, SchemaPruneOperation{
			Kind:        "drop-table",
			Table:       name,
			Description: "drop table " + name,
			Source:      "database public schema",
			SQL:         fmt.Sprintf("DROP TABLE %s", quoteIdent(name)),
		})
	}

	plan.Operations = append(plan.Operations, constraints...)
	plan.Operations = append(plan.Operations, indexes...)
	plan.Operations = append(plan.Operations, columns...)
	plan.Operations = append(plan.Operations, tables...)
	return plan, nil
}

func executeSchemaPrunePlan(ctx context.Context, tx pgx.Tx, plan SchemaPrunePlan) error {
	for _, operation := range plan.Operations {
		if _, err := tx.Exec(ctx, operation.SQL); err != nil {
			return fmt.Errorf("apply schema prune operation %q: %w", operation.Description, err)
		}
	}
	return nil
}

func isPrunableDiagnostic(kind string) bool {
	switch kind {
	case "extra-constraint", "extra-index", "extra-column", "extra-table":
		return true
	default:
		return false
	}
}

func mapDesiredTables(desired desiredSchema) map[string]desiredTable {
	tables := make(map[string]desiredTable, len(desired.Tables))
	for _, table := range desired.Tables {
		tables[table.Name] = table
	}
	return tables
}

func sortedDesiredTableNames(tables map[string]desiredTable) []string {
	names := make([]string, 0, len(tables))
	for name := range tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func pruneExtraConstraints(desired desiredTable, live liveTable) []SchemaPruneOperation {
	expected := map[string]bool{}
	for _, constraint := range desired.Constraints {
		expected[constraint.Name] = true
	}
	var operations []SchemaPruneOperation
	for _, name := range sortedConstraintNames(live.Constraints) {
		constraint := live.Constraints[name]
		if constraint.Type == "primary-key" || constraint.Type == "not-null" || expected[name] {
			continue
		}
		operations = append(operations, SchemaPruneOperation{
			Kind:        "drop-constraint",
			Table:       desired.Name,
			Name:        name,
			Description: "drop constraint " + name + " on " + desired.Name,
			Source:      desired.Source,
			SQL:         fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s", quoteIdent(desired.Name), quoteIdent(name)),
		})
	}
	return operations
}

func pruneExtraIndexes(desired desiredTable, live liveTable) []SchemaPruneOperation {
	expected := map[string]bool{}
	for _, index := range desired.Indexes {
		expected[index.Name] = true
	}
	var operations []SchemaPruneOperation
	for _, name := range sortedIndexNames(live.Indexes) {
		if expected[name] || liveIndexBacksConstraint(name, live) {
			continue
		}
		operations = append(operations, SchemaPruneOperation{
			Kind:        "drop-index",
			Table:       desired.Name,
			Name:        name,
			Description: "drop index " + name + " on " + desired.Name,
			Source:      desired.Source,
			SQL:         fmt.Sprintf("DROP INDEX %s", quoteIdent(name)),
		})
	}
	return operations
}

func pruneExtraColumns(desired desiredTable, live liveTable) []SchemaPruneOperation {
	expected := map[string]bool{}
	for _, column := range desired.SystemColumns {
		expected[column.Name] = true
	}
	for _, column := range desired.Columns {
		expected[column.Name] = true
	}
	var operations []SchemaPruneOperation
	for _, name := range sortedColumnNames(live.Columns) {
		if expected[name] {
			continue
		}
		operations = append(operations, SchemaPruneOperation{
			Kind:        "drop-column",
			Table:       desired.Name,
			Column:      name,
			Description: "drop column " + desired.Name + "." + name,
			Source:      desired.Source,
			SQL:         fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", quoteIdent(desired.Name), quoteIdent(name)),
		})
	}
	return operations
}

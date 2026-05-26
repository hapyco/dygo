package db

import (
	"context"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/entity/catalog"
)

func TestBuildSchemaPrunePlanPlansExtraObjects(t *testing.T) {
	columns := map[string]liveColumn{
		"id":         {Name: "id", Type: "bigint", Nullable: false},
		"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
		"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
		"name":       {Name: "name", Type: "text", Nullable: false},
		"status":     {Name: "status", Type: "text", Nullable: false},
		"legacy":     {Name: "legacy", Type: "text", Nullable: true},
	}
	plan, err := BuildSchemaPrunePlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"app": liveSchemaTable("app", columns, map[string]liveConstraint{
			"app_pkey":         {Name: "app_pkey", Type: "primary-key"},
			"app_name_key":     {Name: "app_name_key", Type: "unique"},
			"app_status_check": {Name: "app_status_check", Type: "check"},
			"app_legacy_check": {Name: "app_legacy_check", Type: "check"},
		}, map[string]liveIndex{
			"app_pkey":       {Name: "app_pkey"},
			"app_name_key":   {Name: "app_name_key"},
			"app_legacy_idx": {Name: "app_legacy_idx"},
		}),
		"old_import": {
			Name: "old_import",
			Columns: map[string]liveColumn{
				"id": {Name: "id", Type: "bigint", Nullable: false},
			},
			Constraints: map[string]liveConstraint{},
			Indexes:     map[string]liveIndex{},
		},
	}})
	if err != nil {
		t.Fatalf("BuildSchemaPrunePlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildSchemaPrunePlan() blockers = %v, want none", plan.Diagnostics)
	}

	descriptions := pruneDescriptions(plan)
	wantDescriptions := []string{
		"drop constraint app_legacy_check on app",
		"drop index app_legacy_idx on app",
		"drop column app.legacy",
		"drop table old_import",
	}
	if strings.Join(wantDescriptions, "\n") != descriptions {
		t.Fatalf("prune descriptions = %q, want %q", descriptions, strings.Join(wantDescriptions, "\n"))
	}

	sql := pruneSQL(plan)
	assertContains(t, sql, `ALTER TABLE "app" DROP CONSTRAINT "app_legacy_check"`)
	assertContains(t, sql, `DROP INDEX "app_legacy_idx"`)
	assertContains(t, sql, `ALTER TABLE "app" DROP COLUMN "legacy"`)
	assertContains(t, sql, `DROP TABLE "old_import"`)
	if strings.Contains(sql, "CASCADE") {
		t.Fatalf("prune SQL contains CASCADE:\n%s", sql)
	}
}

func TestBuildSchemaPrunePlanDropsExtraTables(t *testing.T) {
	plan, err := BuildSchemaPrunePlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"app": liveSchemaTable("app", systemColumns(), nil, nil),
		"old_import": liveSchemaTable("old_import", systemColumns(), map[string]liveConstraint{
			"old_import_pkey": {Name: "old_import_pkey", Type: "primary-key"},
		}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildSchemaPrunePlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildSchemaPrunePlan() blockers = %v, want none", plan.Diagnostics)
	}
	if len(plan.Operations) != 1 || plan.Operations[0].Description != "drop table old_import" {
		t.Fatalf("BuildSchemaPrunePlan() operations = %v, want old_import table drop", plan.Operations)
	}
	assertContains(t, pruneSQL(plan), `DROP TABLE "old_import"`)
}

func TestBuildSchemaPrunePlanSkipsProtectedObjects(t *testing.T) {
	columns := map[string]liveColumn{
		"id":         {Name: "id", Type: "bigint", Nullable: false},
		"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
		"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
		"name":       {Name: "name", Type: "text", Nullable: false},
		"status":     {Name: "status", Type: "text", Nullable: false},
	}
	plan, err := BuildSchemaPrunePlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"app": liveSchemaTable("app", columns, map[string]liveConstraint{
			"app_pkey":          {Name: "app_pkey", Type: "primary-key"},
			"app_name_not_null": {Name: "app_name_not_null", Type: "not-null"},
			"app_name_key":      {Name: "app_name_key", Type: "unique"},
			"app_status_check":  {Name: "app_status_check", Type: "check"},
		}, map[string]liveIndex{
			"app_pkey":     {Name: "app_pkey"},
			"app_name_key": {Name: "app_name_key"},
		}),
	}})
	if err != nil {
		t.Fatalf("BuildSchemaPrunePlan() error = %v, want nil", err)
	}
	if len(plan.Operations) != 0 {
		t.Fatalf("BuildSchemaPrunePlan() operations = %v, want none", plan.Operations)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildSchemaPrunePlan() blockers = %v, want none", plan.Diagnostics)
	}
}

func TestBuildSchemaPrunePlanBlocksNonPrunableDrift(t *testing.T) {
	columns := map[string]liveColumn{
		"id":         {Name: "id", Type: "bigint", Nullable: false},
		"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
		"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
		"name":       {Name: "name", Type: "integer", Nullable: false},
		"status":     {Name: "status", Type: "text", Nullable: false},
		"legacy":     {Name: "legacy", Type: "text", Nullable: true},
	}
	plan, err := BuildSchemaPrunePlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"app": liveSchemaTable("app", columns, map[string]liveConstraint{
			"app_pkey":         {Name: "app_pkey", Type: "primary-key"},
			"app_name_key":     {Name: "app_name_key", Type: "unique"},
			"app_status_check": {Name: "app_status_check", Type: "check"},
		}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildSchemaPrunePlan() error = %v, want nil", err)
	}
	if len(plan.Operations) != 1 || plan.Operations[0].Description != "drop column app.legacy" {
		t.Fatalf("BuildSchemaPrunePlan() operations = %v, want only legacy column drop", plan.Operations)
	}
	if !plan.HasBlockers() {
		t.Fatal("BuildSchemaPrunePlan() blockers = false, want true")
	}
	errText := plan.BlockerError().Error()
	assertContains(t, errText, "column type is integer in database but metadata expects text")
	assertContains(t, errText, SchemaPruneBlockerHelp)
}

func TestBuildSchemaPrunePlanNoopsForMatchingSchema(t *testing.T) {
	plan, err := BuildSchemaPrunePlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"app": liveSchemaTable("app",
			map[string]liveColumn{
				"id":         {Name: "id", Type: "bigint", Nullable: false},
				"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
				"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
				"name":       {Name: "name", Type: "text", Nullable: false},
				"status":     {Name: "status", Type: "text", Nullable: false},
			},
			map[string]liveConstraint{
				"app_pkey":         {Name: "app_pkey", Type: "primary-key"},
				"app_name_key":     {Name: "app_name_key", Type: "unique"},
				"app_status_check": {Name: "app_status_check", Type: "check"},
			},
			nil,
		),
	}})
	if err != nil {
		t.Fatalf("BuildSchemaPrunePlan() error = %v, want nil", err)
	}
	if len(plan.Operations) != 0 {
		t.Fatalf("BuildSchemaPrunePlan() operations = %v, want none", plan.Operations)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildSchemaPrunePlan() blockers = %v, want none", plan.Diagnostics)
	}
}

func TestApplySchemaPrunePlanRejectsBlockersBeforeExecution(t *testing.T) {
	_, err := ApplySchemaPrunePlan(context.Background(), nil, SchemaPrunePlan{
		Diagnostics: []SchemaDiagnostic{
			{Classification: SchemaDiagnosticUnsafe, Table: "user", Column: "email", Message: "column type differs"},
		},
	})
	if err == nil {
		t.Fatal("ApplySchemaPrunePlan() error = nil, want blocker error")
	}
	assertContains(t, err.Error(), "schema prune plan has 1 blocker")
	assertContains(t, err.Error(), "user.email")
	assertContains(t, err.Error(), SchemaPruneBlockerHelp)
}

func pruneDescriptions(plan SchemaPrunePlan) string {
	values := make([]string, 0, len(plan.Operations))
	for _, operation := range plan.Operations {
		values = append(values, operation.Description)
	}
	return strings.Join(values, "\n")
}

func pruneSQL(plan SchemaPrunePlan) string {
	values := make([]string, 0, len(plan.Operations))
	for _, operation := range plan.Operations {
		values = append(values, operation.SQL)
	}
	return strings.Join(values, "\n")
}

package db

import (
	"context"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/dygo-dev/dygo/internal/entity/schema"
	"gopkg.in/yaml.v3"
)

func TestBuildMetadataSchemaPlanForEmptyDatabase(t *testing.T) {
	plan, err := BuildMetadataSchemaPlan(coreSchemaEntities(), LiveSchema{Tables: map[string]liveTable{}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() blockers = %v, want none", plan.Diagnostics)
	}

	descriptions := operationDescriptions(plan)
	for _, want := range []string{
		"create table apps",
		"create table entities",
		"add column apps.name",
		"add column entities.app_id",
		"create index entities_app_id_idx on entities.app_id",
		"add unique constraint apps_name_key on apps.name",
		"add check constraint apps_status_check on apps.status",
		"add foreign-key constraint entities_app_id_fkey on entities.app_id",
	} {
		assertContains(t, descriptions, want)
	}

	sql := operationSQL(plan)
	assertContains(t, sql, `CREATE TABLE "apps"`)
	assertContains(t, sql, `ALTER TABLE "apps" ADD COLUMN "name" text NOT NULL`)
	assertContains(t, sql, `CREATE INDEX "entities_app_id_idx" ON "entities" ("app_id")`)
	assertContains(t, sql, `ALTER TABLE "entities" ADD CONSTRAINT "entities_app_id_fkey" FOREIGN KEY ("app_id") REFERENCES "apps"("id") ON DELETE CASCADE`)
}

func TestBuildMetadataSchemaPlanForMatchingDatabase(t *testing.T) {
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"apps": liveSchemaTable("apps",
			map[string]liveColumn{
				"id":         {Name: "id", Type: "bigint", Nullable: false},
				"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
				"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
				"name":       {Name: "name", Type: "text", Nullable: false},
				"status":     {Name: "status", Type: "text", Nullable: false},
			},
			map[string]liveConstraint{
				"apps_pkey":          {Name: "apps_pkey", Type: "primary-key"},
				"apps_name_not_null": {Name: "apps_name_not_null", Type: "not-null"},
				"apps_name_key":      {Name: "apps_name_key", Type: "unique"},
				"apps_status_check":  {Name: "apps_status_check", Type: "check"},
			},
			nil,
		),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if len(plan.Operations) != 0 {
		t.Fatalf("BuildMetadataSchemaPlan() operations = %v, want none", plan.Operations)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
}

func TestBuildMetadataSchemaPlanAddsMissingNullableColumn(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "crm",
		Path:    "apps/crm/entities/lead.yml",
		Entity: schema.Entity{
			Name:       "lead",
			PluralName: "leads",
			Fields: []schema.Field{
				{Name: "note", Type: "text"},
			},
		},
	}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"leads": liveSchemaTable("leads", systemColumns(), map[string]liveConstraint{"leads_pkey": {Name: "leads_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
	assertContains(t, operationDescriptions(plan), "add column leads.note")
}

func TestBuildMetadataSchemaPlanRejectsMissingRequiredColumnWithoutDefault(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "crm",
		Path:    "apps/crm/entities/lead.yml",
		Entity: schema.Entity{
			Name:       "lead",
			PluralName: "leads",
			Fields: []schema.Field{
				{Name: "full-name", Type: "text", Required: true},
			},
		},
	}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"leads": liveSchemaTable("leads", systemColumns(), map[string]liveConstraint{"leads_pkey": {Name: "leads_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if !plan.HasBlockers() {
		t.Fatal("BuildMetadataSchemaPlan() blockers = false, want true")
	}
	assertContains(t, plan.BlockerError().Error(), "required column is missing and has no safe default")
}

func TestBuildMetadataSchemaPlanAddsMissingIndexAndConstraint(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "crm",
		Path:    "apps/crm/entities/lead.yml",
		Entity: schema.Entity{
			Name:       "lead",
			PluralName: "leads",
			Fields: []schema.Field{
				{Name: "email", Type: "email", Unique: true, Index: true},
			},
		},
	}
	columns := systemColumns()
	columns["email"] = liveColumn{Name: "email", Type: "text", Nullable: true}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"leads": liveSchemaTable("leads", columns, map[string]liveConstraint{"leads_pkey": {Name: "leads_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
	assertContains(t, operationDescriptions(plan), "create index leads_email_idx on leads.email")
	assertContains(t, operationDescriptions(plan), "add unique constraint leads_email_key on leads.email")
}

func TestBuildMetadataSchemaPlanAddsTopLevelIndexesAndConstraints(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "sales",
		Path:    "apps/sales/entities/deal.yml",
		Entity: schema.Entity{
			Name:       "deal",
			PluralName: "deals",
			Fields: []schema.Field{
				{Name: "company", Type: "link", Options: entityOption("company")},
				{Name: "status", Type: "text"},
				{Name: "amount", Type: "currency"},
			},
			Indexes: []schema.Index{
				{Name: "by-company-status", Fields: []string{"company", "status"}},
			},
			Constraints: []schema.Constraint{
				{Type: "unique", Fields: []string{"company", "status"}},
				{Type: "check", Field: "amount", Operator: "gte", Value: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "0"}},
			},
		},
	}
	company := catalog.LoadedEntity{
		AppName: "sales",
		Path:    "apps/sales/entities/company.yml",
		Entity: schema.Entity{
			Name:       "company",
			PluralName: "companies",
			Fields:     []schema.Field{{Name: "name", Type: "text"}},
		},
	}
	columns := systemColumns()
	columns["company_id"] = liveColumn{Name: "company_id", Type: "bigint", Nullable: true}
	columns["status"] = liveColumn{Name: "status", Type: "text", Nullable: true}
	columns["amount"] = liveColumn{Name: "amount", Type: "numeric", Nullable: true}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity, company}, LiveSchema{Tables: map[string]liveTable{
		"deals":     liveSchemaTable("deals", columns, map[string]liveConstraint{"deals_pkey": {Name: "deals_pkey", Type: "primary-key"}}, nil),
		"companies": liveSchemaTable("companies", systemColumns(), map[string]liveConstraint{"companies_pkey": {Name: "companies_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}

	descriptions := operationDescriptions(plan)
	assertContains(t, descriptions, "create index by_company_status on deals(company_id, status)")
	assertContains(t, descriptions, "add unique constraint deals_company_status_key on deals(company_id, status)")
	assertContains(t, descriptions, "add check constraint deals_amount_gte_check on deals.amount")

	sql := operationSQL(plan)
	assertContains(t, sql, `CREATE INDEX "by_company_status" ON "deals" ("company_id", "status")`)
	assertContains(t, sql, `ALTER TABLE "deals" ADD CONSTRAINT "deals_company_status_key" UNIQUE ("company_id", "status")`)
	assertContains(t, sql, `ALTER TABLE "deals" ADD CONSTRAINT "deals_amount_gte_check" CHECK ("amount" >= 0)`)
}

func TestBuildMetadataSchemaPlanReportsChangedIndexAndConstraintDefinition(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "sales",
		Path:    "apps/sales/entities/deal.yml",
		Entity: schema.Entity{
			Name:       "deal",
			PluralName: "deals",
			Fields: []schema.Field{
				{Name: "status", Type: "text"},
				{Name: "source", Type: "text"},
			},
			Indexes: []schema.Index{
				{Name: "by-status-source", Fields: []string{"status", "source"}},
			},
			Constraints: []schema.Constraint{
				{Type: "unique", Fields: []string{"status", "source"}},
			},
		},
	}
	columns := systemColumns()
	columns["status"] = liveColumn{Name: "status", Type: "text", Nullable: true}
	columns["source"] = liveColumn{Name: "source", Type: "text", Nullable: true}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"deals": liveSchemaTable("deals", columns, map[string]liveConstraint{
			"deals_pkey":              {Name: "deals_pkey", Type: "primary-key"},
			"deals_status_source_key": {Name: "deals_status_source_key", Type: "unique", Definition: "UNIQUE (status)"},
		}, map[string]liveIndex{
			"by_status_source": {Name: "by_status_source", Definition: "CREATE INDEX by_status_source ON public.deals USING btree (status)"},
		}),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if !plan.HasBlockers() {
		t.Fatal("BuildMetadataSchemaPlan() blockers = false, want true")
	}
	errText := plan.BlockerError().Error()
	assertContains(t, errText, "index \"by_status_source\" exists in database but differs from metadata")
	assertContains(t, errText, "constraint \"deals_status_source_key\" exists in database but differs from metadata")
}

func TestBuildMetadataSchemaPlanRejectsDuplicateDesiredObjectNames(t *testing.T) {
	_, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{{
		AppName: "crm",
		Path:    "apps/crm/entities/lead.yml",
		Entity: schema.Entity{
			Name:       "lead",
			PluralName: "leads",
			Fields: []schema.Field{
				{Name: "status", Type: "text", Index: true},
			},
			Indexes: []schema.Index{
				{Name: "leads-status-idx", Fields: []string{"status"}},
			},
		},
	}}, LiveSchema{Tables: map[string]liveTable{}})
	if err == nil {
		t.Fatal("BuildMetadataSchemaPlan() error = nil, want duplicate index name error")
	}
	assertContains(t, err.Error(), "duplicate index name")
}

func TestBuildMetadataSchemaPlanReportsExtraTableAndColumn(t *testing.T) {
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"apps": liveSchemaTable("apps",
			map[string]liveColumn{
				"id":         {Name: "id", Type: "bigint", Nullable: false},
				"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
				"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
				"name":       {Name: "name", Type: "text", Nullable: false},
				"status":     {Name: "status", Type: "text", Nullable: false},
				"legacy":     {Name: "legacy", Type: "text", Nullable: true},
			},
			map[string]liveConstraint{
				"apps_pkey":         {Name: "apps_pkey", Type: "primary-key"},
				"apps_name_key":     {Name: "apps_name_key", Type: "unique"},
				"apps_status_check": {Name: "apps_status_check", Type: "check"},
			},
			nil,
		),
		"legacy_tables": liveSchemaTable("legacy_tables", systemColumns(), nil, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	errText := plan.BlockerError().Error()
	assertContains(t, errText, "apps.legacy")
	assertContains(t, errText, "legacy_tables")
}

func TestBuildMetadataSchemaPlanReportsTypeDrift(t *testing.T) {
	columns := systemColumns()
	columns["name"] = liveColumn{Name: "name", Type: "integer", Nullable: false}
	columns["status"] = liveColumn{Name: "status", Type: "text", Nullable: false}

	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"apps": liveSchemaTable("apps", columns, map[string]liveConstraint{
			"apps_pkey":         {Name: "apps_pkey", Type: "primary-key"},
			"apps_name_key":     {Name: "apps_name_key", Type: "unique"},
			"apps_status_check": {Name: "apps_status_check", Type: "check"},
		}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	assertContains(t, plan.BlockerError().Error(), "column type is integer in database but metadata expects text")
}

func TestBuildMetadataSchemaPlanReportsChildTableUnsupported(t *testing.T) {
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{{
		AppName: "crm",
		Path:    "apps/crm/entities/lead.yml",
		Entity: schema.Entity{
			Name:       "lead",
			PluralName: "leads",
			Fields: []schema.Field{
				{Name: "contacts", Type: "child-table", Options: entityOption("lead-contact")},
			},
		},
	}}, LiveSchema{Tables: map[string]liveTable{}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	assertContains(t, plan.BlockerError().Error(), "child-table storage is not supported")
}

func TestBuildMetadataSchemaPlanRejectsDuplicateTableNames(t *testing.T) {
	_, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{
		{AppName: "one", Path: "apps/one/entities/user.yml", Entity: schema.Entity{Name: "user", PluralName: "users"}},
		{AppName: "two", Path: "apps/two/entities/user.yml", Entity: schema.Entity{Name: "user", PluralName: "users"}},
	}, LiveSchema{Tables: map[string]liveTable{}})
	if err == nil {
		t.Fatal("BuildMetadataSchemaPlan() error = nil, want duplicate table error")
	}
	if !strings.Contains(err.Error(), "duplicate table name") {
		t.Fatalf("BuildMetadataSchemaPlan() error = %q, want duplicate table context", err.Error())
	}
}

func TestApplyMetadataSchemaPlanRejectsBlockersBeforeExecution(t *testing.T) {
	_, err := ApplyMetadataSchemaPlan(context.Background(), nil, SchemaPlan{
		Diagnostics: []SchemaDiagnostic{
			{Classification: SchemaDiagnosticUnsafe, Table: "users", Column: "legacy", Message: "column exists in database but not metadata"},
		},
	})
	if err == nil {
		t.Fatal("ApplyMetadataSchemaPlan() error = nil, want blocker error")
	}
	assertContains(t, err.Error(), "schema plan has 1 blocker")
	assertContains(t, err.Error(), "users.legacy")
}

func coreSchemaEntities() []catalog.LoadedEntity {
	return []catalog.LoadedEntity{
		appEntity(),
		{
			AppName: "core",
			Path:    "apps/core/entities/entity.yml",
			Entity: schema.Entity{
				Name:       "entity",
				PluralName: "entities",
				Fields: []schema.Field{
					{Name: "app", Type: "link", Required: true, Index: true, Options: entityOption("app")},
					{Name: "plural-name", Type: "text", Required: true, Unique: true},
				},
			},
		},
	}
}

func appEntity() catalog.LoadedEntity {
	return catalog.LoadedEntity{
		AppName: "core",
		Path:    "apps/core/entities/app.yml",
		Entity: schema.Entity{
			Name:       "app",
			PluralName: "apps",
			Fields: []schema.Field{
				{Name: "name", Type: "text", Required: true, Unique: true},
				{Name: "status", Type: "select", Required: true, Default: stringDefault("active"), Options: options("installed", "active")},
			},
		},
	}
}

func liveSchemaTable(name string, columns map[string]liveColumn, constraints map[string]liveConstraint, indexes map[string]liveIndex) liveTable {
	if columns == nil {
		columns = map[string]liveColumn{}
	}
	if constraints == nil {
		constraints = map[string]liveConstraint{}
	}
	if indexes == nil {
		indexes = map[string]liveIndex{}
	}
	return liveTable{Name: name, Columns: columns, Constraints: constraints, Indexes: indexes}
}

func systemColumns() map[string]liveColumn {
	return map[string]liveColumn{
		"id":         {Name: "id", Type: "bigint", Nullable: false},
		"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
		"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
	}
}

func operationDescriptions(plan SchemaPlan) string {
	values := make([]string, 0, len(plan.Operations))
	for _, operation := range plan.Operations {
		values = append(values, operation.Description)
	}
	return strings.Join(values, "\n")
}

func operationSQL(plan SchemaPlan) string {
	values := make([]string, 0, len(plan.Operations))
	for _, operation := range plan.Operations {
		values = append(values, operation.SQL)
	}
	return strings.Join(values, "\n")
}

func stringDefault(value string) yaml.Node {
	return yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func entityOption(entity string) fieldtype.Options {
	return fieldtype.Options{Entity: entity}
}

func options(values ...string) fieldtype.Options {
	return fieldtype.Options{Values: values}
}

func assertContains(t *testing.T, value string, want string) {
	t.Helper()
	if !strings.Contains(value, want) {
		t.Fatalf("value does not contain %q:\n%s", want, value)
	}
}

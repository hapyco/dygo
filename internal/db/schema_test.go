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
		"create table app",
		"create table entity",
		"add column app.name",
		"add column entity.app_id",
		"create index entity_app_id_idx on entity.app_id",
		"add unique constraint app_name_key on app.name",
		"add check constraint app_status_check on app.status",
		"add foreign-key constraint entity_app_id_fkey on entity.app_id",
	} {
		assertContains(t, descriptions, want)
	}

	sql := operationSQL(plan)
	assertContains(t, sql, `CREATE TABLE "app"`)
	assertContains(t, sql, `ALTER TABLE "app" ADD COLUMN "name" text NOT NULL`)
	assertContains(t, sql, `CREATE INDEX "entity_app_id_idx" ON "entity" ("app_id")`)
	assertContains(t, sql, `ALTER TABLE "entity" ADD CONSTRAINT "entity_app_id_fkey" FOREIGN KEY ("app_id") REFERENCES "app"("id") ON DELETE CASCADE`)
}

func TestBuildMetadataSchemaPlanForMatchingDatabase(t *testing.T) {
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"app": liveSchemaTable("app",
			map[string]liveColumn{
				"id":         {Name: "id", Type: "bigint", Nullable: false},
				"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
				"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
				"name":       {Name: "name", Type: "text", Nullable: false},
				"status":     {Name: "status", Type: "text", Nullable: false},
			},
			map[string]liveConstraint{
				"app_pkey":          {Name: "app_pkey", Type: "primary-key"},
				"app_name_not_null": {Name: "app_name_not_null", Type: "not-null"},
				"app_name_key":      {Name: "app_name_key", Type: "unique"},
				"app_status_check":  {Name: "app_status_check", Type: "check"},
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
			Name: "lead",
			Fields: []schema.Field{
				{Name: "note", Type: "text"},
			},
		},
	}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"lead": liveSchemaTable("lead", systemColumns(), map[string]liveConstraint{"lead_pkey": {Name: "lead_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
	assertContains(t, operationDescriptions(plan), "add column lead.note")
}

func TestBuildMetadataSchemaPlanAddsPasswordHashColumn(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "core",
		Path:    "apps/core/entities/user.yml",
		Entity: schema.Entity{
			Name: "user",
			Fields: []schema.Field{
				{Name: "email", Type: "email", Required: true, Unique: true},
				{Name: "password", Type: "password"},
			},
		},
	}
	columns := systemColumns()
	columns["email"] = liveColumn{Name: "email", Type: "text", Nullable: false}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"user": liveSchemaTable("user", columns, map[string]liveConstraint{
			"user_pkey":      {Name: "user_pkey", Type: "primary-key"},
			"user_email_key": {Name: "user_email_key", Type: "unique"},
		}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
	assertContains(t, operationDescriptions(plan), "add column user.password_hash")
	assertContains(t, operationSQL(plan), `ALTER TABLE "user" ADD COLUMN "password_hash" text`)
}

func TestBuildMetadataSchemaPlanRejectsMissingRequiredColumnWithoutDefault(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "crm",
		Path:    "apps/crm/entities/lead.yml",
		Entity: schema.Entity{
			Name: "lead",
			Fields: []schema.Field{
				{Name: "full-name", Type: "text", Required: true},
			},
		},
	}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"lead": liveSchemaTable("lead", systemColumns(), map[string]liveConstraint{"lead_pkey": {Name: "lead_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if !plan.HasBlockers() {
		t.Fatal("BuildMetadataSchemaPlan() blockers = false, want true")
	}
	assertContains(t, plan.BlockerError().Error(), "required column is missing and has no safe default")
}

func TestBuildMetadataSchemaPlanAddsRequiredColumnForKnownEmptyTable(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "core",
		Path:    "apps/core/entities/session.yml",
		Entity: schema.Entity{
			Name: "session",
			Fields: []schema.Field{
				{Name: "token-digest", Type: "text", Required: true, Unique: true},
			},
		},
	}
	live := liveSchemaTable("session", systemColumns(), map[string]liveConstraint{"session_pkey": {Name: "session_pkey", Type: "primary-key"}}, nil)
	live.RowStateKnown = true
	live.HasRows = false
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"session": live,
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
	assertContains(t, operationDescriptions(plan), "add column session.token_digest")
	assertContains(t, operationDescriptions(plan), "add unique constraint session_token_digest_key on session.token_digest")
}

func TestBuildMetadataSchemaPlanAddsMissingIndexAndConstraint(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "crm",
		Path:    "apps/crm/entities/lead.yml",
		Entity: schema.Entity{
			Name: "lead",
			Fields: []schema.Field{
				{Name: "email", Type: "email", Unique: true, Index: true},
			},
		},
	}
	columns := systemColumns()
	columns["email"] = liveColumn{Name: "email", Type: "text", Nullable: true}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"lead": liveSchemaTable("lead", columns, map[string]liveConstraint{"lead_pkey": {Name: "lead_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
	assertContains(t, operationDescriptions(plan), "create index lead_email_idx on lead.email")
	assertContains(t, operationDescriptions(plan), "add unique constraint lead_email_key on lead.email")
}

func TestBuildMetadataSchemaPlanAddsTopLevelIndexesAndConstraints(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "sales",
		Path:    "apps/sales/entities/deal.yml",
		Entity: schema.Entity{
			Name: "deal",
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
			Name:   "company",
			Fields: []schema.Field{{Name: "name", Type: "text"}},
		},
	}
	columns := systemColumns()
	columns["company_id"] = liveColumn{Name: "company_id", Type: "bigint", Nullable: true}
	columns["status"] = liveColumn{Name: "status", Type: "text", Nullable: true}
	columns["amount"] = liveColumn{Name: "amount", Type: "numeric", Nullable: true}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity, company}, LiveSchema{Tables: map[string]liveTable{
		"deal":    liveSchemaTable("deal", columns, map[string]liveConstraint{"deal_pkey": {Name: "deal_pkey", Type: "primary-key"}}, nil),
		"company": liveSchemaTable("company", systemColumns(), map[string]liveConstraint{"company_pkey": {Name: "company_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}

	descriptions := operationDescriptions(plan)
	assertContains(t, descriptions, "create index by_company_status on deal(company_id, status)")
	assertContains(t, descriptions, "add unique constraint deal_company_status_key on deal(company_id, status)")
	assertContains(t, descriptions, "add check constraint deal_amount_gte_check on deal.amount")

	sql := operationSQL(plan)
	assertContains(t, sql, `CREATE INDEX "by_company_status" ON "deal" ("company_id", "status")`)
	assertContains(t, sql, `ALTER TABLE "deal" ADD CONSTRAINT "deal_company_status_key" UNIQUE ("company_id", "status")`)
	assertContains(t, sql, `ALTER TABLE "deal" ADD CONSTRAINT "deal_amount_gte_check" CHECK ("amount" >= 0)`)
}

func TestBuildMetadataSchemaPlanReportsChangedIndexAndConstraintDefinition(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "sales",
		Path:    "apps/sales/entities/deal.yml",
		Entity: schema.Entity{
			Name: "deal",
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
		"deal": liveSchemaTable("deal", columns, map[string]liveConstraint{
			"deal_pkey":              {Name: "deal_pkey", Type: "primary-key"},
			"deal_status_source_key": {Name: "deal_status_source_key", Type: "unique", Definition: "UNIQUE (status)"},
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
	assertContains(t, errText, "constraint \"deal_status_source_key\" exists in database but differs from metadata")
}

func TestBuildMetadataSchemaPlanRejectsDuplicateDesiredObjectNames(t *testing.T) {
	_, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{{
		AppName: "crm",
		Path:    "apps/crm/entities/lead.yml",
		Entity: schema.Entity{
			Name: "lead",
			Fields: []schema.Field{
				{Name: "status", Type: "text", Index: true},
			},
			Indexes: []schema.Index{
				{Name: "lead-status-idx", Fields: []string{"status"}},
			},
		},
	}}, LiveSchema{Tables: map[string]liveTable{}})
	if err == nil {
		t.Fatal("BuildMetadataSchemaPlan() error = nil, want duplicate index name error")
	}
	assertContains(t, err.Error(), "duplicate index name")
}

func TestBuildMetadataSchemaPlanReportsExtraTableColumnAndIndex(t *testing.T) {
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"app": liveSchemaTable("app",
			map[string]liveColumn{
				"id":         {Name: "id", Type: "bigint", Nullable: false},
				"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
				"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
				"name":       {Name: "name", Type: "text", Nullable: false},
				"status":     {Name: "status", Type: "text", Nullable: false},
				"legacy":     {Name: "legacy", Type: "text", Nullable: true},
			},
			map[string]liveConstraint{
				"app_pkey":         {Name: "app_pkey", Type: "primary-key"},
				"app_name_key":     {Name: "app_name_key", Type: "unique"},
				"app_status_check": {Name: "app_status_check", Type: "check"},
			},
			map[string]liveIndex{
				"app_name_key":   {Name: "app_name_key"},
				"app_legacy_idx": {Name: "app_legacy_idx"},
			},
		),
		"legacy_tables": liveSchemaTable("legacy_tables", systemColumns(), nil, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	errText := plan.BlockerError().Error()
	assertContains(t, errText, "app.legacy")
	assertContains(t, errText, `index "app_legacy_idx" exists in database but not metadata`)
	assertContains(t, errText, "legacy_tables")
}

func TestBuildMetadataSchemaPlanReportsTypeDrift(t *testing.T) {
	columns := systemColumns()
	columns["name"] = liveColumn{Name: "name", Type: "integer", Nullable: false}
	columns["status"] = liveColumn{Name: "status", Type: "text", Nullable: false}

	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"app": liveSchemaTable("app", columns, map[string]liveConstraint{
			"app_pkey":         {Name: "app_pkey", Type: "primary-key"},
			"app_name_key":     {Name: "app_name_key", Type: "unique"},
			"app_status_check": {Name: "app_status_check", Type: "check"},
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
			Name: "lead",
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
		{AppName: "one", Path: "apps/one/entities/user.yml", Entity: schema.Entity{Name: "user"}},
		{AppName: "two", Path: "apps/two/entities/user.yml", Entity: schema.Entity{Name: "user"}},
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
	assertContains(t, err.Error(), SchemaBlockerHelp)
}

func coreSchemaEntities() []catalog.LoadedEntity {
	return []catalog.LoadedEntity{
		appEntity(),
		{
			AppName: "core",
			Path:    "apps/core/entities/entity.yml",
			Entity: schema.Entity{
				Name: "entity",
				Fields: []schema.Field{
					{Name: "app", Type: "link", Required: true, Index: true, Options: entityOption("app")},
					{Name: "name", Type: "text", Required: true, Unique: true},
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
			Name: "app",
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

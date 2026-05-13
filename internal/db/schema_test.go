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
		"add column entity.app_id",
		"create index entity_app_id_idx on entity.app_id",
		"add unique constraint app_name_key on app.name",
		"add check constraint app_status_check on app.status",
	} {
		assertContains(t, descriptions, want)
	}

	sql := operationSQL(plan)
	assertContains(t, sql, `CREATE TABLE "app"`)
	assertContains(t, sql, `"name" text NOT NULL`)
	assertContains(t, sql, `CREATE INDEX "entity_app_id_idx" ON "entity" ("app_id")`)
	if strings.Contains(sql, "FOREIGN KEY") {
		t.Fatalf("operationSQL() contains database foreign key, want framework-level links only:\n%s", sql)
	}
}

func TestBuildMetadataSchemaPlanForMatchingDatabase(t *testing.T) {
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{appEntity()}, LiveSchema{Tables: map[string]liveTable{
		"app": liveSchemaTable("app",
			map[string]liveColumn{
				"id":         {Name: "id", Type: "bigint", Nullable: false},
				"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
				"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
				"name":       {Name: "name", Type: "text", Nullable: false},
				"status":     {Name: "status", Type: "text", Nullable: false, HasDefault: true, DefaultSQL: "'active'::text"},
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

func TestBuildMetadataSchemaPlanUpdatesColumnDefaults(t *testing.T) {
	for _, tt := range []struct {
		name       string
		columns    map[string]liveColumn
		entity     catalog.LoadedEntity
		wantSQL    string
		wantAbsent string
	}{
		{
			name: "sets missing default",
			columns: map[string]liveColumn{
				"id":         {Name: "id", Type: "bigint", Nullable: false},
				"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
				"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
				"name":       {Name: "name", Type: "text", Nullable: false},
				"status":     {Name: "status", Type: "text", Nullable: false},
			},
			entity:  appEntity(),
			wantSQL: `ALTER TABLE "app" ALTER COLUMN "status" SET DEFAULT 'active'`,
		},
		{
			name: "changes drifted default",
			columns: map[string]liveColumn{
				"id":         {Name: "id", Type: "bigint", Nullable: false},
				"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
				"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
				"name":       {Name: "name", Type: "text", Nullable: false},
				"status":     {Name: "status", Type: "text", Nullable: false, HasDefault: true, DefaultSQL: "'installed'::text"},
			},
			entity:  appEntity(),
			wantSQL: `ALTER TABLE "app" ALTER COLUMN "status" SET DEFAULT 'active'`,
		},
		{
			name: "drops extra default",
			columns: map[string]liveColumn{
				"id":         {Name: "id", Type: "bigint", Nullable: false},
				"name":       {Name: "name", Type: "text", Nullable: false},
				"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
				"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
				"note":       {Name: "note", Type: "text", Nullable: true, HasDefault: true, DefaultSQL: "'draft'::text"},
			},
			entity: catalog.LoadedEntity{
				AppName: "crm",
				Path:    "apps/crm/entities/lead.yml",
				Entity: schema.Entity{
					Name: "lead",
					Fields: []schema.Field{
						{Name: "note", Type: "text"},
					},
				},
			},
			wantSQL: `ALTER TABLE "crm_lead" ALTER COLUMN "note" DROP DEFAULT`,
		},
		{
			name: "ignores matching default",
			columns: map[string]liveColumn{
				"id":         {Name: "id", Type: "bigint", Nullable: false},
				"created_at": {Name: "created_at", Type: "timestamptz", Nullable: false},
				"updated_at": {Name: "updated_at", Type: "timestamptz", Nullable: false},
				"name":       {Name: "name", Type: "text", Nullable: false},
				"status":     {Name: "status", Type: "text", Nullable: false, HasDefault: true, DefaultSQL: "'active'::text"},
			},
			entity:     appEntity(),
			wantAbsent: `ALTER COLUMN "status"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			table := entityTableName(tt.entity.AppName, tt.entity.Entity.Name)
			plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{tt.entity}, LiveSchema{Tables: map[string]liveTable{
				table: liveSchemaTable(table, tt.columns, map[string]liveConstraint{table + "_pkey": {Name: table + "_pkey", Type: "primary-key"}}, nil),
			}})
			if err != nil {
				t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
			}
			if plan.HasBlockers() {
				t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
			}
			sql := operationSQL(plan)
			if tt.wantSQL != "" {
				assertContains(t, sql, tt.wantSQL)
			}
			if tt.wantAbsent != "" && strings.Contains(sql, tt.wantAbsent) {
				t.Fatalf("operationSQL() contains %q, want absent:\n%s", tt.wantAbsent, sql)
			}
		})
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
		"crm_lead": liveSchemaTable("crm_lead", systemColumns(), map[string]liveConstraint{"crm_lead_pkey": {Name: "crm_lead_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
	assertContains(t, operationDescriptions(plan), "add column crm_lead.note")
}

func TestBuildMetadataSchemaPlanAddsBigintColumn(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "core",
		Path:    "apps/core/entities/activity.yml",
		Entity: schema.Entity{
			Name: "activity",
			Fields: []schema.Field{
				{Name: "record-id", Type: "bigint"},
			},
		},
	}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"activity": liveSchemaTable("activity", systemColumns(), map[string]liveConstraint{"activity_pkey": {Name: "activity_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
	assertContains(t, operationDescriptions(plan), "add column activity.record_id")
	assertContains(t, operationSQL(plan), `ALTER TABLE "activity" ADD COLUMN "record_id" bigint`)
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
		"crm_lead": liveSchemaTable("crm_lead", systemColumns(), map[string]liveConstraint{"crm_lead_pkey": {Name: "crm_lead_pkey", Type: "primary-key"}}, nil),
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
		"crm_lead": liveSchemaTable("crm_lead", columns, map[string]liveConstraint{"crm_lead_pkey": {Name: "crm_lead_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
	assertContains(t, operationDescriptions(plan), "create index crm_lead_email_idx on crm_lead.email")
	assertContains(t, operationDescriptions(plan), "add unique constraint crm_lead_email_key on crm_lead.email")
}

func TestBuildMetadataSchemaPlanAddsFieldLevelCheck(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "sales",
		Path:    "apps/sales/entities/deal.yml",
		Entity: schema.Entity{
			Name: "deal",
			Fields: []schema.Field{
				{
					Name:  "amount",
					Type:  "currency",
					Check: &schema.Check{Operator: "gte", Value: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "0"}},
				},
			},
		},
	}
	columns := systemColumns()
	columns["amount"] = liveColumn{Name: "amount", Type: "numeric", Nullable: true}
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity}, LiveSchema{Tables: map[string]liveTable{
		"sales_deal": liveSchemaTable("sales_deal", columns, map[string]liveConstraint{"sales_deal_pkey": {Name: "sales_deal_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}

	descriptions := operationDescriptions(plan)
	assertContains(t, descriptions, "add check constraint sales_deal_amount_gte_check on sales_deal.amount")

	sql := operationSQL(plan)
	assertContains(t, sql, `ALTER TABLE "sales_deal" ADD CONSTRAINT "sales_deal_amount_gte_check" CHECK ("amount" >= 0)`)
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
		"sales_deal":    liveSchemaTable("sales_deal", columns, map[string]liveConstraint{"sales_deal_pkey": {Name: "sales_deal_pkey", Type: "primary-key"}}, nil),
		"sales_company": liveSchemaTable("sales_company", systemColumns(), map[string]liveConstraint{"sales_company_pkey": {Name: "sales_company_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}

	descriptions := operationDescriptions(plan)
	assertContains(t, descriptions, "create index by_company_status on sales_deal(company_id, status)")
	assertContains(t, descriptions, "add unique constraint deal_company_status_key on sales_deal(company_id, status)")
	assertContains(t, descriptions, "add check constraint deal_amount_gte_check on sales_deal.amount")

	sql := operationSQL(plan)
	assertContains(t, sql, `CREATE INDEX "by_company_status" ON "sales_deal" ("company_id", "status")`)
	assertContains(t, sql, `ALTER TABLE "sales_deal" ADD CONSTRAINT "deal_company_status_key" UNIQUE ("company_id", "status")`)
	assertContains(t, sql, `ALTER TABLE "sales_deal" ADD CONSTRAINT "deal_amount_gte_check" CHECK ("amount" >= 0)`)
}

func TestBuildMetadataSchemaPlanDoesNotCreateLinkForeignKeys(t *testing.T) {
	entity := catalog.LoadedEntity{
		AppName: "core",
		Path:    "apps/core/entities/activity.yml",
		Entity: schema.Entity{
			Name: "activity",
			Fields: []schema.Field{
				{Name: "actor", Type: "link", Options: entityOption("user")},
			},
		},
	}
	user := catalog.LoadedEntity{
		AppName: "core",
		Path:    "apps/core/entities/user.yml",
		Entity: schema.Entity{
			Name:   "user",
			Fields: []schema.Field{{Name: "email", Type: "email"}},
		},
	}
	columns := systemColumns()
	columns["actor_id"] = liveColumn{Name: "actor_id", Type: "bigint", Nullable: true}

	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{entity, user}, LiveSchema{Tables: map[string]liveTable{
		"activity": liveSchemaTable("activity", columns, map[string]liveConstraint{"activity_pkey": {Name: "activity_pkey", Type: "primary-key"}}, nil),
		"user":     liveSchemaTable("user", systemColumns(), map[string]liveConstraint{"user_pkey": {Name: "user_pkey", Type: "primary-key"}}, nil),
	}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	if plan.HasBlockers() {
		t.Fatalf("BuildMetadataSchemaPlan() diagnostics = %v, want none", plan.Diagnostics)
	}
	if strings.Contains(operationDescriptions(plan), "foreign-key") || strings.Contains(operationSQL(plan), "FOREIGN KEY") {
		t.Fatalf("BuildMetadataSchemaPlan() created database foreign key for link field: %#v", plan.Operations)
	}
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
		"sales_deal": liveSchemaTable("sales_deal", columns, map[string]liveConstraint{
			"sales_deal_pkey":        {Name: "sales_deal_pkey", Type: "primary-key"},
			"deal_status_source_key": {Name: "deal_status_source_key", Type: "unique", Definition: "UNIQUE (status)"},
		}, map[string]liveIndex{
			"by_status_source": {Name: "by_status_source", Definition: "CREATE INDEX by_status_source ON public.sales_deal USING btree (status)"},
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
				{Name: "crm-lead-status-idx", Fields: []string{"status"}},
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

func TestBuildMetadataSchemaPlanScopesNonCoreTablesByApp(t *testing.T) {
	plan, err := BuildMetadataSchemaPlan([]catalog.LoadedEntity{
		{AppName: "one", Path: "apps/one/entities/user.yml", Entity: schema.Entity{Name: "user"}},
		{AppName: "two", Path: "apps/two/entities/user.yml", Entity: schema.Entity{Name: "user"}},
	}, LiveSchema{Tables: map[string]liveTable{}})
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan() error = %v, want nil", err)
	}
	assertContains(t, operationSQL(plan), `CREATE TABLE "one_user"`)
	assertContains(t, operationSQL(plan), `CREATE TABLE "two_user"`)
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
					{Name: "name", Type: "text", Required: true, Index: true},
					{Name: "route-slug", Type: "text", Required: true, Unique: true, Index: true},
				},
				Constraints: []schema.Constraint{
					{Type: "unique", Fields: []string{"app", "name"}},
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
		"name":       {Name: "name", Type: "text", Nullable: false},
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

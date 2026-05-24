package db

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/dygo-dev/dygo/internal/entity/schema"
	"github.com/dygo-dev/dygo/internal/patches"
)

func TestBuildPatchOperationPlanPlansExactSQL(t *testing.T) {
	sqlStatement := "UPDATE \"sales_customer\"\nSET \"email\" = lower(trim(\"email\"));\n"
	tests := []struct {
		name     string
		patch    patches.LoadedPatch
		entities []catalog.LoadedEntity
		live     LiveSchema
		wantSQL  string
		wantDesc string
	}{
		{
			name: "rename field",
			patch: testLoadedPatch(t, "sales", "0001_rename_email", `  - type: rename-field
    entity: customer
    from: customer-email
    to: email
`),
			entities: []catalog.LoadedEntity{
				testEntity("sales", "customer", schema.Field{Name: "email", Type: "email"}),
			},
			live: liveWithTables("sales_customer", map[string]liveColumn{
				"customer_email": {Name: "customer_email", Type: "text", Nullable: true},
			}),
			wantSQL:  `ALTER TABLE "sales_customer" RENAME COLUMN "customer_email" TO "email"`,
			wantDesc: "rename column sales_customer.customer_email to email",
		},
		{
			name: "rename entity",
			patch: testLoadedPatch(t, "sales", "0001_rename_customer", `  - type: rename-entity
    from: customer
    to: account
`),
			entities: []catalog.LoadedEntity{
				testEntity("sales", "account", schema.Field{Name: "email", Type: "email"}),
			},
			live:     liveWithTables("sales_customer", nil),
			wantSQL:  `ALTER TABLE "sales_customer" RENAME TO "sales_account"`,
			wantDesc: "rename table sales_customer to sales_account",
		},
		{
			name: "copy field",
			patch: testLoadedPatch(t, "sales", "0001_copy_amount", `  - type: copy-field
    entity: deal
    from: legacy-amount
    to: amount
    when:
      to-is-null: true
`),
			entities: []catalog.LoadedEntity{
				testEntity("sales", "deal",
					schema.Field{Name: "legacy-amount", Type: "currency"},
					schema.Field{Name: "amount", Type: "currency"},
				),
			},
			live: liveWithTables("sales_deal", map[string]liveColumn{
				"legacy_amount": {Name: "legacy_amount", Type: "numeric", Nullable: true},
				"amount":        {Name: "amount", Type: "numeric", Nullable: true},
			}),
			wantSQL:  `UPDATE "sales_deal" SET "amount" = "legacy_amount" WHERE "amount" IS NULL`,
			wantDesc: "copy column sales_deal.legacy_amount to amount where amount is null",
		},
		{
			name: "backfill field",
			patch: testLoadedPatch(t, "sales", "0001_backfill_status", `  - type: backfill-field
    entity: deal
    field: status
    value: open
    when:
      field-is-null: true
`),
			entities: []catalog.LoadedEntity{
				testEntity("sales", "deal", schema.Field{Name: "status", Type: "select"}),
			},
			live: liveWithTables("sales_deal", map[string]liveColumn{
				"status": {Name: "status", Type: "text", Nullable: true},
			}),
			wantSQL:  `UPDATE "sales_deal" SET "status" = 'open' WHERE "status" IS NULL`,
			wantDesc: "backfill column sales_deal.status where status is null",
		},
		{
			name: "drop field",
			patch: testLoadedPatch(t, "sales", "0001_drop_legacy_status", `  - type: drop-field
    entity: customer
    field: legacy-status
`),
			entities: []catalog.LoadedEntity{
				testEntity("sales", "customer", schema.Field{Name: "email", Type: "email"}),
			},
			live: liveWithTables("sales_customer", map[string]liveColumn{
				"email":         {Name: "email", Type: "text", Nullable: true},
				"legacy_status": {Name: "legacy_status", Type: "text", Nullable: true},
			}),
			wantSQL:  `ALTER TABLE "sales_customer" DROP COLUMN "legacy_status"`,
			wantDesc: "drop column sales_customer.legacy_status",
		},
		{
			name: "change field type",
			patch: testLoadedPatch(t, "sales", "0001_change_amount_type", `  - type: change-field-type
    entity: deal
    field: amount
    to: decimal
    using: nullif(trim(amount), '')::numeric
`),
			entities: []catalog.LoadedEntity{
				testEntity("sales", "deal", schema.Field{Name: "amount", Type: "decimal"}),
			},
			live: liveWithTables("sales_deal", map[string]liveColumn{
				"amount": {Name: "amount", Type: "text", Nullable: true},
			}),
			wantSQL:  `ALTER TABLE "sales_deal" ALTER COLUMN "amount" TYPE numeric USING nullif(trim(amount), '')::numeric`,
			wantDesc: "change type of sales_deal.amount to numeric",
		},
		{
			name: "sql",
			patch: testLoadedPatch(t, "sales", "0001_normalize_emails", `  - type: sql
    name: normalize-emails
    reason: Existing emails need cleanup before a unique constraint.
    statement: |
      UPDATE "sales_customer"
      SET "email" = lower(trim("email"));
`),
			entities: nil,
			live:     LiveSchema{Tables: map[string]liveTable{}},
			wantSQL:  sqlStatement,
			wantDesc: "run SQL normalize-emails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := BuildPatchOperationPlan([]patches.LoadedPatch{tt.patch}, tt.entities, tt.live)
			if err != nil {
				t.Fatalf("BuildPatchOperationPlan() error = %v, want nil", err)
			}
			if len(plan.Operations) != 1 {
				t.Fatalf("BuildPatchOperationPlan() operations len = %d, want 1", len(plan.Operations))
			}
			operation := plan.Operations[0]
			if operation.SQL != tt.wantSQL {
				t.Fatalf("operation SQL = %q, want %q", operation.SQL, tt.wantSQL)
			}
			if operation.Description != tt.wantDesc {
				t.Fatalf("operation Description = %q, want %q", operation.Description, tt.wantDesc)
			}
			if operation.AppName != tt.patch.AppName || operation.PatchID != tt.patch.Patch.ID || operation.Phase != tt.patch.Patch.Phase || operation.Checksum != tt.patch.Checksum {
				t.Fatalf("operation patch metadata = %+v, want patch metadata from %+v", operation, tt.patch)
			}
		})
	}
}

func TestBuildPatchOperationPlanPlansSequentialShapeChanges(t *testing.T) {
	patch := testLoadedPatch(t, "sales", "0001_rename_and_backfill_email", `  - type: rename-field
    entity: customer
    from: customer-email
    to: email
  - type: backfill-field
    entity: customer
    field: email
    value: unknown@example.com
    when:
      field-is-null: true
`)
	plan, err := BuildPatchOperationPlan([]patches.LoadedPatch{patch}, []catalog.LoadedEntity{
		testEntity("sales", "customer", schema.Field{Name: "email", Type: "email"}),
	}, liveWithTables("sales_customer", map[string]liveColumn{
		"customer_email": {Name: "customer_email", Type: "text", Nullable: true},
	}))
	if err != nil {
		t.Fatalf("BuildPatchOperationPlan() error = %v, want nil", err)
	}
	gotSQL := patchOperationSQL(plan.Operations)
	wantSQL := []string{
		`ALTER TABLE "sales_customer" RENAME COLUMN "customer_email" TO "email"`,
		`UPDATE "sales_customer" SET "email" = 'unknown@example.com' WHERE "email" IS NULL`,
	}
	if !reflect.DeepEqual(gotSQL, wantSQL) {
		t.Fatalf("operation SQL = %#v, want %#v", gotSQL, wantSQL)
	}
	if plan.Operations[0].OperationIndex != 0 || plan.Operations[1].OperationIndex != 1 {
		t.Fatalf("operation indexes = %d, %d, want 0, 1", plan.Operations[0].OperationIndex, plan.Operations[1].OperationIndex)
	}
	if plan.Operations[0].Source != "patch sales/0001_rename_and_backfill_email operation 0 at patches/0001_rename_and_backfill_email.yml" {
		t.Fatalf("operation source = %q", plan.Operations[0].Source)
	}
}

func TestBuildPatchOperationPlanUsesFieldStorageRules(t *testing.T) {
	tests := []struct {
		name     string
		patch    patches.LoadedPatch
		entities []catalog.LoadedEntity
		live     LiveSchema
		wantSQL  string
	}{
		{
			name: "link field uses id suffix and infers old link column",
			patch: testLoadedPatch(t, "sales", "0001_copy_company", `  - type: copy-field
    entity: deal
    from: old-company
    to: company
`),
			entities: []catalog.LoadedEntity{
				testEntity("sales", "company", schema.Field{Name: "name", Type: "text"}),
				testEntity("sales", "deal", schema.Field{Name: "company", Type: "link", Options: fieldtype.Options{Entity: "company"}}),
			},
			live: liveWithTables("sales_deal", map[string]liveColumn{
				"old_company_id": {Name: "old_company_id", Type: "bigint", Nullable: true},
				"company_id":     {Name: "company_id", Type: "bigint", Nullable: true},
			}),
			wantSQL: `UPDATE "sales_deal" SET "company_id" = "old_company_id"`,
		},
		{
			name: "password field uses hash suffix",
			patch: testLoadedPatch(t, "core", "0001_drop_password", `  - type: drop-field
    entity: user
    field: password
`),
			entities: []catalog.LoadedEntity{
				testEntity("core", "user", schema.Field{Name: "password", Type: "password"}),
			},
			live: liveWithTables("user", map[string]liveColumn{
				"password_hash": {Name: "password_hash", Type: "text", Nullable: true},
			}),
			wantSQL: `ALTER TABLE "user" DROP COLUMN "password_hash"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := BuildPatchOperationPlan([]patches.LoadedPatch{tt.patch}, tt.entities, tt.live)
			if err != nil {
				t.Fatalf("BuildPatchOperationPlan() error = %v, want nil", err)
			}
			if len(plan.Operations) != 1 || plan.Operations[0].SQL != tt.wantSQL {
				t.Fatalf("operation SQL = %v, want %q", patchOperationSQL(plan.Operations), tt.wantSQL)
			}
		})
	}
}

func TestBuildPatchOperationPlanReportsValidationErrors(t *testing.T) {
	baseEntity := testEntity("sales", "customer", schema.Field{Name: "email", Type: "email"})
	baseLive := liveWithTables("sales_customer", map[string]liveColumn{
		"email": {Name: "email", Type: "text", Nullable: true},
	})
	tests := []struct {
		name     string
		patch    patches.LoadedPatch
		entities []catalog.LoadedEntity
		live     LiveSchema
		want     string
	}{
		{
			name: "unknown operation field",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: backfill-field
    entity: customer
    field: email
    value: test@example.com
    extra: true
`),
			entities: []catalog.LoadedEntity{baseEntity},
			live:     baseLive,
			want:     `unknown field "extra" for backfill-field operation`,
		},
		{
			name: "missing required field",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: backfill-field
    field: email
    value: test@example.com
`),
			entities: []catalog.LoadedEntity{baseEntity},
			live:     baseLive,
			want:     "entity is required",
		},
		{
			name: "non scalar field",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: backfill-field
    entity: [customer]
    field: email
    value: test@example.com
`),
			entities: []catalog.LoadedEntity{baseEntity},
			live:     baseLive,
			want:     "entity must be a scalar string",
		},
		{
			name: "invalid when",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: copy-field
    entity: customer
    from: email
    to: email
    when:
      to-is-null: false
`),
			entities: []catalog.LoadedEntity{baseEntity},
			live:     baseLive,
			want:     "when.to-is-null must be true when set",
		},
		{
			name: "missing entity metadata",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: backfill-field
    entity: lead
    field: email
    value: test@example.com
`),
			entities: []catalog.LoadedEntity{baseEntity},
			live:     baseLive,
			want:     "entity sales/lead is not loaded",
		},
		{
			name: "missing live table",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: backfill-field
    entity: customer
    field: email
    value: test@example.com
`),
			entities: []catalog.LoadedEntity{baseEntity},
			live:     LiveSchema{Tables: map[string]liveTable{}},
			want:     "table sales_customer does not exist",
		},
		{
			name: "missing live column",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: backfill-field
    entity: customer
    field: email
    value: test@example.com
`),
			entities: []catalog.LoadedEntity{baseEntity},
			live:     liveWithTables("sales_customer", nil),
			want:     "column sales_customer.email does not exist",
		},
		{
			name: "rename target column exists",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: rename-field
    entity: customer
    from: old-email
    to: email
`),
			entities: []catalog.LoadedEntity{baseEntity},
			live: liveWithTables("sales_customer", map[string]liveColumn{
				"old_email": {Name: "old_email", Type: "text", Nullable: true},
				"email":     {Name: "email", Type: "text", Nullable: true},
			}),
			want: "target column sales_customer.email already exists",
		},
		{
			name: "rename entity target table exists",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: rename-entity
    from: customer
    to: account
`),
			entities: []catalog.LoadedEntity{
				testEntity("sales", "account", schema.Field{Name: "email", Type: "email"}),
			},
			live: LiveSchema{Tables: map[string]liveTable{
				"sales_customer": liveSchemaTable("sales_customer", systemColumns(), nil, nil),
				"sales_account":  liveSchemaTable("sales_account", systemColumns(), nil, nil),
			}},
			want: "target table sales_account already exists",
		},
		{
			name: "collection storage",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: backfill-field
    entity: customer
    field: contacts
    value: x
`),
			entities: []catalog.LoadedEntity{
				testEntity("sales", "customer", schema.Field{Name: "contacts", Type: "collection"}),
			},
			live: liveWithTables("sales_customer", nil),
			want: `field type "collection" does not have direct column storage`,
		},
		{
			name: "system column drop",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: drop-field
    entity: customer
    field: id
`),
			entities: []catalog.LoadedEntity{baseEntity},
			live:     baseLive,
			want:     `drop-field cannot drop system column "id"`,
		},
		{
			name: "change type mismatch",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: change-field-type
    entity: customer
    field: email
    to: decimal
    using: email::numeric
`),
			entities: []catalog.LoadedEntity{baseEntity},
			live:     baseLive,
			want:     `field "email" metadata type is "email", patch requested "decimal"`,
		},
		{
			name: "sql missing statement",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: sql
    name: normalize
    reason: test
`),
			entities: nil,
			live:     LiveSchema{Tables: map[string]liveTable{}},
			want:     "statement is required",
		},
		{
			name: "sql transaction control",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: sql
    name: bad
    reason: test
    statement: |
      BEGIN;
      UPDATE "sales_customer" SET "email" = lower("email");
`),
			entities: nil,
			live:     LiveSchema{Tables: map[string]liveTable{}},
			want:     "transaction control",
		},
		{
			name: "sql database level operation",
			patch: testLoadedPatch(t, "sales", "0001_bad", `  - type: sql
    name: bad
    reason: test
    statement: ALTER SYSTEM SET work_mem = '64MB';
`),
			entities: nil,
			live:     LiveSchema{Tables: map[string]liveTable{}},
			want:     "database-level operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildPatchOperationPlan([]patches.LoadedPatch{tt.patch}, tt.entities, tt.live)
			if err == nil {
				t.Fatalf("BuildPatchOperationPlan() error = nil, want %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildPatchOperationPlan() error = %q, want containing %q", err.Error(), tt.want)
			}
		})
	}
}

func TestBuildPatchOperationPlanSQLValidationIgnoresLiteralsAndComments(t *testing.T) {
	patch := testLoadedPatch(t, "sales", "0001_literal_words", `  - type: sql
    name: literal-words
    reason: Transaction words in values are not transaction control.
    statement: |
      -- BEGIN in a comment is not a transaction.
      UPDATE "sales_customer"
      SET "email" = 'COMMIT@example.com'
      WHERE "name" = $$ROLLBACK$$;
`)

	plan, err := BuildPatchOperationPlan([]patches.LoadedPatch{patch}, nil, LiveSchema{Tables: map[string]liveTable{}})
	if err != nil {
		t.Fatalf("BuildPatchOperationPlan() error = %v, want nil", err)
	}
	if len(plan.Operations) != 1 || !strings.Contains(plan.Operations[0].SQL, "$$ROLLBACK$$") {
		t.Fatalf("SQL statement was not preserved: %+v", plan.Operations)
	}
}

func testLoadedPatch(t *testing.T, appName string, id string, operationsYAML string) patches.LoadedPatch {
	t.Helper()
	body := fmt.Sprintf(`kind: patch
version: 1
id: %s
phase: pre-sync
description: Test patch.
operations:
%s`, id, operationsYAML)
	patch, err := patches.Decode([]byte(body))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	return patches.LoadedPatch{
		AppName:         appName,
		AppDir:          "apps/" + appName,
		Path:            "apps/" + appName + "/patches/" + id + ".yml",
		AppRelativePath: "patches/" + id + ".yml",
		Checksum:        "sha256:" + id,
		Patch:           patch,
	}
}

func testEntity(appName string, name string, fields ...schema.Field) catalog.LoadedEntity {
	for index := range fields {
		if fields[index].Label == "" {
			fields[index].Label = fields[index].Name
		}
	}
	return catalog.LoadedEntity{
		AppName: appName,
		AppDir:  "apps/" + appName,
		Path:    "apps/" + appName + "/entities/" + name + ".yml",
		Entity: schema.Entity{
			Name:   name,
			Label:  name,
			Fields: fields,
		},
	}
}

func liveWithTables(table string, columns map[string]liveColumn) LiveSchema {
	return LiveSchema{Tables: map[string]liveTable{
		table: liveSchemaTable(table, columnsWithSystem(columns), nil, nil),
	}}
}

func columnsWithSystem(columns map[string]liveColumn) map[string]liveColumn {
	all := systemColumns()
	for name, column := range columns {
		if column.Name == "" {
			column.Name = name
		}
		all[name] = column
	}
	return all
}

func patchOperationSQL(operations []PatchOperation) []string {
	sql := make([]string, 0, len(operations))
	for _, operation := range operations {
		sql = append(sql, operation.SQL)
	}
	return sql
}

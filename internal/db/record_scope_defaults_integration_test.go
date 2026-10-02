package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/entity/schema"
	"gopkg.in/yaml.v3"
)

func TestPostgresCreateScopeUsesDatabaseDefaults(t *testing.T) {
	pool, metadata := auditDatabase(t)
	entity := auditEntity("scope-default",
		schema.Field{Name: "status", Label: "Status", Type: "text", Required: true, Default: stringDefault("draft")},
		schema.Field{Name: "enabled", Label: "Enabled", Type: "boolean", Default: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}},
		schema.Field{Name: "count", Label: "Count", Type: "int", Default: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "0"}},
		schema.Field{Name: "label", Label: "Label", Type: "text", Default: stringDefault("owner's $1")},
		schema.Field{Name: "note", Label: "Note", Type: "text"},
	)
	metadata.Entities = append(metadata.Entities, entity)
	syncAuditMetadata(t, pool, metadata)
	ctx := context.Background()
	alias := quoteIdent(recordSelectSourceAlias)
	for index, tt := range []struct {
		name, where string
		args        []any
		write       map[string]string
		input       map[string]string
		allowed     bool
	}{
		{name: "allowed effective defaults", where: fmt.Sprintf(`%s.status = $1 AND %s.enabled = false AND %s.count = 0 AND %s.label = $2`, alias, alias, alias, alias), args: []any{"draft", "owner's $1"}, write: map[string]string{"status": "FALSE"}, allowed: true},
		{name: "denied effective default", where: alias + `.status = $1`, args: []any{"approved"}, allowed: false},
		{name: "explicit denied default", where: alias + `.status = $1`, args: []any{"draft"}, write: map[string]string{"status": "FALSE"}, input: map[string]string{"status": `"draft"`}, allowed: false},
		{name: "explicit value overrides default", where: alias + `.status = $1`, args: []any{"approved"}, input: map[string]string{"status": `"approved"`}, allowed: true},
		{name: "explicit false overrides default", where: alias + `.enabled = true`, input: map[string]string{"enabled": "false"}, allowed: false},
		{name: "explicit null is not default", where: alias + `.label = $1`, args: []any{"owner's $1"}, input: map[string]string{"label": "null"}, allowed: false},
		{name: "conditional field write uses default", where: "TRUE", write: map[string]string{"note": alias + `.status = 'draft'`}, input: map[string]string{"note": `"allowed"`}, allowed: true},
		{name: "conditional field write denies default", where: "TRUE", write: map[string]string{"note": alias + `.status = 'approved'`}, input: map[string]string{"note": `"denied"`}, allowed: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := recordInput(tt.input)
			input["name"] = []byte(fmt.Sprintf(`"scope-%d"`, index))
			store := NewRecordStore(pool).WithScope(RecordScope{Where: tt.where, Args: tt.args, FieldWrite: tt.write})
			record, err := store.CreateRecordByIdentity(ctx, "audit", "scope-default", input)
			if tt.allowed {
				if err != nil {
					t.Fatal(err)
				}
				expectedStatus := "draft"
				if tt.input["status"] == `"approved"` {
					expectedStatus = "approved"
				}
				if record["status"] != expectedStatus || record["enabled"] != false || record["count"] != int32(0) || record["label"] != "owner's $1" {
					t.Fatalf("wrong effective values: %+v", record)
				}
			} else {
				var recordErr RecordError
				if !errors.As(err, &recordErr) || recordErr.Code != RecordErrorPermissionDenied {
					t.Fatalf("wanted permission denial, got %v", err)
				}
				var count int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_scope_default WHERE name=$1`, fmt.Sprintf("scope-%d", index)).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatal("denied create persisted a row")
				}
			}
			// Default authorization must not turn omitted Fields into explicit writes.
			if _, exists := input["status"]; exists && tt.input["status"] == "" {
				t.Fatal("authorization modified caller input")
			}
		})
	}
}

func TestPostgresCreateScopeUsesAuthoritativeCatalogDefaults(t *testing.T) {
	pool, metadata := auditDatabase(t)
	entity := auditEntity("precise-default",
		schema.Field{Name: "amount", Label: "Amount", Type: "decimal", Default: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: "1.000000000000000001"}},
		schema.Field{Name: "day", Label: "Day", Type: "date", Default: stringDefault("today")},
		schema.Field{Name: "enabled", Label: "Enabled", Type: "boolean", Default: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}},
		schema.Field{Name: "count", Label: "Count", Type: "int", Default: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "7"}},
		schema.Field{Name: "note", Label: "Note", Type: "text"},
	)
	metadata.Entities = append(metadata.Entities, entity)
	syncAuditMetadata(t, pool, metadata)
	ctx := context.Background()
	// Simulate a schema created on an earlier date: PostgreSQL freezes 'today'
	// during DDL, while metadata continues to contain the authored word.
	if _, err := pool.Exec(ctx, `ALTER TABLE audit_precise_default ALTER COLUMN day SET DEFAULT '2000-01-01'::date`); err != nil {
		t.Fatal(err)
	}
	alias := quoteIdent(recordSelectSourceAlias)
	for index, tt := range []struct {
		name, where, fieldWrite string
		input                   map[string]string
		allowed                 bool
	}{
		{name: "precise numeric row scope", where: alias + `.amount = 1.000000000000000001`, allowed: true},
		{name: "rounded metadata must not grant writes", where: "TRUE", fieldWrite: alias + `.amount = 1`, input: map[string]string{"note": `"denied"`}},
		{name: "frozen date row scope", where: alias + `.day = DATE '2000-01-01'`, allowed: true},
		{name: "fresh today must not grant writes", where: "TRUE", fieldWrite: alias + `.day = CURRENT_DATE`, input: map[string]string{"note": `"denied"`}},
		{name: "explicit zero false override", where: alias + `.enabled = false AND ` + alias + `.count = 0`, input: map[string]string{"enabled": "false", "count": "0"}, allowed: true},
		{name: "explicit null overrides", where: alias + `.amount IS NULL`, input: map[string]string{"amount": "null"}, allowed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := recordInput(tt.input)
			input["name"] = []byte(fmt.Sprintf(`"precise-%d"`, index))
			scope := RecordScope{Where: tt.where}
			if tt.fieldWrite != "" {
				scope.FieldWrite = map[string]string{"note": tt.fieldWrite}
			}
			record, err := NewRecordStore(pool).WithScope(scope).CreateRecordByIdentity(ctx, "audit", "precise-default", input)
			if tt.allowed {
				if err != nil {
					t.Fatal(err)
				}
				if tt.name == "explicit zero false override" && (record["enabled"] != false || record["count"] != int32(0)) {
					t.Fatalf("explicit values lost: %+v", record)
				}
			} else {
				var recordErr RecordError
				if !errors.As(err, &recordErr) || recordErr.Code != RecordErrorPermissionDenied {
					t.Fatalf("wanted denial, got %v", err)
				}
			}
		})
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE audit_precise_default DROP COLUMN amount`); err != nil {
		t.Fatal(err)
	}
	_, err := NewRecordStore(pool).WithScope(RecordScope{Where: "FALSE"}).CreateRecordByIdentity(ctx, "audit", "precise-default", recordInput(map[string]string{"name": `"missing-column"`}))
	if err == nil || !strings.Contains(err.Error(), `default column "amount" is missing`) {
		t.Fatalf("missing catalog column must fail clearly: %v", err)
	}
}

func TestPostgresDefaultScopeLockLivesUntilTransactionEnd(t *testing.T) {
	pool, metadata := auditDatabase(t)
	metadata.Entities = append(metadata.Entities, auditEntity("locked-default", schema.Field{Name: "status", Label: "Status", Type: "text", Default: stringDefault("draft")}))
	syncAuditMetadata(t, pool, metadata)
	ctx := context.Background()
	store := NewRecordStore(pool)
	layout, err := store.recordLayoutByIdentity(ctx, "audit", "locked-default")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.proposedRecordDefaults(ctx, layout, map[string]string{}); err == nil || !strings.Contains(err.Error(), "requires a database transaction") {
		t.Fatalf("nontransaction default authorization must fail: %v", err)
	}
	for _, commit := range []bool{false, true} {
		t.Run(fmt.Sprintf("commit=%t", commit), func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			txStore := NewRecordStore(tx)
			if err := txStore.proposedRecordDefaults(ctx, layout, map[string]string{}); err != nil {
				t.Fatal(err)
			}
			pid := tx.Conn().PgConn().PID()
			var locked bool
			lockQuery := `SELECT EXISTS (SELECT 1 FROM pg_locks WHERE pid=$1 AND relation='audit_locked_default'::regclass AND mode='RowExclusiveLock' AND granted)`
			if err := pool.QueryRow(ctx, lockQuery, pid).Scan(&locked); err != nil || !locked {
				t.Fatalf("transaction must hold relation lock: locked=%t error=%v", locked, err)
			}
			if commit {
				err = tx.Commit(ctx)
			} else {
				err = tx.Rollback(ctx)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, lockQuery, pid).Scan(&locked); err != nil || locked {
				t.Fatalf("transaction end must release relation lock: locked=%t error=%v", locked, err)
			}
		})
	}
}

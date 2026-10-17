package db

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/entity/schema"
)

func TestGenerateRecordNameScopesSeriesByAppAndEntity(t *testing.T) {
	queryer := &fakeRecordQueryer{row: newFakeRow(int64(1))}
	store := NewRecordStore(queryer)
	layout := recordLayout{
		EntityID: 20,
		AppName:  "crm",
		Entity:   "invoice",
		Naming: schema.Naming{
			Strategy: schema.NamingStrategySeries,
			Pattern:  "INV-{####}",
		},
	}

	name, err := store.generateRecordName(context.Background(), layout, nil)
	if err != nil {
		t.Fatalf("generateRecordName() error = %v, want nil", err)
	}
	if name != "INV-0001" {
		t.Fatalf("generateRecordName() = %q, want INV-0001", name)
	}
	wantArgs := []any{"crm/invoice:INV-{####}", int64(20), "crm/invoice:INV-{####}", "INV-{####}", "invoice:INV-{####}"}
	if len(queryer.rowArgs) != 1 || !reflect.DeepEqual(queryer.rowArgs[0], wantArgs) {
		t.Fatalf("naming series args = %#v, want %#v", queryer.rowArgs, wantArgs)
	}
	if len(queryer.rowSQL) != 1 || !strings.Contains(queryer.rowSQL[0], `legacy."key" = $5`) {
		t.Fatalf("naming series queries = %q, want legacy counter adoption", queryer.rowSQL)
	}
}

func TestLegacySeriesKeyPreservesExistingCounterIdentity(t *testing.T) {
	if got := legacySeriesKey("crm/invoice:INV-2026-{####}", "crm", "invoice"); got != "invoice:INV-2026-{####}" {
		t.Fatalf("legacySeriesKey() = %q, want legacy Entity key", got)
	}
}

func TestPostgresNamingSeriesAdoptsLegacyCounter(t *testing.T) {
	pool, metadata := auditDatabase(t)
	invoice := auditEntity("invoice")
	invoice.Entity.Naming = schema.Naming{Strategy: schema.NamingStrategySeries, Pattern: "INV-{####}"}
	metadata.Entities = append(metadata.Entities, invoice)
	syncAuditMetadata(t, pool, metadata)
	ctx := context.Background()
	store := NewRecordStoreWithHookPolicy(pool, RecordMutationHooksNone)
	layout, err := store.recordLayoutByIdentity(ctx, "audit", "invoice")
	if err != nil {
		t.Fatal(err)
	}
	legacyKey := "invoice:INV-{####}"
	if _, err := pool.Exec(ctx, `INSERT INTO naming_series (name, entity_id, key, pattern, current) VALUES ($1, $2, $1, $3, 41)`, legacyKey, layout.EntityID, "INV-{####}"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"INV-0042", "INV-0043"} {
		record, err := store.CreateRecordByIdentity(ctx, "audit", "invoice", nil)
		if err != nil {
			t.Fatal(err)
		}
		if record["name"] != want {
			t.Fatalf("created Record name = %v, want %s", record["name"], want)
		}
	}
	for key, want := range map[string]int64{legacyKey: 41, "audit/invoice:INV-{####}": 43} {
		var current int64
		if err := pool.QueryRow(ctx, `SELECT current FROM naming_series WHERE key = $1`, key).Scan(&current); err != nil {
			t.Fatal(err)
		}
		if current != want {
			t.Fatalf("naming series %q current = %d, want %d", key, current, want)
		}
	}
}

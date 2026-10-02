package db

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/internal/entity/schema"
)

func TestPostgresRandomNameCollisionRecovery(t *testing.T) {
	for _, collection := range []bool{false, true} {
		for _, outcome := range []string{"retry", "exhausted", "other-unique", "hook-rollback"} {
			t.Run(fmt.Sprintf("collection=%t/%s", collection, outcome), func(t *testing.T) {
				pool, metadata := auditDatabase(t)
				target := auditEntity("collision", schema.Field{Name: "code", Label: "Code", Type: "text", Unique: true})
				target.Entity.Naming = schema.Naming{Strategy: schema.NamingStrategyRandom, Length: 16}
				target.Entity.IsCollection = collection
				parent := auditEntity("collision-parent", schema.Field{Name: "rows", Label: "Rows", Type: "collection", Options: fieldtype.Options{Entity: "collision"}})
				metadata.Entities = append(metadata.Entities, target)
				if collection {
					metadata.Entities = append(metadata.Entities, parent)
				}
				syncAuditMetadata(t, pool, metadata)
				ctx := context.Background()
				store := NewRecordStore(pool)
				create := func(store RecordStore, name, code string) error {
					input := recordInput(map[string]string{"code": fmt.Sprintf("%q", code)})
					entity := "collision"
					if collection {
						entity = "collision-parent"
						input = recordInput(map[string]string{"name": fmt.Sprintf("%q", name), "rows": fmt.Sprintf(`[{"code":%q}]`, code)})
					}
					_, err := store.CreateRecordByIdentity(ctx, "audit", entity, input)
					return err
				}
				if err := create(store, "seed", "seed"); err != nil {
					t.Fatal(err)
				}
				var initialActivity int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM activity`).Scan(&initialActivity); err != nil {
					t.Fatal(err)
				}
				condition := "nextval('collision_attempt') = 1"
				if outcome == "exhausted" {
					condition = "nextval('collision_attempt') > 0"
				}
				if outcome == "other-unique" {
					condition = "nextval('collision_attempt') < 0"
				}
				_, err := pool.Exec(ctx, fmt.Sprintf(`CREATE SEQUENCE collision_attempt;
CREATE FUNCTION force_collision() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF %s THEN NEW.name := (SELECT name FROM audit_collision LIMIT 1); END IF; RETURN NEW; END $$;
CREATE TRIGGER collision BEFORE INSERT ON audit_collision FOR EACH ROW EXECUTE FUNCTION force_collision()`, condition))
				if err != nil {
					t.Fatal(err)
				}
				if outcome == "hook-rollback" {
					hooks := DefaultRecordHookRegistry()
					entity := "collision"
					if collection {
						entity = "collision-parent"
					}
					if err := hooks.RegisterEntity("audit", entity, RecordAfterCreate, "reject", func(context.Context, RecordHookContext) error { return errors.New("reject after insert") }); err != nil {
						t.Fatal(err)
					}
					store = NewRecordStoreWithHooks(pool, hooks)
				}
				code := "new"
				if outcome == "other-unique" {
					code = "seed"
				}
				err = create(store, "new", code)
				expectedAttempts := int64(2)
				expectedCount := 2
				switch outcome {
				case "retry":
					if err != nil {
						t.Fatal(err)
					}
				case "exhausted", "other-unique":
					expectedCount = 1
					var recordErr RecordError
					if !errors.As(err, &recordErr) || recordErr.Code != RecordErrorConstraintViolation {
						t.Fatalf("expected constraint violation, got %v", err)
					}
					constraint := "audit_collision_name_key"
					expectedAttempts = randomNameRetries + 1
					if outcome == "other-unique" {
						constraint = "audit_collision_code_key"
						expectedAttempts = 1
					}
					if recordErr.Details["constraint"] != constraint {
						t.Fatalf("wrong constraint: %v", recordErr.Details)
					}
				case "hook-rollback":
					expectedCount = 1
					if err == nil {
						t.Fatal("hook failure should abort mutation")
					}
				}
				var attempts int64
				if err := pool.QueryRow(ctx, `SELECT last_value FROM collision_attempt`).Scan(&attempts); err != nil {
					t.Fatal(err)
				}
				if attempts != expectedAttempts {
					t.Fatalf("attempts=%d, want %d", attempts, expectedAttempts)
				}
				var count int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_collision`).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != expectedCount {
					t.Fatalf("rows=%d, want %d", count, expectedCount)
				}
				if collection {
					if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_collision_parent`).Scan(&count); err != nil {
						t.Fatal(err)
					}
					if count != expectedCount {
						t.Fatalf("parents=%d, want %d", count, expectedCount)
					}
				}
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM activity`).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != initialActivity+expectedCount-1 {
					t.Fatalf("activity=%d, want %d", count, initialActivity+expectedCount-1)
				}
			})
		}
	}
}

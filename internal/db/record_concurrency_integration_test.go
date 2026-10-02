package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/hapyco/dygo/internal/entity/schema"
)

func TestPostgresMutationHooksLockCurrentSnapshot(t *testing.T) {
	for _, outcome := range []string{"update", "delete", "rollback"} {
		t.Run(outcome, func(t *testing.T) {
			pool, metadata := auditDatabase(t)
			metadata.Entities = append(metadata.Entities, auditEntity("counter", schema.Field{Name: "value", Label: "Value", Type: "int"}))
			syncAuditMetadata(t, pool, metadata)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			store := NewRecordStore(pool)
			rec, err := store.CreateRecordByIdentity(ctx, "audit", "counter", recordInput(map[string]string{"name": `"counter"`, "value": "0"}))
			if err != nil {
				t.Fatal(err)
			}
			id := rec["id"].(int64)
			entered := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			firstHooks := DefaultRecordHookRegistry()
			increment := func(h RecordHookContext) { h.Input["value"], _ = json.Marshal(h.OldRecord["value"].(int32) + 1) }
			if err := firstHooks.RegisterEntity("audit", "counter", RecordBeforeUpdate, "first", func(ctx context.Context, h RecordHookContext) error {
				increment(h)
				close(entered)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}); err != nil {
				t.Fatal(err)
			}
			if outcome == "rollback" {
				if err := firstHooks.RegisterEntity("audit", "counter", RecordAfterUpdate, "reject", func(context.Context, RecordHookContext) error { return errors.New("reject first mutation") }); err != nil {
					t.Fatal(err)
				}
			}
			firstDone := make(chan error, 1)
			go func() {
				_, err := NewRecordStoreWithHooks(pool, firstHooks).UpdateRecordByIdentity(ctx, "audit", "counter", id, recordInput(map[string]string{"value": "0"}))
				firstDone <- err
			}()
			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			// Ordinary readers remain non-blocking while the mutation owns its lock.
			if rec, err := store.GetRecordByIdentity(ctx, "audit", "counter", id); err != nil || rec["value"] != int32(0) {
				t.Fatalf("ordinary read: %v, %v", rec, err)
			}
			secondConn, err := pool.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer secondConn.Release()
			secondPID := secondConn.Conn().PgConn().PID()
			observed := make(chan int32, 1)
			secondHooks := DefaultRecordHookRegistry()
			event := RecordBeforeUpdate
			if outcome == "delete" {
				event = RecordBeforeDelete
			}
			if err := secondHooks.RegisterEntity("audit", "counter", event, "second", func(_ context.Context, h RecordHookContext) error {
				observed <- h.OldRecord["value"].(int32)
				if event == RecordBeforeUpdate {
					increment(h)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			// Include a correlated permission subquery to exercise scoped lock SQL.
			secondStore := NewRecordStoreWithHooks(secondConn, secondHooks).WithScope(RecordScope{Where: fmt.Sprintf(`EXISTS (SELECT 1 FROM audit_counter visible WHERE visible.name = %s.name)`, quoteIdent(recordSelectSourceAlias))})
			secondDone := make(chan error, 1)
			go func() {
				if outcome == "delete" {
					secondDone <- secondStore.DeleteRecordByIdentity(ctx, "audit", "counter", id)
					return
				}
				_, err := secondStore.UpdateRecordByIdentity(ctx, "audit", "counter", id, recordInput(map[string]string{"value": "0"}))
				secondDone <- err
			}()
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				var waiting bool
				if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')`, secondPID).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case err := <-secondDone:
					t.Fatalf("second mutation did not wait for snapshot lock: %v", err)
				case <-ticker.C:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
			unblock()
			firstErr := <-firstDone
			if outcome == "rollback" {
				if firstErr == nil {
					t.Fatal("first mutation should fail")
				}
			} else if firstErr != nil {
				t.Fatal(firstErr)
			}
			if err := <-secondDone; err != nil {
				t.Fatal(err)
			}
			wantOld := int32(1)
			if outcome == "rollback" {
				wantOld = 0
			}
			if old := <-observed; old != wantOld {
				t.Fatalf("second hook old value=%d, want %d", old, wantOld)
			}
			if outcome == "delete" {
				if _, err := store.GetRecordByIdentity(ctx, "audit", "counter", id); err == nil {
					t.Fatal("record should be deleted")
				}
			} else {
				rec, err := store.GetRecordByIdentity(ctx, "audit", "counter", id)
				if err != nil {
					t.Fatal(err)
				}
				if rec["value"] != wantOld+1 {
					t.Fatalf("final value=%v, want %d", rec["value"], wantOld+1)
				}
			}
			var activityCount int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM activity`).Scan(&activityCount); err != nil {
				t.Fatal(err)
			}
			wantActivity := 3
			if outcome == "rollback" {
				wantActivity = 2
			}
			if activityCount != wantActivity {
				t.Fatalf("activity count=%d, want %d", activityCount, wantActivity)
			}
		})
	}
}

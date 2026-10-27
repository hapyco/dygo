package dygodata_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hapyco/dygo/internal/actions"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/dygodata"
	"github.com/hapyco/dygo/internal/hooks"
	jobruntime "github.com/hapyco/dygo/internal/jobs/runtime"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/pkg/dygo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresSDKSystemWriterScopeAndRollback(t *testing.T) {
	url := os.Getenv("DYGO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set DYGO_TEST_DATABASE_URL to run PostgreSQL regressions")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("dygo_system_sdk_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	cfg := admin.Config().Copy()
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		pool.Close()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Error(err)
		}
	}()
	root, _ := filepath.Abs("../..")
	if _, err = db.SyncMetadataSchema(ctx, pool, root); err != nil {
		t.Fatal(err)
	}
	// Test-only system metadata keeps the production Role contract unchanged.
	if _, err = pool.Exec(ctx, `UPDATE entity SET is_system=true WHERE name='core.role'`); err != nil {
		t.Fatal(err)
	}
	input := func(name, label string) dygo.RecordInput {
		return dygo.RecordInput{"name": json.RawMessage(fmt.Sprintf("%q", name)), "label": json.RawMessage(fmt.Sprintf("%q", label))}
	}
	if _, err := dygodata.NewRecordData(pool, nil).Create(ctx, "core", "user", dygo.RecordInput{"email": json.RawMessage(`"writer@example.com"`), "full-name": json.RawMessage(`"Writer"`)}); err != nil {
		t.Fatal(err)
	}
	testRuntimeSystemWriters(t, ctx, pool)
	testSystemUpsertConcurrentConflict(t, ctx, pool)
	base := dygodata.NewRecordData(pool, nil)
	for name, view := range map[string]dygo.RecordData{"ordinary": base, "administrator": base.AsActor(dygo.Actor{Administrator: true}), "system": base.AsSystem("ordinary system access")} {
		t.Run(name, func(t *testing.T) {
			if _, err := view.Create(ctx, "core", "role", input("denied", "Denied")); err == nil || !strings.Contains(err.Error(), "trusted system Record writer") {
				t.Fatalf("ordinary system write error = %v", err)
			}
		})
	}
	for name, writer := range map[string]dygo.SystemRecordWriter{"missing scope": base.System("reason"), "empty reason": base.WithAppScope("core").System(" "), "foreign app": base.WithAppScope("sales").System("reason")} {
		t.Run(name, func(t *testing.T) {
			if _, err := writer.Create(ctx, "role", input("denied", "Denied")); err == nil {
				t.Fatal("invalid writer accepted")
			}
		})
	}
	for name, view := range map[string]dygo.RecordData{"scope": base.WithAppScope("core"), "actor": base.WithAppScope("core").AsActor(dygo.Actor{UserID: 77, Email: "writer@example.com"}), "system": base.WithAppScope("core").AsSystem("parent system reason")} {
		t.Run(name, func(t *testing.T) {
			reason := "maintain role " + name
			// An App service only needs the writer delegated by its scoped caller.
			create := func(writer dygo.SystemRecordWriter) (dygo.Record, error) {
				return writer.Create(ctx, "role", input("trusted-"+name, "Trusted"))
			}
			record, err := create(view.System(reason))
			if err != nil {
				t.Fatal(err)
			}
			id := record["id"].(int64)
			if _, err = view.System(reason).Update(ctx, "role", id, dygo.RecordInput{"label": json.RawMessage(`"Updated"`)}); err != nil {
				t.Fatal(err)
			}
			upserted, err := view.System(reason).Upsert(ctx, "role", dygo.RecordInput{"name": input("trusted-"+name, "")["name"]}, dygo.RecordInput{"label": json.RawMessage(`"Upserted"`)})
			if err != nil || upserted["id"] != id {
				t.Fatalf("upsert=%v error=%v", upserted, err)
			}
			var details string
			if err = pool.QueryRow(ctx, `SELECT details::text FROM activity WHERE record_id=$1 AND entity_id=(SELECT id FROM entity WHERE name='core.role') ORDER BY id DESC LIMIT 1`, id).Scan(&details); err != nil {
				t.Fatal(err)
			}
			if name == "actor" {
				var actor string
				if err = pool.QueryRow(ctx, `SELECT u.name FROM activity a JOIN "user" u ON u.id=a.actor_id WHERE a.record_id=$1 AND a.entity_id=(SELECT id FROM entity WHERE name='core.role') ORDER BY a.id DESC LIMIT 1`, id).Scan(&actor); err != nil || actor != "writer@example.com" {
					t.Fatalf("actor=%q error=%v", actor, err)
				}
			}
			if !strings.Contains(details, reason) {
				t.Fatalf("activity reason missing: %s", details)
			}
			if err = view.System(reason).Delete(ctx, "role", id); err != nil {
				t.Fatal(err)
			}
			sentinel := errors.New("rollback caller transaction")
			var before int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM activity`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			err = view.Transaction(ctx, func(ctx context.Context, tx dygo.RecordData) error {
				if _, err := tx.System(reason).Create(ctx, "role", input("rolled-back-"+name, "Rollback")); err != nil {
					return err
				}
				return sentinel
			})
			if !errors.Is(err, sentinel) {
				t.Fatalf("transaction = %v", err)
			}
			var after int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM activity`).Scan(&after); err != nil || after != before {
				t.Fatalf("Activity rollback before=%d after=%d error=%v", before, after, err)
			}
			var count int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM role WHERE name=$1`, "rolled-back-"+name).Scan(&count); err != nil || count != 0 {
				t.Fatalf("rollback count=%d error=%v", count, err)
			}
		})
	}
}

func testRuntimeSystemWriters(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	mutate := func(ctx context.Context, records dygo.RecordData, name string) error {
		writer := records.System("runtime " + name)
		record, err := writer.Create(ctx, "role", dygo.RecordInput{"name": json.RawMessage(fmt.Sprintf("%q", "runtime-"+name)), "label": json.RawMessage(`"Runtime"`)})
		if err != nil {
			return err
		}
		id := record["id"].(int64)
		if _, err = writer.Update(ctx, "role", id, dygo.RecordInput{"label": json.RawMessage(`"Updated"`)}); err != nil {
			return err
		}
		if _, err = writer.Upsert(ctx, "role", dygo.RecordInput{"name": json.RawMessage(fmt.Sprintf("%q", "runtime-"+name))}, dygo.RecordInput{"label": json.RawMessage(`"Upserted"`)}); err != nil {
			return err
		}
		return writer.Delete(ctx, "role", id)
	}
	calls := 0
	registry, err := hooks.NewRecordHookRegistry([]dygo.RecordHookRegistrar{func(registry dygo.RecordHookRegistry) error {
		if err := registry.RegisterEntity("core", "role", dygo.RecordBeforeCreate, "detect-recursion", func(context.Context, dygo.RecordHook) error {
			calls++
			return errors.New("system writer reentered App hook")
		}); err != nil {
			return err
		}
		return registry.RegisterEntity("core", "user", dygo.RecordBeforeCreate, "system-writer", func(ctx context.Context, hook dygo.RecordHook) error { return mutate(ctx, hook.Records, "hook") })
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = registry.Run(ctx, db.RecordHookContext{Event: db.RecordBeforeCreate, AppName: "core", Entity: "user", Queryer: pool}); err != nil {
		t.Fatal(err)
	}
	actionRegistry, err := actions.NewRegistry([]dygo.EntityActionRegistrar{func(registry dygo.EntityActionRegistry) error {
		return registry.RegisterEntity("core", "role", dygo.EntityActionDefinition{Name: "maintain", Label: "Maintain", Selection: dygo.ActionSelectionCollection}, func(ctx context.Context, call dygo.EntityActionCall) (any, error) {
			return nil, mutate(ctx, call.Records, "action")
		})
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = (actions.Executor{DB: pool, Registry: actionRegistry, RecordHooks: registry, Authorizer: allowSystemTestAction{}}).Execute(ctx, "role", "maintain", dygo.Actor{UserID: 77, Email: "writer@example.com", Administrator: true}, nil, nil); err != nil {
		t.Fatal(err)
	}
	jobs, err := jobruntime.NewRegistry([]dygo.JobRegistrar{func(registry dygo.JobRegistry) error {
		return registry.RegisterJob("core", "maintain", func(ctx context.Context, execution dygo.JobExecution) error {
			return mutate(ctx, execution.Records, "job")
		})
	}})
	if err != nil {
		t.Fatal(err)
	}
	store := &systemTestJobStore{}
	result, err := (jobruntime.Worker{Store: store, Registry: jobs, Queryer: pool, RecordHooks: registry}).Run(ctx, jobruntime.Options{Once: true, Queues: []jobruntime.Queue{{Name: "default", Concurrency: 1}}})
	if err != nil || result.Succeeded != 1 || store.failure != "" {
		t.Fatalf("Job result=%+v error=%v failure=%s", result, err, store.failure)
	}
	if calls != 0 {
		t.Fatalf("App hooks reentered %d times", calls)
	}
}

type allowSystemTestAction struct{}

func (allowSystemTestAction) Authorize(context.Context, dygo.PermissionRequest) error { return nil }

type systemTestJobStore struct {
	failure string
}

func (*systemTestJobStore) PruneTerminal(context.Context, []string, time.Time, int) (int, error) {
	return 0, nil
}

func (*systemTestJobStore) RecoverExpired(context.Context, time.Time) (int, error) { return 0, nil }
func (*systemTestJobStore) RunDueSchedules(context.Context, []string, string, time.Time, int) (int, error) {
	return 0, nil
}
func (*systemTestJobStore) Claim(context.Context, []string, int, string, time.Time) ([]jobstore.Execution, error) {
	return []jobstore.Execution{{ID: 1, AppName: "core", JobName: "maintain", Attempts: 1, Timeout: time.Minute}}, nil
}
func (*systemTestJobStore) Complete(context.Context, jobstore.Execution, json.RawMessage, time.Time) error {
	return nil
}
func (s *systemTestJobStore) Fail(_ context.Context, _ jobstore.Execution, message string, _ time.Time) error {
	s.failure = message
	return nil
}
func (*systemTestJobStore) NextRunAfter(context.Context, []string, time.Time) (*time.Time, error) {
	return nil, nil
}
func (*systemTestJobStore) NextScheduleRunAt(context.Context, []string, time.Time) (*time.Time, error) {
	return nil, nil
}
func (*systemTestJobStore) FailFinal(context.Context, jobstore.Execution, string, time.Time) error {
	return nil
}
func (*systemTestJobStore) ExpireClaim(context.Context, jobstore.Execution, time.Time) error {
	return nil
}
func (*systemTestJobStore) Enqueue(context.Context, string, string, json.RawMessage, jobstore.EnqueueOptions) (jobstore.Execution, error) {
	return jobstore.Execution{}, nil
}
func (*systemTestJobStore) CancelQueued(context.Context, string, time.Time) (jobstore.Execution, error) {
	return jobstore.Execution{}, nil
}
func (*systemTestJobStore) Retry(context.Context, string, string) (jobstore.Execution, error) {
	return jobstore.Execution{}, nil
}

func testSystemUpsertConcurrentConflict(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback(ctx)
	second, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Rollback(ctx)
	if _, err = second.Exec(ctx, `SET LOCAL application_name='system-upsert-conflict'`); err != nil {
		t.Fatal(err)
	}
	match := dygo.RecordInput{"name": json.RawMessage(`"concurrent-system-role"`)}
	input := dygo.RecordInput{"label": json.RawMessage(`"Concurrent"`)}
	if _, err = dygodata.NewRecordData(first, nil).WithAppScope("core").System("concurrent first").Upsert(ctx, "role", match, input); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := dygodata.NewRecordData(second, nil).WithAppScope("core").System("concurrent second").Upsert(ctx, "role", match, input)
		result <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var waiting bool
		if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name='system-upsert-conflict' AND wait_event='transactionid')`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("concurrent Upsert did not reach the unique constraint lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err = first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	err = <-result
	var recordErr db.RecordError
	if !errors.As(err, &recordErr) || recordErr.Code != db.RecordErrorConstraintViolation {
		t.Fatalf("concurrent Upsert error=%v, want Record constraint conflict", err)
	}
}

package imports

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/dygodata"
	jobruntime "github.com/hapyco/dygo/internal/jobs/runtime"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/pkg/dygo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresImportWorkerPersistsOutcomesWithCurrentActorPermissions(t *testing.T) {
	pool := importTestDatabase(t)
	ctx := context.Background()
	store := db.NewRecordStoreWithHookPolicy(pool, db.RecordMutationHooksNone)
	user, err := store.SystemWriter().InsertReturningByIdentity(ctx, "core", "user", db.RecordInput{
		"email":         json.RawMessage(`"import-test@example.com"`),
		"full-name":     json.RawMessage(`"Import Test"`),
		"enabled":       json.RawMessage(`true`),
		"administrator": json.RawMessage(`true`),
	}, db.SystemMutationBootstrap)
	if err != nil {
		t.Fatal(err)
	}
	actor := dygo.Actor{UserID: user["id"].(int64), Email: "import-test@example.com", Administrator: true}
	jobs, err := jobstore.New(pool)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(pool, dygodata.NewJobData(jobs), permissions.NewChecker(pool))
	registry, err := jobruntime.NewRegistry([]dygo.JobRegistrar{JobRegistrar()})
	if err != nil {
		t.Fatal(err)
	}
	worker := jobruntime.Worker{Store: jobs, Registry: registry, Queryer: pool}

	for _, tc := range []struct {
		name   string
		revoke bool
	}{
		{name: "allowed"},
		{name: "permissions-revoked-after-enqueue", revoke: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := "import-test-" + tc.name
			info, err := service.Start(ctx, actor, Target{App: "core", Entity: "role"}, strings.NewReader("name,label\n"+name+",Imported role\n"))
			if err != nil {
				t.Fatal(err)
			}
			if tc.revoke {
				if _, err := pool.Exec(ctx, `UPDATE "user" SET administrator = false WHERE id = $1`, actor.UserID); err != nil {
					t.Fatal(err)
				}
			}
			result, err := worker.Run(ctx, jobruntime.Options{Queues: []jobruntime.Queue{{Name: "default", Concurrency: 1}}, WorkerID: "import-test-worker", Once: true})
			if err != nil {
				t.Fatal(err)
			}
			if result.Claimed != 1 || result.Succeeded != 1 || result.Failed != 0 {
				t.Fatalf("worker result = %+v, want one completed processor", result)
			}
			status, err := service.Status(ctx, actor, info.ID)
			if err != nil {
				t.Fatal(err)
			}
			wantStatus, wantSucceeded, wantFailed := "succeeded", 1, 0
			if tc.revoke {
				wantStatus, wantSucceeded, wantFailed = "failed", 0, 1
			}
			if status.Status != wantStatus || status.TotalRows != 1 || status.Processed != 1 || status.Succeeded != wantSucceeded || status.Failed != wantFailed || len(status.Rows) != 1 {
				t.Fatalf("import outcome = %+v", status)
			}
			row := status.Rows[0]
			if row.Status != wantStatus || row.RowNumber != 1 {
				t.Fatalf("row outcome = %+v", row)
			}
			var records int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM "role" WHERE name = $1`, name).Scan(&records); err != nil {
				t.Fatal(err)
			}
			if records != wantSucceeded {
				t.Fatalf("created target Records = %d, want %d", records, wantSucceeded)
			}
			if tc.revoke {
				if row.RecordID != 0 || !strings.Contains(row.Error, "permission denied") {
					t.Fatalf("denied row = %+v, want permission failure without a target Record", row)
				}
			} else {
				created, err := store.GetRecordByIdentity(ctx, "core", "role", row.RecordID)
				if err != nil || created["name"] != name || row.Error != "" {
					t.Fatalf("created Record = %+v, row = %+v, error = %v", created, row, err)
				}
			}
			var executionStatus string
			if err := pool.QueryRow(ctx, `SELECT status FROM "job_execution" WHERE idempotency_key = $1`, "import:"+info.Name).Scan(&executionStatus); err != nil {
				t.Fatal(err)
			}
			if executionStatus != "succeeded" {
				t.Fatalf("processor status = %q, want succeeded", executionStatus)
			}
		})
	}
}

func importTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DYGO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set DYGO_TEST_DATABASE_URL to run PostgreSQL regressions")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("dygo_import_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config := admin.Config().Copy()
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize())
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SyncMetadataSchema(ctx, pool, root); err != nil {
		t.Fatal(err)
	}
	return pool
}

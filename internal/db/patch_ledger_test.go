package db

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPatchLedgerListPatchRuns(t *testing.T) {
	appliedAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	queryer := &fakePatchLedgerQueryer{
		rows: []pgx.Rows{newFakeRows([][]any{
			{"crm", "0001_rename_email", "apps/crm/patches/0001_rename_email.yml", PatchPhasePreSync, "sha256:a", appliedAt, "0.1.0"},
			{"sales", "0001_backfill", "apps/sales/patches/0001_backfill.yml", PatchPhasePostSync, "sha256:b", appliedAt.Add(time.Hour), ""},
		})},
	}

	runs, err := NewPatchLedger(queryer).ListPatchRuns(context.Background())
	if err != nil {
		t.Fatalf("ListPatchRuns() error = %v, want nil", err)
	}
	if len(runs) != 2 || runs[0].AppName != "crm" || runs[1].Phase != PatchPhasePostSync {
		t.Fatalf("ListPatchRuns() = %+v, want ordered patch runs", runs)
	}
	query := queryer.queries[0]
	for _, want := range []string{`FROM "patch_run" p`, `JOIN "app" a`, `ORDER BY a.name, p.patch_id`} {
		if !strings.Contains(query, want) {
			t.Fatalf("ListPatchRuns() query = %q, want %q", query, want)
		}
	}
}

func TestPatchLedgerGetPatchRun(t *testing.T) {
	appliedAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	queryer := &fakePatchLedgerQueryer{
		row: []pgx.Row{newFakeRow("crm", "0001_rename_email", "apps/crm/patches/0001_rename_email.yml", PatchPhasePreSync, "sha256:a", appliedAt, "0.1.0")},
	}

	run, err := NewPatchLedger(queryer).GetPatchRun(context.Background(), "crm", "0001_rename_email")
	if err != nil {
		t.Fatalf("GetPatchRun() error = %v, want nil", err)
	}
	if run.AppName != "crm" || run.PatchID != "0001_rename_email" || run.AppliedAt != appliedAt {
		t.Fatalf("GetPatchRun() = %+v, want crm patch run", run)
	}
	if !strings.Contains(queryer.rowSQL[0], `WHERE a.name = $1 AND p.patch_id = $2`) {
		t.Fatalf("GetPatchRun() query = %q, want app and patch id lookup", queryer.rowSQL[0])
	}
	if !reflect.DeepEqual(queryer.rowArgs[0], []any{"crm", "0001_rename_email"}) {
		t.Fatalf("GetPatchRun() args = %#v, want app and patch id", queryer.rowArgs[0])
	}
}

func TestPatchLedgerGetPatchRunNotFound(t *testing.T) {
	queryer := &fakePatchLedgerQueryer{row: []pgx.Row{fakeRow{err: pgx.ErrNoRows}}}

	_, err := NewPatchLedger(queryer).GetPatchRun(context.Background(), "crm", "missing")
	if !IsMetadataNotFound(err) {
		t.Fatalf("GetPatchRun() error = %v, want metadata not found", err)
	}
}

func TestPatchLedgerRecordPatchRunInserts(t *testing.T) {
	appliedAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	queryer := &fakePatchLedgerQueryer{
		row: []pgx.Row{
			fakeRow{err: pgx.ErrNoRows},
			newFakeRow(int64(10)),
		},
	}
	run := PatchRun{
		AppName:     "crm",
		PatchID:     "0001_rename_email",
		Path:        "apps/crm/patches/0001_rename_email.yml",
		Phase:       PatchPhasePreSync,
		Checksum:    "sha256:a",
		AppliedAt:   appliedAt,
		DygoVersion: "0.1.0",
	}

	err := NewPatchLedger(queryer).RecordPatchRun(context.Background(), run)
	if err != nil {
		t.Fatalf("RecordPatchRun() error = %v, want nil", err)
	}
	if len(queryer.execSQL) != 1 || !strings.Contains(queryer.execSQL[0], `INSERT INTO "patch_run"`) {
		t.Fatalf("RecordPatchRun() insert SQL = %#v, want patch_run insert", queryer.execSQL)
	}
	if !strings.Contains(queryer.execSQL[0], `"app_id", "applied_at", "checksum", "dygo_version", "patch_id", "path", "phase", "name"`) {
		t.Fatalf("RecordPatchRun() insert SQL = %q, want metadata-driven field columns", queryer.execSQL[0])
	}
	wantArgs := []any{int64(10), appliedAt.Format(time.RFC3339), "sha256:a", "0.1.0", "0001_rename_email", "apps/crm/patches/0001_rename_email.yml", PatchPhasePreSync, "crm.0001_rename_email"}
	if !reflect.DeepEqual(queryer.execArgs[0], wantArgs) {
		t.Fatalf("RecordPatchRun() insert args = %#v, want %#v", queryer.execArgs[0], wantArgs)
	}
}

func TestPatchLedgerRecordPatchRunRejectsAlreadyApplied(t *testing.T) {
	appliedAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	queryer := &fakePatchLedgerQueryer{
		row: []pgx.Row{newFakeRow("crm", "0001_rename_email", "apps/crm/patches/0001_rename_email.yml", PatchPhasePreSync, "sha256:a", appliedAt, "0.1.0")},
	}

	err := NewPatchLedger(queryer).RecordPatchRun(context.Background(), PatchRun{
		AppName:     "crm",
		PatchID:     "0001_rename_email",
		Path:        "apps/crm/patches/0001_rename_email.yml",
		Phase:       PatchPhasePreSync,
		Checksum:    "sha256:a",
		AppliedAt:   appliedAt,
		DygoVersion: "0.1.0",
	})
	if !IsPatchRunAlreadyApplied(err) {
		t.Fatalf("RecordPatchRun() error = %v, want already applied", err)
	}
	if len(queryer.rowSQL) != 1 {
		t.Fatalf("RecordPatchRun() row queries = %d, want no app lookup or insert after duplicate", len(queryer.rowSQL))
	}
}

func TestPatchLedgerRecordPatchRunRejectsChecksumMismatch(t *testing.T) {
	appliedAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	queryer := &fakePatchLedgerQueryer{
		row: []pgx.Row{newFakeRow("crm", "0001_rename_email", "apps/crm/patches/0001_rename_email.yml", PatchPhasePreSync, "sha256:old", appliedAt, "0.1.0")},
	}

	err := NewPatchLedger(queryer).RecordPatchRun(context.Background(), PatchRun{
		AppName:     "crm",
		PatchID:     "0001_rename_email",
		Path:        "apps/crm/patches/0001_rename_email.yml",
		Phase:       PatchPhasePreSync,
		Checksum:    "sha256:new",
		AppliedAt:   appliedAt,
		DygoVersion: "0.1.0",
	})
	if !IsPatchRunChecksumMismatch(err) {
		t.Fatalf("RecordPatchRun() error = %v, want checksum mismatch", err)
	}
	if len(queryer.rowSQL) != 1 {
		t.Fatalf("RecordPatchRun() row queries = %d, want no app lookup or insert after mismatch", len(queryer.rowSQL))
	}
}

func TestPatchLedgerRecordPatchRunRequiresExistingApp(t *testing.T) {
	queryer := &fakePatchLedgerQueryer{
		row: []pgx.Row{
			fakeRow{err: pgx.ErrNoRows},
			fakeRow{err: pgx.ErrNoRows},
		},
	}

	err := NewPatchLedger(queryer).RecordPatchRun(context.Background(), PatchRun{
		AppName:  "missing",
		PatchID:  "0001_patch",
		Path:     "apps/missing/patches/0001_patch.yml",
		Phase:    PatchPhasePreSync,
		Checksum: "sha256:a",
	})
	if !IsMetadataNotFound(err) {
		t.Fatalf("RecordPatchRun() error = %v, want missing app", err)
	}
}

func TestPatchLedgerRecordPatchRunValidatesRequiredFields(t *testing.T) {
	valid := PatchRun{
		AppName:  "crm",
		PatchID:  "0001_patch",
		Path:     "apps/crm/patches/0001_patch.yml",
		Phase:    PatchPhasePreSync,
		Checksum: "sha256:a",
	}
	tests := []struct {
		name string
		run  PatchRun
		want string
	}{
		{name: "app", run: patchRunWith(valid, func(run *PatchRun) { run.AppName = "" }), want: "app is required"},
		{name: "patch id", run: patchRunWith(valid, func(run *PatchRun) { run.PatchID = "" }), want: "id is required"},
		{name: "path", run: patchRunWith(valid, func(run *PatchRun) { run.Path = "" }), want: "path is required"},
		{name: "phase", run: patchRunWith(valid, func(run *PatchRun) { run.Phase = "during-sync" }), want: "phase must be"},
		{name: "checksum", run: patchRunWith(valid, func(run *PatchRun) { run.Checksum = "" }), want: "checksum is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewPatchLedger(&fakePatchLedgerQueryer{}).RecordPatchRun(context.Background(), tt.run)
			if err == nil {
				t.Fatal("RecordPatchRun() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("RecordPatchRun() error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func patchRunWith(run PatchRun, mutate func(*PatchRun)) PatchRun {
	mutate(&run)
	return run
}

type fakePatchLedgerQueryer struct {
	rows []pgx.Rows
	row  []pgx.Row

	queries  []string
	args     [][]any
	rowSQL   []string
	rowArgs  [][]any
	execSQL  []string
	execArgs [][]any
}

func (q *fakePatchLedgerQueryer) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.queries = append(q.queries, sql)
	q.args = append(q.args, args)
	if rows, ok := fakePatchRunMetadataRows(sql, args...); ok {
		return rows, nil
	}
	if len(q.rows) == 0 {
		return newFakeRows(nil), nil
	}
	rows := q.rows[0]
	q.rows = q.rows[1:]
	return rows, nil
}

func (q *fakePatchLedgerQueryer) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.rowSQL = append(q.rowSQL, sql)
	q.rowArgs = append(q.rowArgs, args)
	if isPatchRunMetadataQuery(sql, args...) {
		return newFakeRow(int64(2), "core.patch-run", "patch-run", "patch-run", "Patch Run", "Ledger entry", "git-pull-request-arrow", false, false, []byte(`{"strategy":"template","template":"{app}.{patch-id}"}`), "core", "Core")
	}
	if strings.Contains(sql, `SELECT "name" FROM "app"`) && len(args) == 1 && args[0] == int64(10) {
		return newFakeRow("crm")
	}
	if len(q.row) == 0 {
		return fakeRow{err: pgx.ErrNoRows}
	}
	row := q.row[0]
	q.row = q.row[1:]
	return row
}

func (q *fakePatchLedgerQueryer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execSQL = append(q.execSQL, sql)
	q.execArgs = append(q.execArgs, args)
	return pgconn.NewCommandTag("INSERT 1"), nil
}

func isPatchRunMetadataQuery(sql string, args ...any) bool {
	return strings.Contains(sql, `WHERE a.name = $1 AND e.key = $2`) &&
		len(args) == 2 &&
		args[0] == "core" &&
		args[1] == "patch-run"
}

func fakePatchRunMetadataRows(sql string, args ...any) (pgx.Rows, bool) {
	if len(args) != 1 || args[0] != int64(2) {
		return nil, false
	}
	switch {
	case strings.Contains(sql, `FROM "field"`):
		return newFakeRows([][]any{
			{"app", "App", "link", true, false, true, nil, nil, 1, []byte(`{"entity":"app"}`)},
			{"patch-id", "Patch ID", "text", true, false, true, nil, nil, 2, nil},
			{"path", "Path", "text", true, false, false, nil, nil, 3, nil},
			{"phase", "Phase", "select", true, false, true, nil, nil, 4, []byte(`{"values":["pre-sync","post-sync"]}`)},
			{"checksum", "Checksum", "text", true, false, false, nil, nil, 5, nil},
			{"applied-at", "Applied At", "datetime", true, false, false, nil, nil, 6, nil},
			{"dygo-version", "dygo Version", "text", false, false, false, nil, nil, 7, nil},
		}), true
	case strings.Contains(sql, `FROM "index"`), strings.Contains(sql, `FROM "constraint"`):
		return newFakeRows(nil), true
	default:
		return nil, false
	}
}

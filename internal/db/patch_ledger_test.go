package db

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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
			newFakeRow(int64(20)),
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
	if !strings.Contains(queryer.rowSQL[2], `INSERT INTO "patch_run"`) || !strings.Contains(queryer.rowSQL[2], `RETURNING id`) {
		t.Fatalf("RecordPatchRun() insert SQL = %q, want insert returning id", queryer.rowSQL[2])
	}
	wantArgs := []any{"crm/0001_rename_email", int64(10), "0001_rename_email", "apps/crm/patches/0001_rename_email.yml", PatchPhasePreSync, "sha256:a", appliedAt, "0.1.0"}
	if !reflect.DeepEqual(queryer.rowArgs[2], wantArgs) {
		t.Fatalf("RecordPatchRun() insert args = %#v, want %#v", queryer.rowArgs[2], wantArgs)
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

	queries []string
	args    [][]any
	rowSQL  []string
	rowArgs [][]any
}

func (q *fakePatchLedgerQueryer) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.queries = append(q.queries, sql)
	q.args = append(q.args, args)
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
	if len(q.row) == 0 {
		return fakeRow{err: pgx.ErrNoRows}
	}
	row := q.row[0]
	q.row = q.row[1:]
	return row
}

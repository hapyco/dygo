package db

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/schema"
	"github.com/dygo-dev/dygo/internal/patches"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestApplyPatchPlanAppliesPatchesInTransactions(t *testing.T) {
	root := t.TempDir()
	plan := PatchPlan{
		Phase: PatchPhasePreSync,
		Pending: []PlannedPatch{
			{
				AppName:  "sales",
				PatchID:  "0001_rename_email",
				Phase:    PatchPhasePreSync,
				Path:     filepath.Join(root, "apps", "sales", "patches", "0001_rename_email.yml"),
				Checksum: "sha256:a",
				Operations: []PatchOperation{
					{OperationIndex: 0, Type: PatchOperationRenameField, SQL: `ALTER TABLE "sales_customer" RENAME COLUMN "customer_email" TO "email"`},
					{OperationIndex: 1, Type: PatchOperationBackfillField, SQL: `UPDATE "sales_customer" SET "email" = lower("email")`},
				},
			},
			{
				AppName:  "support",
				PatchID:  "0002_cleanup",
				Phase:    PatchPhasePreSync,
				Path:     filepath.Join(root, "apps", "support", "patches", "0002_cleanup.yml"),
				Checksum: "sha256:b",
				Operations: []PatchOperation{
					{OperationIndex: 0, Type: PatchOperationSQL, SQL: `SELECT 1`},
				},
			},
		},
	}
	beginner := &fakePatchApplyBeginner{}

	result, err := ApplyPatchPlan(context.Background(), beginner, plan, root, "dev")
	if err != nil {
		t.Fatalf("ApplyPatchPlan() error = %v, want nil", err)
	}
	if len(beginner.txs) != 2 {
		t.Fatalf("transactions = %d, want 2", len(beginner.txs))
	}
	if !beginner.txs[0].committed || !beginner.txs[1].committed {
		t.Fatalf("transactions committed = %t/%t, want true/true", beginner.txs[0].committed, beginner.txs[1].committed)
	}
	wantSQL := []string{
		`ALTER TABLE "sales_customer" RENAME COLUMN "customer_email" TO "email"`,
		`UPDATE "sales_customer" SET "email" = lower("email")`,
	}
	if !reflect.DeepEqual(beginner.txs[0].execSQL, wantSQL) {
		t.Fatalf("first transaction SQL = %#v, want %#v", beginner.txs[0].execSQL, wantSQL)
	}
	if !containsEvent(beginner.txs[0].events, "exec:patch-run") {
		t.Fatalf("first transaction events = %#v, want patch-run ledger write", beginner.txs[0].events)
	}
	if len(result.Applied) != 2 || result.Applied[0].Path != "apps/sales/patches/0001_rename_email.yml" || result.Applied[0].DygoVersion != "dev" {
		t.Fatalf("ApplyPatchPlan() result = %+v, want two applied repo-relative patch runs", result)
	}
	if result.Applied[0].AppliedAt.IsZero() {
		t.Fatal("ApplyPatchPlan() AppliedAt is zero, want timestamp")
	}
}

func containsEvent(events []string, want string) bool {
	for _, event := range events {
		if event == want {
			return true
		}
	}
	return false
}

func TestApplyPatchPlanRollsBackOperationFailure(t *testing.T) {
	plan := PatchPlan{
		Phase: PatchPhasePreSync,
		Pending: []PlannedPatch{{
			AppName:  "sales",
			PatchID:  "0001_bad",
			Phase:    PatchPhasePreSync,
			Path:     "apps/sales/patches/0001_bad.yml",
			Checksum: "sha256:a",
			Operations: []PatchOperation{{
				OperationIndex: 0,
				Type:           PatchOperationSQL,
				SQL:            `SELECT broken`,
			}},
		}},
	}
	beginner := &fakePatchApplyBeginner{execErrs: []error{errors.New("syntax error")}}

	result, err := ApplyPatchPlan(context.Background(), beginner, plan, "", "dev")
	if err == nil {
		t.Fatal("ApplyPatchPlan() error = nil, want operation error")
	}
	if !strings.Contains(err.Error(), "apply patch sales/0001_bad operation 0 sql") {
		t.Fatalf("ApplyPatchPlan() error = %q, want patch operation context", err.Error())
	}
	if len(result.Applied) != 0 {
		t.Fatalf("ApplyPatchPlan() applied = %+v, want none", result.Applied)
	}
	if len(beginner.txs) != 1 || !beginner.txs[0].rolledBack || beginner.txs[0].committed {
		t.Fatalf("transaction state = %+v, want rollback without commit", beginner.txs)
	}
	if len(beginner.txs[0].rowSQL) != 0 {
		t.Fatalf("ledger queries = %d, want none after operation failure", len(beginner.txs[0].rowSQL))
	}
}

func TestApplyPatchPlanRollsBackLedgerFailure(t *testing.T) {
	plan := PatchPlan{
		Phase: PatchPhasePreSync,
		Pending: []PlannedPatch{{
			AppName:  "sales",
			PatchID:  "0001_bad_ledger",
			Phase:    PatchPhasePreSync,
			Path:     "apps/sales/patches/0001_bad_ledger.yml",
			Checksum: "sha256:a",
			Operations: []PatchOperation{{
				OperationIndex: 0,
				Type:           PatchOperationSQL,
				SQL:            `SELECT 1`,
			}},
		}},
	}
	beginner := &fakePatchApplyBeginner{insertErr: errors.New("insert failed")}

	_, err := ApplyPatchPlan(context.Background(), beginner, plan, "", "dev")
	if err == nil {
		t.Fatal("ApplyPatchPlan() error = nil, want ledger error")
	}
	if !strings.Contains(err.Error(), "record patch sales/0001_bad_ledger") {
		t.Fatalf("ApplyPatchPlan() error = %q, want record context", err.Error())
	}
	if len(beginner.txs) != 1 || !beginner.txs[0].rolledBack || beginner.txs[0].committed {
		t.Fatalf("transaction state = %+v, want rollback without commit", beginner.txs)
	}
}

func TestApplyPatchPlanStopsAfterFirstFailure(t *testing.T) {
	plan := PatchPlan{
		Phase: PatchPhasePreSync,
		Pending: []PlannedPatch{
			{
				AppName:  "sales",
				PatchID:  "0001_bad",
				Phase:    PatchPhasePreSync,
				Path:     "apps/sales/patches/0001_bad.yml",
				Checksum: "sha256:a",
				Operations: []PatchOperation{{
					OperationIndex: 0,
					Type:           PatchOperationSQL,
					SQL:            `SELECT broken`,
				}},
			},
			{
				AppName:  "sales",
				PatchID:  "0002_later",
				Phase:    PatchPhasePreSync,
				Path:     "apps/sales/patches/0002_later.yml",
				Checksum: "sha256:b",
				Operations: []PatchOperation{{
					OperationIndex: 0,
					Type:           PatchOperationSQL,
					SQL:            `SELECT 1`,
				}},
			},
		},
	}
	beginner := &fakePatchApplyBeginner{execErrs: []error{errors.New("syntax error")}}

	_, err := ApplyPatchPlan(context.Background(), beginner, plan, "", "dev")
	if err == nil {
		t.Fatal("ApplyPatchPlan() error = nil, want first patch error")
	}
	if len(beginner.txs) != 1 {
		t.Fatalf("transactions = %d, want stop after first failure", len(beginner.txs))
	}
}

func TestApplyPatchPlanNoPendingOpensNoTransaction(t *testing.T) {
	beginner := &fakePatchApplyBeginner{}
	result, err := ApplyPatchPlan(context.Background(), beginner, PatchPlan{Phase: PatchPhasePostSync}, "", "dev")
	if err != nil {
		t.Fatalf("ApplyPatchPlan() error = %v, want nil", err)
	}
	if len(beginner.txs) != 0 {
		t.Fatalf("transactions = %d, want none", len(beginner.txs))
	}
	if result.Phase != PatchPhasePostSync || len(result.Applied) != 0 {
		t.Fatalf("ApplyPatchPlan() result = %+v, want empty post-sync result", result)
	}
}

func TestMigratorApplyPatchPlanDumpsSchemaAfterSuccessfulApply(t *testing.T) {
	root := t.TempDir()
	snapshotter := &fakePatchSnapshotter{}
	migrator := Migrator{Snapshotter: snapshotter}
	plan := PatchPlan{
		Phase: PatchPhasePreSync,
		Pending: []PlannedPatch{{
			AppName:  "sales",
			PatchID:  "0001_patch",
			Phase:    PatchPhasePreSync,
			Path:     filepath.Join(root, "apps", "sales", "patches", "0001_patch.yml"),
			Checksum: "sha256:a",
			Operations: []PatchOperation{{
				OperationIndex: 0,
				Type:           PatchOperationSQL,
				SQL:            `SELECT 1`,
			}},
		}},
	}

	result, err := migrator.applyPatchPlan(context.Background(), &fakePatchApplyBeginner{}, plan, root, "postgres://user:secret@localhost/dygo", "dev")
	if err != nil {
		t.Fatalf("applyPatchPlan() error = %v, want nil", err)
	}
	if len(result.Applied) != 1 {
		t.Fatalf("applyPatchPlan() applied = %+v, want one patch", result.Applied)
	}
	if snapshotter.calls != 1 || snapshotter.root != root {
		t.Fatalf("snapshotter calls/root = %d/%q, want 1/%q", snapshotter.calls, snapshotter.root, root)
	}
}

func TestMigratorApplyPatchPlanSkipsSchemaDumpForNoop(t *testing.T) {
	snapshotter := &fakePatchSnapshotter{}
	migrator := Migrator{Snapshotter: snapshotter}
	result, err := migrator.applyPatchPlan(context.Background(), &fakePatchApplyBeginner{}, PatchPlan{Phase: PatchPhasePostSync}, "/repo", "postgres://user:secret@localhost/dygo", "dev")
	if err != nil {
		t.Fatalf("applyPatchPlan() error = %v, want nil", err)
	}
	if len(result.Applied) != 0 {
		t.Fatalf("applyPatchPlan() applied = %+v, want none", result.Applied)
	}
	if snapshotter.calls != 0 {
		t.Fatalf("snapshotter calls = %d, want 0", snapshotter.calls)
	}
}

func TestPatchRunnerPreSyncRenameWorkflow(t *testing.T) {
	desired := []catalog.LoadedEntity{
		testEntity("sales", "customer", schema.Field{Name: "email", Type: "email"}),
	}
	oldLive := liveWithTables("sales_customer", map[string]liveColumn{
		"customer_email": {Name: "customer_email", Type: "text", Nullable: true},
	})
	metadataPlan, err := BuildMetadataSchemaPlan(desired, oldLive)
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan(old) error = %v, want nil", err)
	}
	if !containsDiagnosticKind(metadataPlan.Diagnostics, "extra-column") {
		t.Fatalf("metadata diagnostics = %+v, want extra-column before rename patch", metadataPlan.Diagnostics)
	}

	patch := testLoadedPatch(t, "sales", "0001_rename_email", `  - type: rename-field
    entity: customer
    from: customer-email
    to: email
`)
	patchPlan, err := BuildPatchPlan([]patches.LoadedPatch{patch}, desired, oldLive, nil, PatchPhasePreSync)
	if err != nil {
		t.Fatalf("BuildPatchPlan() error = %v, want nil", err)
	}
	beginner := &fakePatchApplyBeginner{}
	if _, err := ApplyPatchPlan(context.Background(), beginner, patchPlan, "", "dev"); err != nil {
		t.Fatalf("ApplyPatchPlan() error = %v, want nil", err)
	}
	if len(beginner.txs) != 1 || beginner.txs[0].execSQL[0] != `ALTER TABLE "sales_customer" RENAME COLUMN "customer_email" TO "email"` {
		t.Fatalf("patch SQL = %+v, want rename SQL", beginner.txs)
	}

	newLive := liveWithTables("sales_customer", map[string]liveColumn{
		"email": {Name: "email", Type: "text", Nullable: true},
	})
	afterPlan, err := BuildMetadataSchemaPlan(desired, newLive)
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan(new) error = %v, want nil", err)
	}
	if len(afterPlan.Diagnostics) != 0 {
		t.Fatalf("metadata diagnostics after patch = %+v, want none", afterPlan.Diagnostics)
	}
}

func TestPatchRunnerPostSyncBackfillWorkflow(t *testing.T) {
	desired := []catalog.LoadedEntity{
		testEntity("sales", "deal", schema.Field{Name: "status", Type: "select"}),
	}
	beforeSync := liveWithTables("sales_deal", nil)
	metadataPlan, err := BuildMetadataSchemaPlan(desired, beforeSync)
	if err != nil {
		t.Fatalf("BuildMetadataSchemaPlan(before sync) error = %v, want nil", err)
	}
	if !containsOperation(metadataPlan.Operations, "add column sales_deal.status") {
		t.Fatalf("metadata operations = %+v, want status add before post-sync patch", metadataPlan.Operations)
	}

	afterSync := liveWithTables("sales_deal", map[string]liveColumn{
		"status": {Name: "status", Type: "text", Nullable: true},
	})
	patch := testLoadedPatch(t, "sales", "0001_backfill_status", `  - type: backfill-field
    entity: deal
    field: status
    value: open
    when:
      field-is-null: true
`)
	patch.Patch.Phase = PatchPhasePostSync
	patchPlan, err := BuildPatchPlan([]patches.LoadedPatch{patch}, desired, afterSync, nil, PatchPhasePostSync)
	if err != nil {
		t.Fatalf("BuildPatchPlan() error = %v, want nil", err)
	}
	beginner := &fakePatchApplyBeginner{}
	if _, err := ApplyPatchPlan(context.Background(), beginner, patchPlan, "", "dev"); err != nil {
		t.Fatalf("ApplyPatchPlan() error = %v, want nil", err)
	}
	if len(beginner.txs) != 1 || beginner.txs[0].execSQL[0] != `UPDATE "sales_deal" SET "status" = 'open' WHERE "status" IS NULL` {
		t.Fatalf("patch SQL = %+v, want backfill SQL", beginner.txs)
	}
}

func containsDiagnosticKind(diagnostics []SchemaDiagnostic, kind string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind {
			return true
		}
	}
	return false
}

func containsOperation(operations []SchemaOperation, description string) bool {
	for _, operation := range operations {
		if operation.Description == description {
			return true
		}
	}
	return false
}

type fakePatchApplyBeginner struct {
	beginErr  error
	execErrs  []error
	insertErr error
	commitErr error
	txs       []*fakePatchApplyTx
}

func (b *fakePatchApplyBeginner) Begin(context.Context) (pgx.Tx, error) {
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	tx := &fakePatchApplyTx{
		execErrs:  b.execErrs,
		insertErr: b.insertErr,
		commitErr: b.commitErr,
	}
	b.execErrs = nil
	b.txs = append(b.txs, tx)
	return tx, nil
}

type fakePatchApplyTx struct {
	execSQL    []string
	execErrs   []error
	insertErr  error
	commitErr  error
	rowSQL     []string
	rowArgs    [][]any
	events     []string
	committed  bool
	rolledBack bool
}

func (tx *fakePatchApplyTx) Begin(context.Context) (pgx.Tx, error) {
	return &fakePatchApplyTx{}, nil
}

func (tx *fakePatchApplyTx) Commit(context.Context) error {
	tx.events = append(tx.events, "commit")
	if tx.commitErr != nil {
		return tx.commitErr
	}
	tx.committed = true
	return nil
}

func (tx *fakePatchApplyTx) Rollback(context.Context) error {
	tx.events = append(tx.events, "rollback")
	tx.rolledBack = true
	return nil
}

func (tx *fakePatchApplyTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (tx *fakePatchApplyTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakePatchApplyTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakePatchApplyTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}

func (tx *fakePatchApplyTx) Conn() *pgx.Conn {
	return nil
}

func (tx *fakePatchApplyTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if strings.Contains(sql, `INSERT INTO "patch_run"`) {
		tx.events = append(tx.events, "exec:patch-run")
		if tx.insertErr != nil {
			return pgconn.CommandTag{}, tx.insertErr
		}
		return pgconn.NewCommandTag("INSERT 1"), nil
	}
	tx.events = append(tx.events, "exec")
	tx.execSQL = append(tx.execSQL, sql)
	if len(tx.execErrs) > 0 {
		err := tx.execErrs[0]
		tx.execErrs = tx.execErrs[1:]
		if err != nil {
			return pgconn.CommandTag{}, err
		}
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

func (tx *fakePatchApplyTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	if rows, ok := fakePatchRunMetadataRows(sql, args...); ok {
		switch {
		case strings.Contains(sql, `FROM "field"`):
			tx.events = append(tx.events, "query:fields")
		case strings.Contains(sql, `FROM "index"`):
			tx.events = append(tx.events, "query:indexes")
		case strings.Contains(sql, `FROM "constraint"`):
			tx.events = append(tx.events, "query:constraints")
		}
		return rows, nil
	}
	return newFakeRows(nil), nil
}

func (tx *fakePatchApplyTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	tx.rowSQL = append(tx.rowSQL, sql)
	tx.rowArgs = append(tx.rowArgs, args)
	switch {
	case strings.Contains(sql, `WHERE a.name = $1 AND p.patch_id = $2`):
		tx.events = append(tx.events, "queryrow:get")
		return fakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sql, `SELECT id FROM "app"`):
		tx.events = append(tx.events, "queryrow:app")
		return newFakeRow(int64(10))
	case isPatchRunMetadataQuery(sql, args...):
		tx.events = append(tx.events, "queryrow:metadata")
		return newFakeRow(int64(2), "core.patch-run", "patch-run", "patch-run", "Patch Run", "Ledger entry", "git-pull-request-arrow", false, false, []byte(`{"strategy":"template","template":"{app}.{patch-id}"}`), "core", "Core")
	case strings.Contains(sql, `SELECT "name" FROM "app"`) && len(args) == 1 && args[0] == int64(10):
		tx.events = append(tx.events, "queryrow:link")
		return newFakeRow("sales")
	default:
		return fakeRow{err: pgx.ErrNoRows}
	}
}

type fakePatchSnapshotter struct {
	calls       int
	root        string
	databaseURL string
	err         error
}

func (s *fakePatchSnapshotter) Dump(_ context.Context, root string, databaseURL string) error {
	s.calls++
	s.root = root
	s.databaseURL = databaseURL
	return s.err
}

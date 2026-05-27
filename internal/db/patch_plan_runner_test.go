package db

import (
	"strings"
	"testing"
	"time"

	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/schema"
	"github.com/hapyco/dygo/internal/patches"
)

func TestBuildPatchPlanSplitsPendingAndApplied(t *testing.T) {
	applied := testLoadedPatch(t, "sales", "0000_applied", `  - type: sql
    name: applied-patch
    reason: Already handled.
    statement: SELECT 1;
`)
	pending := testLoadedPatch(t, "sales", "0001_rename_email", `  - type: rename-field
    entity: customer
    from: customer-email
    to: email
`)
	appliedAt := time.Date(2026, 5, 15, 10, 30, 0, 0, time.UTC)

	plan, err := BuildPatchPlan(
		[]patches.LoadedPatch{applied, pending},
		[]catalog.LoadedEntity{testEntity("sales", "customer", schema.Field{Name: "email", Type: "email"})},
		liveWithTables("sales_customer", map[string]liveColumn{
			"customer_email": {Name: "customer_email", Type: "text", Nullable: true},
		}),
		[]PatchRun{{
			AppName:     "sales",
			PatchID:     "0000_applied",
			Path:        applied.AppRelativePath,
			Phase:       PatchPhasePreSync,
			Checksum:    applied.Checksum,
			AppliedAt:   appliedAt,
			DygoVersion: "dev",
		}},
		PatchPhasePreSync,
	)
	if err != nil {
		t.Fatalf("BuildPatchPlan() error = %v, want nil", err)
	}
	if plan.Phase != PatchPhasePreSync {
		t.Fatalf("plan phase = %q, want %q", plan.Phase, PatchPhasePreSync)
	}
	if len(plan.Applied) != 1 || plan.Applied[0].PatchID != "0000_applied" || !plan.Applied[0].Run.AppliedAt.Equal(appliedAt) {
		t.Fatalf("applied patches = %+v, want applied patch with run", plan.Applied)
	}
	if len(plan.Pending) != 1 || plan.Pending[0].PatchID != "0001_rename_email" {
		t.Fatalf("pending patches = %+v, want pending rename patch", plan.Pending)
	}
	if len(plan.Pending[0].Operations) != 1 {
		t.Fatalf("pending operations len = %d, want 1", len(plan.Pending[0].Operations))
	}
	if plan.Pending[0].Operations[0].SQL != `ALTER TABLE "sales_customer" RENAME COLUMN "customer_email" TO "email"` {
		t.Fatalf("pending operation SQL = %q", plan.Pending[0].Operations[0].SQL)
	}
}

func TestBuildPatchPlanFiltersPhase(t *testing.T) {
	postSync := testLoadedPatch(t, "sales", "0001_post_sync", `  - type: sql
    name: post-sync
    reason: Runs after sync.
    statement: SELECT 1;
`)
	postSync.Patch.Phase = PatchPhasePostSync

	plan, err := BuildPatchPlan([]patches.LoadedPatch{postSync}, nil, LiveSchema{Tables: map[string]liveTable{}}, nil, PatchPhasePreSync)
	if err != nil {
		t.Fatalf("BuildPatchPlan() error = %v, want nil", err)
	}
	if len(plan.Pending) != 0 || len(plan.Applied) != 0 {
		t.Fatalf("plan = %+v, want no pre-sync patches", plan)
	}
}

func TestBuildPatchPlanRejectsChecksumMismatch(t *testing.T) {
	patch := testLoadedPatch(t, "sales", "0001_rename_email", `  - type: sql
    name: normalize
    reason: Normalize legacy data.
    statement: SELECT 1;
`)

	_, err := BuildPatchPlan([]patches.LoadedPatch{patch}, nil, LiveSchema{Tables: map[string]liveTable{}}, []PatchRun{{
		AppName:  "sales",
		PatchID:  "0001_rename_email",
		Path:     patch.AppRelativePath,
		Phase:    PatchPhasePreSync,
		Checksum: "old-checksum",
	}}, PatchPhasePreSync)
	if err == nil {
		t.Fatal("BuildPatchPlan() error = nil, want checksum mismatch")
	}
	if !IsPatchRunChecksumMismatch(err) {
		t.Fatalf("BuildPatchPlan() error = %T %v, want checksum mismatch", err, err)
	}
	if !strings.Contains(err.Error(), "patch run sales/0001_rename_email checksum mismatch") {
		t.Fatalf("BuildPatchPlan() error = %q, want checksum mismatch text", err.Error())
	}
}

func TestBuildPatchPlanRejectsInvalidPhase(t *testing.T) {
	_, err := BuildPatchPlan(nil, nil, LiveSchema{Tables: map[string]liveTable{}}, nil, "during-sync")
	if err == nil {
		t.Fatal("BuildPatchPlan() error = nil, want invalid phase error")
	}
	if !strings.Contains(err.Error(), `patch phase must be "pre-sync" or "post-sync"`) {
		t.Fatalf("BuildPatchPlan() error = %q, want phase error", err.Error())
	}
}

func TestPatchLedgerTablesAvailableRequiresMetadataTables(t *testing.T) {
	tests := []struct {
		name string
		live LiveSchema
		want bool
	}{
		{name: "empty", live: LiveSchema{Tables: map[string]liveTable{}}, want: false},
		{name: "app only", live: LiveSchema{Tables: map[string]liveTable{"app": {Name: "app"}}}, want: false},
		{name: "patch run only", live: LiveSchema{Tables: map[string]liveTable{"patch_run": {Name: "patch_run"}}}, want: false},
		{name: "ledger ready", live: LiveSchema{Tables: map[string]liveTable{
			"app":       {Name: "app"},
			"patch_run": {Name: "patch_run"},
		}}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := patchLedgerTablesAvailable(tt.live); got != tt.want {
				t.Fatalf("patchLedgerTablesAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

package db

import (
	"context"
	"fmt"

	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/patches"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PatchPlan is a read-only view of patch files against the applied patch ledger.
type PatchPlan struct {
	Phase   string
	Pending []PlannedPatch
	Applied []AppliedPatch
}

// PlannedPatch is one discovered patch that has not been applied yet.
type PlannedPatch struct {
	AppName         string
	PatchID         string
	Phase           string
	Path            string
	AppRelativePath string
	Checksum        string
	Description     string
	Operations      []PatchOperation
}

// AppliedPatch is one discovered patch that already has a matching ledger entry.
type AppliedPatch struct {
	AppName         string
	PatchID         string
	Phase           string
	Path            string
	AppRelativePath string
	Checksum        string
	Description     string
	Run             PatchRun
}

// PatchPlan compares discovered patch files with the patch ledger without writing to the database.
func (m Migrator) PatchPlan(ctx context.Context, root string, databaseURL string, phase string) (PatchPlan, error) {
	pool, err := connectMetadataPool(ctx, databaseURL)
	if err != nil {
		return PatchPlan{}, err
	}
	defer pool.Close()

	plan, err := PlanPatches(ctx, pool, root, phase)
	if err != nil {
		return PatchPlan{}, fmt.Errorf("plan patches: %w", err)
	}
	return plan, nil
}

// PlanPatches discovers patch files, reads the ledger, and builds a read-only patch plan.
func PlanPatches(ctx context.Context, pool *pgxpool.Pool, root string, phase string) (PatchPlan, error) {
	metadata, err := loadMetadataCatalog(root)
	if err != nil {
		return PatchPlan{}, err
	}
	loaded, err := patches.Discover(metadata.Apps)
	if err != nil {
		return PatchPlan{}, err
	}
	live, err := InspectLiveSchema(ctx, pool)
	if err != nil {
		return PatchPlan{}, err
	}
	runs, err := NewPatchLedger(pool).ListPatchRuns(ctx)
	if err != nil {
		return PatchPlan{}, err
	}
	return BuildPatchPlan(loaded, metadata.Entities, live, runs, phase)
}

// BuildPatchPlan builds a read-only pending/applied patch plan from already-loaded inputs.
func BuildPatchPlan(loaded []patches.LoadedPatch, entities []catalog.LoadedEntity, live LiveSchema, runs []PatchRun, phase string) (PatchPlan, error) {
	if !validPatchPhase(phase) {
		return PatchPlan{}, fmt.Errorf("patch phase must be %q or %q", PatchPhasePreSync, PatchPhasePostSync)
	}

	runByPatch := map[string]PatchRun{}
	for _, run := range runs {
		runByPatch[patchRunKey(run.AppName, run.PatchID)] = run
	}

	pendingLoaded := []patches.LoadedPatch{}
	pendingByPatch := map[string]int{}
	plan := PatchPlan{Phase: phase}
	for _, patch := range loaded {
		if patch.Patch.Phase != phase {
			continue
		}

		if run, ok := runByPatch[patchRunKey(patch.AppName, patch.Patch.ID)]; ok {
			if run.Checksum != patch.Checksum {
				return PatchPlan{}, PatchRunChecksumMismatchError{
					AppName:         patch.AppName,
					PatchID:         patch.Patch.ID,
					AppliedChecksum: run.Checksum,
					CurrentChecksum: patch.Checksum,
				}
			}
			plan.Applied = append(plan.Applied, appliedPatchFromLoaded(patch, run))
			continue
		}

		pendingByPatch[patchRunKey(patch.AppName, patch.Patch.ID)] = len(plan.Pending)
		plan.Pending = append(plan.Pending, plannedPatchFromLoaded(patch))
		pendingLoaded = append(pendingLoaded, patch)
	}

	if len(pendingLoaded) == 0 {
		return plan, nil
	}
	operationPlan, err := BuildPatchOperationPlan(pendingLoaded, entities, live)
	if err != nil {
		return PatchPlan{}, err
	}
	for _, operation := range operationPlan.Operations {
		key := patchRunKey(operation.AppName, operation.PatchID)
		index, ok := pendingByPatch[key]
		if !ok {
			return PatchPlan{}, fmt.Errorf("planned operation references unknown pending patch %s/%s", operation.AppName, operation.PatchID)
		}
		plan.Pending[index].Operations = append(plan.Pending[index].Operations, operation)
	}
	return plan, nil
}

func validPatchPhase(phase string) bool {
	return phase == PatchPhasePreSync || phase == PatchPhasePostSync
}

func patchRunKey(appName string, patchID string) string {
	return appName + "\x00" + patchID
}

func plannedPatchFromLoaded(patch patches.LoadedPatch) PlannedPatch {
	return PlannedPatch{
		AppName:         patch.AppName,
		PatchID:         patch.Patch.ID,
		Phase:           patch.Patch.Phase,
		Path:            patch.Path,
		AppRelativePath: patch.AppRelativePath,
		Checksum:        patch.Checksum,
		Description:     patch.Patch.Description,
	}
}

func appliedPatchFromLoaded(patch patches.LoadedPatch, run PatchRun) AppliedPatch {
	return AppliedPatch{
		AppName:         patch.AppName,
		PatchID:         patch.Patch.ID,
		Phase:           patch.Patch.Phase,
		Path:            patch.Path,
		AppRelativePath: patch.AppRelativePath,
		Checksum:        patch.Checksum,
		Description:     patch.Patch.Description,
		Run:             run,
	}
}

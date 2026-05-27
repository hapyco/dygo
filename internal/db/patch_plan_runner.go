package db

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/patches"
	"github.com/jackc/pgx/v5"
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

// PatchApplyResult reports patches successfully applied by one apply command.
type PatchApplyResult struct {
	Phase   string
	Applied []PatchRun
}

type patchTransactionBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
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

// ApplyPatches applies pending patches for one phase and writes a schema snapshot after success.
func (m Migrator) ApplyPatches(ctx context.Context, root string, databaseURL string, phase string, dygoVersion string) (PatchApplyResult, error) {
	pool, err := connectMetadataPool(ctx, databaseURL)
	if err != nil {
		return PatchApplyResult{}, err
	}
	defer pool.Close()

	plan, err := PlanPatches(ctx, pool, root, phase)
	if err != nil {
		return PatchApplyResult{}, fmt.Errorf("plan patches: %w", err)
	}
	return m.applyPatchPlan(ctx, pool, plan, root, databaseURL, dygoVersion)
}

func (m Migrator) applyPatchPlan(ctx context.Context, beginner patchTransactionBeginner, plan PatchPlan, root string, databaseURL string, dygoVersion string) (PatchApplyResult, error) {
	result, err := ApplyPatchPlan(ctx, beginner, plan, root, dygoVersion)
	if err != nil {
		return result, err
	}
	if len(result.Applied) == 0 {
		return result, nil
	}
	if err := m.dumpSchema(ctx, root, databaseURL); err != nil {
		return result, fmt.Errorf("dump schema after patches apply: %w", err)
	}
	return result, nil
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
	runs := []PatchRun{}
	if patchLedgerTablesAvailable(live) {
		runs, err = NewPatchLedger(pool).ListPatchRuns(ctx)
		if err != nil {
			return PatchPlan{}, err
		}
	}
	return BuildPatchPlan(loaded, metadata.Entities, live, runs, phase)
}

func patchLedgerTablesAvailable(live LiveSchema) bool {
	if live.Tables == nil {
		return false
	}
	// Fresh databases do not have metadata tables yet; treat the patch ledger as
	// empty until schema sync creates it.
	_, hasApp := live.Tables["app"]
	_, hasPatchRun := live.Tables["patch_run"]
	return hasApp && hasPatchRun
}

// ApplyPatchPlan applies planned pending patches using one transaction per patch.
func ApplyPatchPlan(ctx context.Context, beginner patchTransactionBeginner, plan PatchPlan, root string, dygoVersion string) (PatchApplyResult, error) {
	if beginner == nil {
		return PatchApplyResult{}, fmt.Errorf("patch transaction beginner is required")
	}
	result := PatchApplyResult{Phase: plan.Phase}
	for _, patch := range plan.Pending {
		run, err := applyOnePatch(ctx, beginner, patch, root, dygoVersion)
		if err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, run)
	}
	return result, nil
}

func applyOnePatch(ctx context.Context, beginner patchTransactionBeginner, patch PlannedPatch, root string, dygoVersion string) (PatchRun, error) {
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return PatchRun{}, fmt.Errorf("begin patch %s/%s transaction: %w", patch.AppName, patch.PatchID, err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	for _, operation := range patch.Operations {
		if _, err := tx.Exec(ctx, operation.SQL); err != nil {
			return PatchRun{}, fmt.Errorf("apply patch %s/%s operation %d %s: %w", patch.AppName, patch.PatchID, operation.OperationIndex, operation.Type, err)
		}
	}

	path, err := patchLedgerPath(root, patch)
	if err != nil {
		return PatchRun{}, err
	}
	run := PatchRun{
		AppName:     patch.AppName,
		PatchID:     patch.PatchID,
		Path:        path,
		Phase:       patch.Phase,
		Checksum:    patch.Checksum,
		AppliedAt:   time.Now().UTC(),
		DygoVersion: dygoVersion,
	}
	if err := NewPatchLedger(tx).RecordPatchRun(ctx, run); err != nil {
		return PatchRun{}, fmt.Errorf("record patch %s/%s: %w", patch.AppName, patch.PatchID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PatchRun{}, fmt.Errorf("commit patch %s/%s: %w", patch.AppName, patch.PatchID, err)
	}
	committed = true
	return run, nil
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

func patchLedgerPath(root string, patch PlannedPatch) (string, error) {
	path := strings.TrimSpace(patch.Path)
	if path == "" {
		if strings.TrimSpace(patch.AppRelativePath) != "" {
			return filepath.ToSlash(filepath.Clean(patch.AppRelativePath)), nil
		}
		return "", fmt.Errorf("patch %s/%s path is required", patch.AppName, patch.PatchID)
	}
	if !filepath.IsAbs(path) {
		return filepath.ToSlash(filepath.Clean(path)), nil
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("project root is required to record patch %s/%s path", patch.AppName, patch.PatchID)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("make patch %s/%s path relative to project root: %w", patch.AppName, patch.PatchID, err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("patch %s/%s path is outside project root", patch.AppName, patch.PatchID)
	}
	return filepath.ToSlash(rel), nil
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

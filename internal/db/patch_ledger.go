package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dygo-dev/dygo/internal/entity/schema"
	"github.com/jackc/pgx/v5"
)

const (
	PatchPhasePreSync  = "pre-sync"
	PatchPhasePostSync = "post-sync"
)

// PatchRun is one successful app patch ledger entry.
type PatchRun struct {
	AppName     string
	PatchID     string
	Path        string
	Phase       string
	Checksum    string
	AppliedAt   time.Time
	DygoVersion string
}

// PatchLedgerQueryer is the database behavior needed by the patch ledger.
type PatchLedgerQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// PatchLedger reads and writes successful app patch ledger entries.
type PatchLedger struct {
	queryer PatchLedgerQueryer
}

// PatchRunAlreadyAppliedError reports an attempt to record an applied patch twice.
type PatchRunAlreadyAppliedError struct {
	AppName string
	PatchID string
}

func (e PatchRunAlreadyAppliedError) Error() string {
	return fmt.Sprintf("patch run %s/%s is already recorded", e.AppName, e.PatchID)
}

// PatchRunChecksumMismatchError reports a changed patch file after it was applied.
type PatchRunChecksumMismatchError struct {
	AppName         string
	PatchID         string
	AppliedChecksum string
	CurrentChecksum string
}

func (e PatchRunChecksumMismatchError) Error() string {
	return fmt.Sprintf("patch run %s/%s checksum mismatch: applied %s, current %s", e.AppName, e.PatchID, e.AppliedChecksum, e.CurrentChecksum)
}

// IsPatchRunAlreadyApplied reports whether err is a PatchRunAlreadyAppliedError.
func IsPatchRunAlreadyApplied(err error) bool {
	var alreadyApplied PatchRunAlreadyAppliedError
	return errors.As(err, &alreadyApplied)
}

// IsPatchRunChecksumMismatch reports whether err is a PatchRunChecksumMismatchError.
func IsPatchRunChecksumMismatch(err error) bool {
	var mismatch PatchRunChecksumMismatchError
	return errors.As(err, &mismatch)
}

// NewPatchLedger returns a patch ledger backed by queryer.
func NewPatchLedger(queryer PatchLedgerQueryer) PatchLedger {
	return PatchLedger{queryer: queryer}
}

// ListPatchRuns returns all successful patch runs ordered by app and patch id.
func (l PatchLedger) ListPatchRuns(ctx context.Context) ([]PatchRun, error) {
	if err := l.requireQueryer(); err != nil {
		return nil, err
	}
	rows, err := l.queryer.Query(ctx, `
SELECT a.name, p.patch_id, p.path, p.phase, p.checksum, p.applied_at, COALESCE(p.dygo_version, '')
FROM "patch_run" p
JOIN "app" a ON a.id = p.app_id
ORDER BY a.name, p.patch_id`)
	if err != nil {
		return nil, fmt.Errorf("query patch runs: %w", err)
	}
	defer rows.Close()

	runs := []PatchRun{}
	for rows.Next() {
		run, err := scanPatchRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read patch runs: %w", err)
	}
	return runs, nil
}

// GetPatchRun returns one successful patch run by app and patch id.
func (l PatchLedger) GetPatchRun(ctx context.Context, appName string, patchID string) (PatchRun, error) {
	if err := l.requireQueryer(); err != nil {
		return PatchRun{}, err
	}
	row := l.queryer.QueryRow(ctx, `
SELECT a.name, p.patch_id, p.path, p.phase, p.checksum, p.applied_at, COALESCE(p.dygo_version, '')
FROM "patch_run" p
JOIN "app" a ON a.id = p.app_id
WHERE a.name = $1 AND p.patch_id = $2`, appName, patchID)
	run, err := scanPatchRun(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return PatchRun{}, MetadataNotFoundError{Kind: "patch-run", Name: patchRunIdentityName(appName, patchID)}
	}
	if err != nil {
		return PatchRun{}, fmt.Errorf("query patch run %s/%s: %w", appName, patchID, err)
	}
	return run, nil
}

// RecordPatchRun inserts one successful patch run into the ledger.
func (l PatchLedger) RecordPatchRun(ctx context.Context, run PatchRun) error {
	if err := l.requireQueryer(); err != nil {
		return err
	}
	if err := validatePatchRun(run); err != nil {
		return err
	}
	existing, err := l.GetPatchRun(ctx, run.AppName, run.PatchID)
	if err == nil {
		if existing.Checksum != run.Checksum {
			return PatchRunChecksumMismatchError{
				AppName:         run.AppName,
				PatchID:         run.PatchID,
				AppliedChecksum: existing.Checksum,
				CurrentChecksum: run.Checksum,
			}
		}
		return PatchRunAlreadyAppliedError{AppName: run.AppName, PatchID: run.PatchID}
	}
	if !IsMetadataNotFound(err) {
		return err
	}

	appID, err := l.appID(ctx, run.AppName)
	if err != nil {
		return err
	}
	appliedAt := run.AppliedAt
	if appliedAt.IsZero() {
		appliedAt = time.Now().UTC()
	}
	naming, err := l.patchRunNaming(ctx)
	if err != nil {
		return err
	}
	name, err := patchRunRecordName(run, naming)
	if err != nil {
		return err
	}
	if err := l.queryer.QueryRow(ctx, `
INSERT INTO "patch_run" (name, app_id, patch_id, path, phase, checksum, applied_at, dygo_version)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id`, name, appID, run.PatchID, run.Path, run.Phase, run.Checksum, appliedAt, nullIfEmpty(run.DygoVersion)).Scan(new(int64)); err != nil {
		return fmt.Errorf("record patch run %s/%s: %w", run.AppName, run.PatchID, err)
	}
	return nil
}

func (l PatchLedger) patchRunNaming(ctx context.Context) (schema.Naming, error) {
	var raw string
	err := l.queryer.QueryRow(ctx, `
SELECT COALESCE(e.naming::text, '')
FROM "entity" e
JOIN "app" a ON a.id = e.app_id
WHERE a.name = 'core' AND e.key = 'patch-run'`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return schema.Naming{}, MetadataNotFoundError{Kind: "entity", Name: "core.patch-run"}
	}
	if err != nil {
		return schema.Naming{}, fmt.Errorf("query patch-run naming metadata: %w", err)
	}
	var naming schema.Naming
	if err := json.Unmarshal([]byte(raw), &naming); err != nil {
		return schema.Naming{}, fmt.Errorf("decode patch-run naming metadata: %w", err)
	}
	return naming, nil
}

func patchRunRecordName(run PatchRun, naming schema.Naming) (string, error) {
	return deterministicRecordNameFromValues("patch-run", naming, map[string]string{
		"app":          run.AppName,
		"patch-id":     run.PatchID,
		"path":         run.Path,
		"phase":        run.Phase,
		"checksum":     run.Checksum,
		"dygo-version": run.DygoVersion,
	})
}

func patchRunIdentityName(appName string, patchID string) string {
	return appName + "." + patchID
}

func (l PatchLedger) appID(ctx context.Context, appName string) (int64, error) {
	var id int64
	err := l.queryer.QueryRow(ctx, `SELECT id FROM "app" WHERE name = $1`, appName).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, MetadataNotFoundError{Kind: "app", Name: appName}
	}
	if err != nil {
		return 0, fmt.Errorf("query patch run app %q: %w", appName, err)
	}
	return id, nil
}

func scanPatchRun(row interface{ Scan(...any) error }) (PatchRun, error) {
	var run PatchRun
	if err := row.Scan(&run.AppName, &run.PatchID, &run.Path, &run.Phase, &run.Checksum, &run.AppliedAt, &run.DygoVersion); err != nil {
		return PatchRun{}, err
	}
	return run, nil
}

func validatePatchRun(run PatchRun) error {
	if strings.TrimSpace(run.AppName) == "" {
		return fmt.Errorf("patch run app is required")
	}
	if strings.TrimSpace(run.PatchID) == "" {
		return fmt.Errorf("patch run id is required")
	}
	if strings.TrimSpace(run.Path) == "" {
		return fmt.Errorf("patch run path is required")
	}
	if run.Phase != PatchPhasePreSync && run.Phase != PatchPhasePostSync {
		return fmt.Errorf("patch run phase must be %q or %q", PatchPhasePreSync, PatchPhasePostSync)
	}
	if strings.TrimSpace(run.Checksum) == "" {
		return fmt.Errorf("patch run checksum is required")
	}
	return nil
}

func (l PatchLedger) requireQueryer() error {
	if l.queryer == nil {
		return fmt.Errorf("patch ledger queryer is required")
	}
	return nil
}

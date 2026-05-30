// Package hooks adapts public app hook registrations into dygo internals.
package hooks

import (
	"context"
	"fmt"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/hookevents"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/internal/sdkdata"
	"github.com/hapyco/dygo/pkg/sdk"
)

// NewRecordHookRegistry returns dygo's framework hooks plus compiled app hooks.
func NewRecordHookRegistry(registrars []sdk.RecordHookRegistrar) (*db.RecordHookRegistry, error) {
	registry := db.DefaultRecordHookRegistry()
	adapter := recordHookRegistry{registry: registry}
	for index, registrar := range registrars {
		if registrar == nil {
			return nil, fmt.Errorf("record hook registrar %d is required", index+1)
		}
		if err := registrar(adapter); err != nil {
			return nil, fmt.Errorf("register record hook registrar %d: %w", index+1, err)
		}
	}
	return registry, nil
}

type recordHookRegistry struct {
	registry *db.RecordHookRegistry
}

func (r recordHookRegistry) RegisterEntity(appName string, entity string, event sdk.RecordHookEvent, name string, fn sdk.RecordHookFunc) error {
	if fn == nil {
		return fmt.Errorf("record hook %q function is required", name)
	}
	dbEvent, err := recordHookEvent(event)
	if err != nil {
		return err
	}
	return r.registry.RegisterEntity(appName, entity, dbEvent, name, func(ctx context.Context, hookCtx db.RecordHookContext) error {
		var jobs sdk.JobData
		if beginner, ok := hookCtx.Queryer.(jobstore.Beginner); ok {
			jobData, err := sdkdata.NewJobDataFromBeginner(beginner)
			if err != nil {
				return err
			}
			jobs = jobData
		}
		return fn(ctx, sdk.RecordHook{
			Event:       sdk.RecordHookEvent(hookCtx.Event),
			Operation:   hookCtx.Operation,
			EntityID:    hookCtx.EntityID,
			AppName:     hookCtx.AppName,
			Entity:      hookCtx.Entity,
			RouteSlug:   hookCtx.RouteSlug,
			EntityLabel: hookCtx.EntityLabel,
			RecordID:    hookCtx.RecordID,
			Input:       sdk.RecordInput(hookCtx.Input),
			OldRecord:   sdk.Record(hookCtx.OldRecord),
			NewRecord:   sdk.Record(hookCtx.NewRecord),
			Changes:     hookCtx.Changes,
			Snapshot:    sdk.Record(hookCtx.Snapshot),
			Records:     sdkdata.NewRecordDataWithHookPolicy(hookCtx.Queryer, db.RecordMutationHooksFrameworkOnly),
			Jobs:        jobs,
		})
	})
}

func recordHookEvent(event sdk.RecordHookEvent) (db.RecordHookEvent, error) {
	if !hookevents.Supported(string(event)) {
		return "", fmt.Errorf("record hook event %q is not supported", event)
	}
	return db.RecordHookEvent(event), nil
}

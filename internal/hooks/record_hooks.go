// Package hooks adapts public app hook registrations into dygo internals.
package hooks

import (
	"context"
	"fmt"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/dygodata"
	"github.com/hapyco/dygo/internal/hookevents"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/pkg/dygo"
)

// NewRecordHookRegistry returns dygo's framework hooks plus compiled app hooks.
func NewRecordHookRegistry(registrars []dygo.RecordHookRegistrar) (*db.RecordHookRegistry, error) {
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

func (r recordHookRegistry) RegisterEntity(appName string, entity string, event dygo.RecordHookEvent, name string, fn dygo.RecordHookFunc) error {
	if fn == nil {
		return fmt.Errorf("record hook %q function is required", name)
	}
	dbEvent, err := recordHookEvent(event)
	if err != nil {
		return err
	}
	return r.registry.RegisterEntity(appName, entity, dbEvent, name, func(ctx context.Context, hookCtx db.RecordHookContext) error {
		var jobs dygo.JobData
		if beginner, ok := hookCtx.Queryer.(jobstore.Beginner); ok {
			jobData, err := dygodata.NewJobDataFromBeginner(beginner)
			if err != nil {
				return err
			}
			jobs = jobData
		}
		ctx = withRecordHookLogContext(ctx, hookCtx)
		return fn(ctx, dygo.RecordHook{
			Event:       dygo.RecordHookEvent(hookCtx.Event),
			Operation:   hookCtx.Operation,
			EntityID:    hookCtx.EntityID,
			AppName:     hookCtx.AppName,
			Entity:      hookCtx.Entity,
			RouteSlug:   hookCtx.RouteSlug,
			EntityLabel: hookCtx.EntityLabel,
			RecordID:    hookCtx.RecordID,
			Input:       dygo.RecordInput(hookCtx.Input),
			OldRecord:   dygo.Record(hookCtx.OldRecord),
			NewRecord:   dygo.Record(hookCtx.NewRecord),
			Changes:     hookCtx.Changes,
			Snapshot:    dygo.Record(hookCtx.Snapshot),
			Records:     dygodata.NewRecordDataWithHookPolicy(hookCtx.Queryer, db.RecordMutationHooksFrameworkOnly),
			Jobs:        jobs,
		})
	})
}

func withRecordHookLogContext(ctx context.Context, hookCtx db.RecordHookContext) context.Context {
	if hookCtx.Queryer != nil {
		ctx = dygo.WithLogWriter(ctx, dygodata.NewLogData(hookCtx.Queryer))
	}
	defaults := dygo.LogContext{
		Source:              dygo.SourceHook,
		App:                 hookCtx.AppName,
		ReferenceEntity:     hookCtx.AppName + "." + hookCtx.Entity,
		ReferenceRecordID:   hookCtx.RecordID,
		ReferenceRecordName: hookRecordName(hookCtx),
	}
	if actor, ok := db.ActivityActorNameFromContext(ctx); ok {
		defaults.Actor = actor
	}
	return dygo.WithLogContext(ctx, defaults)
}

func hookRecordName(hookCtx db.RecordHookContext) string {
	for _, record := range []db.Record{hookCtx.NewRecord, hookCtx.OldRecord, hookCtx.Snapshot} {
		if name, ok := record["name"].(string); ok && name != "" {
			return name
		}
	}
	return ""
}

func recordHookEvent(event dygo.RecordHookEvent) (db.RecordHookEvent, error) {
	if !hookevents.Supported(string(event)) {
		return "", fmt.Errorf("record hook event %q is not supported", event)
	}
	return db.RecordHookEvent(event), nil
}

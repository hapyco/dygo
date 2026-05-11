// Package hooks adapts public app hook registrations into dygo internals.
package hooks

import (
	"context"
	"fmt"

	"github.com/dygo-dev/dygo/internal/db"
	"github.com/dygo-dev/dygo/pkg/sdk"
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

func (r recordHookRegistry) RegisterEntity(entity string, event sdk.RecordHookEvent, name string, fn sdk.RecordHookFunc) error {
	if fn == nil {
		return fmt.Errorf("record hook %q function is required", name)
	}
	return r.registry.RegisterEntity(entity, db.RecordHookEvent(event), name, func(ctx context.Context, hookCtx db.RecordHookContext) error {
		return fn(ctx, sdk.RecordHookContext{
			Event:       sdk.RecordHookEvent(hookCtx.Event),
			Operation:   hookCtx.Operation,
			EntityID:    hookCtx.EntityID,
			Entity:      hookCtx.Entity,
			EntityLabel: hookCtx.EntityLabel,
			RecordID:    hookCtx.RecordID,
			Input:       sdk.RecordInput(hookCtx.Input),
			OldRecord:   sdk.Record(hookCtx.OldRecord),
			NewRecord:   sdk.Record(hookCtx.NewRecord),
			Changes:     hookCtx.Changes,
			Snapshot:    sdk.Record(hookCtx.Snapshot),
		})
	})
}

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

func (r recordHookRegistry) RegisterEntity(appName string, entity string, event sdk.RecordHookEvent, name string, fn sdk.RecordHookFunc) error {
	if fn == nil {
		return fmt.Errorf("record hook %q function is required", name)
	}
	return r.registry.RegisterEntity(appName, entity, db.RecordHookEvent(event), name, func(ctx context.Context, hookCtx db.RecordHookContext) error {
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
			Records:     recordData{queryer: hookCtx.Queryer},
		})
	})
}

type recordData struct {
	queryer db.RecordQueryer
}

func (d recordData) store() db.RecordStore {
	return db.NewRecordStore(d.queryer)
}

func (d recordData) List(ctx context.Context, appName string, entity string, params sdk.RecordListParams) (sdk.RecordListResult, error) {
	result, err := d.store().ListRecordsByIdentity(ctx, appName, entity, db.RecordListParams(params))
	if err != nil {
		return sdk.RecordListResult{}, err
	}
	return sdk.RecordListResult{
		Records: sdkRecords(result.Records),
		Limit:   result.Limit,
		Offset:  result.Offset,
		Count:   result.Count,
	}, nil
}

func (d recordData) Get(ctx context.Context, appName string, entity string, id int64) (sdk.Record, error) {
	record, err := d.store().GetRecordByIdentity(ctx, appName, entity, id)
	if err != nil {
		return nil, err
	}
	return sdk.Record(record), nil
}

func (d recordData) Find(ctx context.Context, appName string, entity string, match sdk.RecordInput) (sdk.Record, error) {
	record, err := d.store().FindRecordByIdentity(ctx, appName, entity, db.RecordInput(match))
	if err != nil {
		return nil, err
	}
	return sdk.Record(record), nil
}

func (d recordData) Create(ctx context.Context, appName string, entity string, input sdk.RecordInput) (sdk.Record, error) {
	record, err := d.store().CreateRecordByIdentity(ctx, appName, entity, db.RecordInput(input))
	if err != nil {
		return nil, err
	}
	return sdk.Record(record), nil
}

func (d recordData) Update(ctx context.Context, appName string, entity string, id int64, input sdk.RecordInput) (sdk.Record, error) {
	record, err := d.store().UpdateRecordByIdentity(ctx, appName, entity, id, db.RecordInput(input))
	if err != nil {
		return nil, err
	}
	return sdk.Record(record), nil
}

func (d recordData) Delete(ctx context.Context, appName string, entity string, id int64) error {
	return d.store().DeleteRecordByIdentity(ctx, appName, entity, id)
}

func sdkRecords(records []db.Record) []sdk.Record {
	converted := make([]sdk.Record, len(records))
	for i, record := range records {
		converted[i] = sdk.Record(record)
	}
	return converted
}

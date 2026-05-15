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
	dbEvent, err := recordHookEvent(event)
	if err != nil {
		return err
	}
	return r.registry.RegisterEntity(appName, entity, dbEvent, name, func(ctx context.Context, hookCtx db.RecordHookContext) error {
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

func recordHookEvent(event sdk.RecordHookEvent) (db.RecordHookEvent, error) {
	switch event {
	case sdk.RecordBeforeValidate:
		return db.RecordBeforeValidate, nil
	case sdk.RecordValidate:
		return db.RecordValidate, nil
	case sdk.RecordBeforeCreate:
		return db.RecordBeforeCreate, nil
	case sdk.RecordAfterCreate:
		return db.RecordAfterCreate, nil
	case sdk.RecordBeforeUpdate:
		return db.RecordBeforeUpdate, nil
	case sdk.RecordAfterUpdate:
		return db.RecordAfterUpdate, nil
	case sdk.RecordBeforeDelete:
		return db.RecordBeforeDelete, nil
	case sdk.RecordAfterDelete:
		return db.RecordAfterDelete, nil
	default:
		return "", fmt.Errorf("record hook event %q is not supported", event)
	}
}

type recordData struct {
	queryer db.RecordQueryer
}

func (d recordData) store() db.RecordStore {
	return db.NewRecordStore(d.queryer)
}

func (d recordData) List(ctx context.Context, appName string, entity string, params sdk.RecordListParams) (sdk.RecordListResult, error) {
	result, err := d.store().ListRecordsByIdentity(ctx, appName, entity, dbRecordListParams(params))
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

func dbRecordListParams(params sdk.RecordListParams) db.RecordListParams {
	converted := db.RecordListParams{
		Limit:  params.Limit,
		Offset: params.Offset,
	}
	if len(params.Filters) > 0 {
		converted.Filters = make([]db.RecordFilter, len(params.Filters))
		for i, filter := range params.Filters {
			converted.Filters[i] = db.RecordFilter{Field: filter.Field, Value: filter.Value}
		}
	}
	if len(params.Sort) > 0 {
		converted.Sort = make([]db.RecordSort, len(params.Sort))
		for i, sortTerm := range params.Sort {
			converted.Sort[i] = db.RecordSort{Field: sortTerm.Field, Desc: sortTerm.Desc}
		}
	}
	return converted
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

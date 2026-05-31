// Package dygodata adapts internal runtime services to public dygo interfaces.
package dygodata

import (
	"context"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/pkg/dygo"
)

// RecordData exposes metadata-backed Record access through the public SDK.
type RecordData struct {
	queryer       db.RecordQueryer
	hooks         *db.RecordHookRegistry
	mutationHooks db.RecordMutationHookPolicy
}

// NewRecordData returns dygo RecordData that uses the supplied Record hooks.
func NewRecordData(queryer db.RecordQueryer, hooks *db.RecordHookRegistry) RecordData {
	return RecordData{queryer: queryer, hooks: hooks, mutationHooks: db.RecordMutationHooksFrameworkOnly}
}

// NewRecordDataWithHookPolicy returns dygo RecordData with an explicit mutation hook policy.
func NewRecordDataWithHookPolicy(queryer db.RecordQueryer, policy db.RecordMutationHookPolicy) RecordData {
	return RecordData{queryer: queryer, mutationHooks: policy}
}

func (d RecordData) store() db.RecordStore {
	if d.hooks != nil {
		return db.NewRecordStoreWithHooks(d.queryer, d.hooks)
	}
	return db.NewRecordStoreWithHookPolicy(d.queryer, d.mutationHooks)
}

// List returns a page of Records by app/entity identity.
func (d RecordData) List(ctx context.Context, appName string, entity string, params dygo.RecordListParams) (dygo.RecordListResult, error) {
	result, err := d.store().ListRecordsByIdentity(ctx, appName, entity, dbRecordListParams(params))
	if err != nil {
		return dygo.RecordListResult{}, err
	}
	return dygo.RecordListResult{
		Records: dygoRecords(result.Records),
		Limit:   result.Limit,
		Offset:  result.Offset,
		Count:   result.Count,
	}, nil
}

func dbRecordListParams(params dygo.RecordListParams) db.RecordListParams {
	converted := db.RecordListParams{
		Limit:  params.Limit,
		Offset: params.Offset,
	}
	if len(params.Filters) > 0 {
		converted.Filters = make([]db.RecordFilter, len(params.Filters))
		for i, filter := range params.Filters {
			converted.Filters[i] = db.RecordFilter{Field: filter.Field, Operator: filter.Operator, Value: filter.Value}
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

// Get returns one Record by app/entity identity and row ID.
func (d RecordData) Get(ctx context.Context, appName string, entity string, id int64) (dygo.Record, error) {
	record, err := d.store().GetRecordByIdentity(ctx, appName, entity, id)
	if err != nil {
		return nil, err
	}
	return dygo.Record(record), nil
}

// Find returns one Record matching metadata-backed fields.
func (d RecordData) Find(ctx context.Context, appName string, entity string, match dygo.RecordInput) (dygo.Record, error) {
	record, err := d.store().FindRecordByIdentity(ctx, appName, entity, db.RecordInput(match))
	if err != nil {
		return nil, err
	}
	return dygo.Record(record), nil
}

// Create creates one Record by app/entity identity.
func (d RecordData) Create(ctx context.Context, appName string, entity string, input dygo.RecordInput) (dygo.Record, error) {
	record, err := d.store().CreateRecordByIdentity(ctx, appName, entity, db.RecordInput(input))
	if err != nil {
		return nil, err
	}
	return dygo.Record(record), nil
}

// Update updates one Record by app/entity identity and row ID.
func (d RecordData) Update(ctx context.Context, appName string, entity string, id int64, input dygo.RecordInput) (dygo.Record, error) {
	record, err := d.store().UpdateRecordByIdentity(ctx, appName, entity, id, db.RecordInput(input))
	if err != nil {
		return nil, err
	}
	return dygo.Record(record), nil
}

// Delete deletes one Record by app/entity identity and row ID.
func (d RecordData) Delete(ctx context.Context, appName string, entity string, id int64) error {
	return d.store().DeleteRecordByIdentity(ctx, appName, entity, id)
}

func dygoRecords(records []db.Record) []dygo.Record {
	converted := make([]dygo.Record, len(records))
	for i, record := range records {
		converted[i] = dygo.Record(record)
	}
	return converted
}

package db

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

// SystemMutationOptions declares which public Record behavior an internal writer is bypassing.
type SystemMutationOptions struct {
	RunHooks      bool
	WriteActivity bool
	ReturnRecord  bool
	Bootstrap     bool
}

// SystemRecordWriter writes framework-owned Records through metadata-backed mutation code.
type SystemRecordWriter struct {
	store RecordStore
}

// NewSystemRecordWriter returns an internal Record writer backed by queryer.
func NewSystemRecordWriter(queryer RecordQueryer) SystemRecordWriter {
	return SystemRecordWriter{store: NewRecordStoreWithHookPolicy(queryer, RecordMutationHooksNone)}
}

// SystemWriter returns an internal Record writer sharing the store's queryer and metadata reader.
func (s RecordStore) SystemWriter() SystemRecordWriter {
	return SystemRecordWriter{store: s}
}

// InsertByIdentity inserts one app-scoped system Record.
func (w SystemRecordWriter) InsertByIdentity(ctx context.Context, appName string, entity string, input RecordInput, opts SystemMutationOptions) (Record, error) {
	if err := w.store.requireQueryer(); err != nil {
		return nil, err
	}
	store, opts, err := w.mutationStore(appName, entity, opts)
	if err != nil {
		return nil, err
	}
	if opts.ReturnRecord {
		return store.createRecordByIdentity(ctx, appName, entity, input)
	}
	layout, err := store.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return nil, err
	}
	if layout.IsSingle {
		return nil, singleRecordOperationError(layout, "create")
	}
	if layout.IsCollection {
		return nil, collectionRecordOperationError(layout, "create")
	}
	_, err = store.insertRecordWithLayout(ctx, layout, cloneRecordInput(input), false)
	return nil, err
}

// UpsertByIdentity creates or updates one app-scoped system Record by a metadata-backed match.
func (w SystemRecordWriter) UpsertByIdentity(ctx context.Context, appName string, entity string, match RecordInput, input RecordInput, opts SystemMutationOptions) (Record, error) {
	if err := w.store.requireQueryer(); err != nil {
		return nil, err
	}
	store, opts, err := w.mutationStore(appName, entity, opts)
	if err != nil {
		return nil, err
	}
	var record Record
	record, err = store.withRecordMutation(ctx, func(txStore RecordStore) (Record, error) {
		existing, err := txStore.FindRecordByIdentity(ctx, appName, entity, match)
		if err != nil {
			if !isRecordNotFound(err) {
				return nil, err
			}
			return txStore.createRecordByIdentity(ctx, appName, entity, input)
		}
		id, err := activityRecordID(existing)
		if err != nil {
			return nil, err
		}
		return txStore.updateRecordByIdentity(ctx, appName, entity, id, input)
	})
	if err != nil {
		return nil, err
	}
	if opts.ReturnRecord {
		return record, nil
	}
	return nil, nil
}

func (w SystemRecordWriter) mutationStore(appName string, entity string, opts SystemMutationOptions) (RecordStore, SystemMutationOptions, error) {
	if opts.Bootstrap {
		opts.RunHooks = false
		opts.WriteActivity = false
	}
	if opts.RunHooks && !opts.ReturnRecord {
		return RecordStore{}, opts, recordError(RecordErrorInvalidRequest, "system mutations that run hooks must return the created record", map[string]any{"app": appName, "entity": entity}, nil)
	}
	store := w.store
	if !opts.RunHooks {
		store.hooks = nil
	} else if !opts.WriteActivity {
		store.hooks = store.hooks.withoutHook("activity-history")
	}
	return store, opts, nil
}

func isRecordNotFound(err error) bool {
	var recordErr RecordError
	return errors.As(err, &recordErr) && recordErr.Code == RecordErrorNotFound
}

func systemRecordString(value string) json.RawMessage {
	return json.RawMessage(strconv.Quote(value))
}

func systemRecordInt(value int64) json.RawMessage {
	return json.RawMessage(strconv.FormatInt(value, 10))
}

func systemRecordBool(value bool) json.RawMessage {
	return json.RawMessage(strconv.FormatBool(value))
}

package db

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
)

// SystemMutationPolicy declares which public Record behavior an internal writer should use.
type SystemMutationPolicy string

const (
	// SystemMutationBootstrap writes during setup/bootstrap without hooks or activity.
	SystemMutationBootstrap SystemMutationPolicy = "bootstrap"
	// SystemMutationSilent writes through metadata-backed storage without hooks or activity.
	SystemMutationSilent SystemMutationPolicy = "silent"
	// SystemMutationFramework runs dygo framework hooks, including activity.
	SystemMutationFramework SystemMutationPolicy = "framework"
	// SystemMutationFull uses the writer's configured Record hooks.
	SystemMutationFull SystemMutationPolicy = "full"
)

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

// InsertByIdentity inserts one app-scoped system Record without returning it.
func (w SystemRecordWriter) InsertByIdentity(ctx context.Context, appName string, entity string, input RecordInput, policy SystemMutationPolicy) error {
	if err := w.store.requireQueryer(); err != nil {
		return err
	}
	store, err := w.mutationStore(appName, entity, policy)
	if err != nil {
		return err
	}
	if policy == SystemMutationFramework || policy == SystemMutationFull {
		_, err := store.createRecordByIdentity(ctx, appName, entity, input)
		return err
	}
	layout, err := store.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return err
	}
	if layout.IsSingle {
		return singleRecordOperationError(layout, "create")
	}
	if layout.IsCollection {
		return collectionRecordOperationError(layout, "create")
	}
	input = cloneRecordInput(input)
	if err := store.applyFetchedFields(ctx, layout, input, nil); err != nil {
		return err
	}
	_, err = store.insertRecordWithLayout(ctx, layout, input, false)
	return err
}

// InsertReturningByIdentity inserts one app-scoped system Record and returns it.
func (w SystemRecordWriter) InsertReturningByIdentity(ctx context.Context, appName string, entity string, input RecordInput, policy SystemMutationPolicy) (Record, error) {
	if err := w.store.requireQueryer(); err != nil {
		return nil, err
	}
	store, err := w.mutationStore(appName, entity, policy)
	if err != nil {
		return nil, err
	}
	return store.createRecordByIdentity(ctx, appName, entity, input)
}

// UpsertByIdentity creates or updates one app-scoped system Record by a metadata-backed match without returning it.
func (w SystemRecordWriter) UpsertByIdentity(ctx context.Context, appName string, entity string, match RecordInput, input RecordInput, policy SystemMutationPolicy) error {
	_, err := w.upsertByIdentity(ctx, appName, entity, match, input, policy)
	return err
}

// UpsertReturningByIdentity creates or updates one app-scoped system Record by a metadata-backed match and returns it.
func (w SystemRecordWriter) UpsertReturningByIdentity(ctx context.Context, appName string, entity string, match RecordInput, input RecordInput, policy SystemMutationPolicy) (Record, error) {
	return w.upsertByIdentity(ctx, appName, entity, match, input, policy)
}

func (w SystemRecordWriter) upsertByIdentity(ctx context.Context, appName string, entity string, match RecordInput, input RecordInput, policy SystemMutationPolicy) (Record, error) {
	if err := w.store.requireQueryer(); err != nil {
		return nil, err
	}
	store, err := w.mutationStore(appName, entity, policy)
	if err != nil {
		return nil, err
	}
	record, err := store.withRecordMutation(ctx, func(txStore RecordStore) (Record, error) {
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
	return record, nil
}

func (w SystemRecordWriter) mutationStore(appName string, entity string, policy SystemMutationPolicy) (RecordStore, error) {
	store := w.store
	store.allowSystemMutations = true
	switch policy {
	case SystemMutationBootstrap, SystemMutationSilent:
		store.hooks = nil
	case SystemMutationFramework:
		store.hooks = DefaultRecordHookRegistry()
	case SystemMutationFull:
	default:
		return RecordStore{}, recordError(RecordErrorInvalidRequest, "system mutation policy is invalid", map[string]any{"app": appName, "entity": entity, "policy": string(policy)}, nil)
	}
	return store, nil
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

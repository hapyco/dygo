package db

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/dygo-dev/dygo/internal/corevalues"
	"github.com/jackc/pgx/v5"
)

const (
	// ActivitySourceAPI marks Record mutations performed through HTTP APIs.
	ActivitySourceAPI = "api"
	// ActivitySourceFixtures marks Record mutations performed by fixture apply.
	ActivitySourceFixtures = "fixtures"
)

type activityActorContextKey struct{}
type activitySourceContextKey struct{}

type recordBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// WithActivityActor attaches an optional actor user ID to Record mutation Activity.
func WithActivityActor(ctx context.Context, userID int64) context.Context {
	if ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, activityActorContextKey{}, userID)
}

// WithActivitySource attaches an optional source label to Record mutation Activity.
func WithActivitySource(ctx context.Context, source string) context.Context {
	if ctx == nil {
		return ctx
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return ctx
	}
	return context.WithValue(ctx, activitySourceContextKey{}, source)
}

// ActivityActorFromContext returns the optional actor user ID for Record mutation Activity.
func ActivityActorFromContext(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}
	value, ok := ctx.Value(activityActorContextKey{}).(int64)
	if !ok || value <= 0 {
		return 0, false
	}
	return value, true
}

// ActivitySourceFromContext returns the optional source label for Record mutation Activity.
func ActivitySourceFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	source, ok := ctx.Value(activitySourceContextKey{}).(string)
	if !ok || strings.TrimSpace(source) == "" {
		return "", false
	}
	return source, true
}

func (s RecordStore) withRecordMutation(ctx context.Context, fn func(RecordStore) (Record, error)) (Record, error) {
	beginner, ok := s.queryer.(recordBeginner)
	if !ok {
		return fn(s)
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return nil, recordError(RecordErrorInternal, "begin record transaction failed", nil, err)
	}
	txStore := NewRecordStoreWithHooks(tx, s.hooks)
	record, err := fn(txStore)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		_ = tx.Rollback(ctx)
		return nil, recordError(RecordErrorInternal, "commit record transaction failed", nil, err)
	}
	return record, nil
}

func recordActivityHook(ctx context.Context, hookCtx RecordHookContext) error {
	if hookCtx.Entity == "activity" {
		return nil
	}
	if hookCtx.Operation == corevalues.ActivityOperationUpdate && len(hookCtx.Changes) == 0 {
		return nil
	}
	changesJSON, err := activityJSONRaw(hookCtx.Changes)
	if err != nil {
		return err
	}
	snapshotJSON, err := activityJSONRaw(hookCtx.Snapshot)
	if err != nil {
		return err
	}
	detailsJSON, err := activityJSONRaw(activityDetails(ctx))
	if err != nil {
		return err
	}
	input := RecordInput{
		"kind":      systemRecordString(corevalues.ActivityKindRecord),
		"operation": systemRecordString(hookCtx.Operation),
		"status":    systemRecordString(corevalues.ActivityStatusSuccess),
		"entity":    systemRecordInt(hookCtx.EntityID),
		"record-id": systemRecordInt(hookCtx.RecordID),
		"title":     systemRecordString(activityTitle(hookCtx.EntityLabel, hookCtx.Entity, hookCtx.Operation)),
	}
	if actorID, ok := ActivityActorFromContext(ctx); ok {
		input["actor"] = systemRecordInt(actorID)
	}
	if changesJSON != nil {
		input["changes"] = changesJSON
	}
	if snapshotJSON != nil {
		input["snapshot"] = snapshotJSON
	}
	if detailsJSON != nil {
		input["details"] = detailsJSON
	}
	_, err = NewSystemRecordWriter(hookCtx.Queryer).InsertByIdentity(ctx, "core", "activity", input, SystemMutationOptions{})
	return err
}

func (l recordLayout) activityChanges(input RecordInput, oldRecord Record, newRecord Record) []map[string]any {
	input = normalizeRecordInput(input)
	changes := []map[string]any{}
	for _, name := range sortedRecordInputNames(input) {
		field := l.FieldByName[name]
		if field.WriteOnly {
			changes = append(changes, map[string]any{"field": name, "redacted": true})
			continue
		}
		oldValue := oldRecord[name]
		newValue := newRecord[name]
		if recordValuesEqual(oldValue, newValue) {
			continue
		}
		changes = append(changes, map[string]any{"field": name, "old": oldValue, "new": newValue})
	}
	return changes
}

func activityRecordID(record Record) (int64, error) {
	value, ok := record["id"]
	if !ok {
		return 0, recordError(RecordErrorInternal, "record id is missing from activity payload", nil, nil)
	}
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case float64:
		if math.Trunc(typed) == typed {
			return int64(typed), nil
		}
	}
	return 0, recordError(RecordErrorInternal, "record id is invalid for activity payload", nil, fmt.Errorf("id type %T", value))
}

func activityDetails(ctx context.Context) map[string]any {
	source, ok := ActivitySourceFromContext(ctx)
	if !ok {
		return nil
	}
	return map[string]any{"source": source}
}

func activityJSONRaw(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	switch typed := value.(type) {
	case []map[string]any:
		if len(typed) == 0 {
			return nil, nil
		}
	case map[string]any:
		if len(typed) == 0 {
			return nil, nil
		}
	case Record:
		if len(typed) == 0 {
			return nil, nil
		}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, recordError(RecordErrorInternal, "encode activity payload failed", nil, err)
	}
	return json.RawMessage(encoded), nil
}

func activityTitle(label string, entity string, operation string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		label = entity
	}
	switch operation {
	case corevalues.ActivityOperationCreate:
		return "Created " + label
	case corevalues.ActivityOperationUpdate:
		return "Updated " + label
	case corevalues.ActivityOperationDelete:
		return "Deleted " + label
	default:
		return operation + " " + label
	}
}

func recordValuesEqual(left any, right any) bool {
	return reflect.DeepEqual(left, right)
}

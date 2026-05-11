package db

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	// ActivitySourceAPI marks Record mutations performed through HTTP APIs.
	ActivitySourceAPI = "api"
	// ActivitySourceFixtures marks Record mutations performed by fixture apply.
	ActivitySourceFixtures = "fixtures"

	activityOperationCreate = "create"
	activityOperationUpdate = "update"
	activityOperationDelete = "delete"
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
	txStore := NewRecordStore(tx)
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

func (s RecordStore) writeRecordActivity(ctx context.Context, layout recordLayout, operation string, recordID int64, changes []map[string]any, snapshot Record) error {
	if layout.Entity == "activity" {
		return nil
	}
	activityName, err := randomRecordName(0)
	if err != nil {
		return recordError(RecordErrorInternal, "generate activity name failed", nil, err)
	}
	changesJSON, err := activityJSON(changes)
	if err != nil {
		return err
	}
	snapshotJSON, err := activityJSON(snapshot)
	if err != nil {
		return err
	}
	detailsJSON, err := activityJSON(activityDetails(ctx))
	if err != nil {
		return err
	}
	var actor any
	if actorID, ok := ActivityActorFromContext(ctx); ok {
		actor = actorID
	}
	_, err = s.queryer.Exec(ctx, `INSERT INTO "activity" ("kind", "operation", "status", "entity_id", "record_id", "actor_id", "title", "message", "changes", "snapshot", "details", "name") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11::jsonb, $12)`,
		"record",
		operation,
		"success",
		layout.EntityID,
		recordID,
		actor,
		activityTitle(layout, operation),
		nil,
		changesJSON,
		snapshotJSON,
		detailsJSON,
		activityName,
	)
	if err != nil {
		return classifyRecordDBError(err, "activity")
	}
	return nil
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

func activityJSON(value any) (any, error) {
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
	return string(encoded), nil
}

func activityTitle(layout recordLayout, operation string) string {
	label := strings.TrimSpace(layout.Label)
	if label == "" {
		label = layout.Entity
	}
	switch operation {
	case activityOperationCreate:
		return "Created " + label
	case activityOperationUpdate:
		return "Updated " + label
	case activityOperationDelete:
		return "Deleted " + label
	default:
		return operation + " " + label
	}
}

func recordValuesEqual(left any, right any) bool {
	return reflect.DeepEqual(left, right)
}

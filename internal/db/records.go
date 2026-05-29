package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/hapyco/dygo/internal/auth"
	"github.com/hapyco/dygo/internal/corevalues"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/internal/entity/schema"
	"github.com/hapyco/dygo/internal/recordfilter"
	"github.com/hapyco/dygo/internal/recordquery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

const (
	RecordErrorInvalidRequest      = "invalid_request"
	RecordErrorValidation          = "validation_error"
	RecordErrorNotFound            = "not_found"
	RecordErrorConstraintViolation = "constraint_violation"
	RecordErrorSchemaNotReady      = "schema_not_ready"
	RecordErrorInternal            = "internal_error"
)

const (
	randomNameRetries = 5

	recordSelectSourceAlias = "_dygo_record"
	// TODO(collections): move row min/max limits into collection field metadata options.
	recordCollectionMaxRows = 500
)

// RecordQueryer is the database behavior needed by the Record store.
type RecordQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// RecordStore reads and mutates Records through persisted Entity metadata.
type RecordStore struct {
	queryer              RecordQueryer
	metadata             MetadataReader
	hooks                *RecordHookRegistry
	allowSystemMutations bool
}

// Record is one metadata-backed saved Entity instance.
type Record map[string]any

// RecordInput is the decoded request data for creating or updating a Record.
type RecordInput map[string]json.RawMessage

// RecordListParams controls Record list pagination.
type RecordListParams = recordquery.Params

// RecordFilter is one operator-based Record list filter on a metadata or system field.
type RecordFilter = recordquery.Filter

// RecordSort is one Record list sort term on a metadata or system field.
type RecordSort = recordquery.Sort

// RecordListResult is a page of Records.
type RecordListResult struct {
	Records []Record
	Limit   int
	Offset  int
	Count   int
	Total   int
}

// RecordMutationHookPolicy controls which Record hooks run for a RecordStore mutation.
type RecordMutationHookPolicy string

const (
	// RecordMutationHooksFrameworkOnly runs dygo framework hooks such as Activity, but no app hooks.
	RecordMutationHooksFrameworkOnly RecordMutationHookPolicy = "framework-only"
	// RecordMutationHooksNone suppresses all Record hooks.
	RecordMutationHooksNone RecordMutationHookPolicy = "none"
)

// RecordError reports stable Record runtime failures for API mapping.
type RecordError struct {
	Code    string
	Message string
	Details map[string]any
	Err     error
}

func (e RecordError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return e.Message + ": " + e.Err.Error()
}

func (e RecordError) Unwrap() error {
	return e.Err
}

// IsRecordError reports whether err is a RecordError.
func IsRecordError(err error) bool {
	var recordErr RecordError
	return errors.As(err, &recordErr)
}

// NewRecordStore returns a Record store backed by queryer with framework hooks enabled.
func NewRecordStore(queryer RecordQueryer) RecordStore {
	return NewRecordStoreWithHookPolicy(queryer, RecordMutationHooksFrameworkOnly)
}

// NewRecordStoreWithHookPolicy returns a Record store backed by queryer and an explicit hook policy.
func NewRecordStoreWithHookPolicy(queryer RecordQueryer, policy RecordMutationHookPolicy) RecordStore {
	switch policy {
	case RecordMutationHooksNone:
		return NewRecordStoreWithHooks(queryer, nil)
	case RecordMutationHooksFrameworkOnly, "":
		return NewRecordStoreWithHooks(queryer, DefaultRecordHookRegistry())
	default:
		return NewRecordStoreWithHooks(queryer, DefaultRecordHookRegistry())
	}
}

// NewRecordStoreWithHooks returns a Record store backed by queryer and hooks.
func NewRecordStoreWithHooks(queryer RecordQueryer, hooks *RecordHookRegistry) RecordStore {
	return RecordStore{queryer: queryer, metadata: NewMetadataReader(queryer), hooks: hooks}
}

// ListRecords returns one deterministic page of Records for entity.
func (s RecordStore) ListRecords(ctx context.Context, entity string, params RecordListParams) (RecordListResult, error) {
	if err := s.requireQueryer(); err != nil {
		return RecordListResult{}, err
	}
	params, err := normalizeRecordListParams(params)
	if err != nil {
		return RecordListResult{}, err
	}
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return RecordListResult{}, err
	}
	return s.listRecords(ctx, layout, entity, params)
}

// ListRecordsByIdentity returns one deterministic page of Records for an app-scoped Entity identity.
func (s RecordStore) ListRecordsByIdentity(ctx context.Context, appName string, entity string, params RecordListParams) (RecordListResult, error) {
	if err := s.requireQueryer(); err != nil {
		return RecordListResult{}, err
	}
	params, err := normalizeRecordListParams(params)
	if err != nil {
		return RecordListResult{}, err
	}
	layout, err := s.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return RecordListResult{}, err
	}
	return s.listRecords(ctx, layout, recordIdentityName(appName, entity), params)
}

func (s RecordStore) listRecords(ctx context.Context, layout recordLayout, entity string, params RecordListParams) (RecordListResult, error) {
	if layout.IsSingle {
		return RecordListResult{}, singleRecordOperationError(layout, "list")
	}
	if layout.IsCollection {
		return RecordListResult{}, collectionRecordOperationError(layout, "list")
	}
	query, err := s.listQuery(ctx, layout, params)
	if err != nil {
		return RecordListResult{}, err
	}
	sql := fmt.Sprintf("SELECT %s, COUNT(*) OVER() FROM %s AS %s", layout.selectList(), quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias))
	if query.Where != "" {
		sql += " WHERE " + query.Where
	}
	sql += " ORDER BY " + query.OrderBy
	args := append(query.Args, params.Limit, params.Offset)
	sql += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.queryer.Query(ctx, sql, args...)
	if err != nil {
		return RecordListResult{}, classifyRecordDBError(err, entity)
	}
	defer rows.Close()

	records := []Record{}
	total := 0
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return RecordListResult{}, recordError(RecordErrorInternal, "read record row failed", map[string]any{"entity": entity}, err)
		}
		if len(values) == layout.recordValueCount()+1 {
			total, err = scanRecordTotal(values[len(values)-1], entity)
			if err != nil {
				return RecordListResult{}, err
			}
			values = values[:len(values)-1]
		}
		record, err := layout.recordFromValues(values)
		if err != nil {
			return RecordListResult{}, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return RecordListResult{}, classifyRecordDBError(err, entity)
	}
	if total == 0 {
		total = len(records)
	}
	return RecordListResult{Records: records, Limit: params.Limit, Offset: params.Offset, Count: len(records), Total: total}, nil
}

type recordListQuery struct {
	Where   string
	OrderBy string
	Args    []any
}

// GetRecord returns one Record by id.
func (s RecordStore) GetRecord(ctx context.Context, entity string, id int64) (Record, error) {
	if err := s.requireQueryer(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, invalidRecordIDError(entity)
	}
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return nil, err
	}
	return s.getRecordWithLayout(ctx, layout, id)
}

// GetRecordByIdentity returns one Record by app-scoped Entity identity and id.
func (s RecordStore) GetRecordByIdentity(ctx context.Context, appName string, entity string, id int64) (Record, error) {
	if err := s.requireQueryer(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, invalidRecordIDError(recordIdentityName(appName, entity))
	}
	layout, err := s.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return nil, err
	}
	return s.getRecordWithLayout(ctx, layout, id)
}

// FindRecord returns one Record matching metadata field values.
func (s RecordStore) FindRecord(ctx context.Context, entity string, match RecordInput) (Record, error) {
	if err := s.requireQueryer(); err != nil {
		return nil, err
	}
	match = normalizeRecordInput(match)
	if len(match) == 0 {
		return nil, recordError(RecordErrorInvalidRequest, "at least one match field is required", map[string]any{"entity": entity}, nil)
	}
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return nil, err
	}
	return s.findRecordWithLayout(ctx, layout, entity, match)
}

// FindRecordByIdentity returns one Record matching metadata field values for an app-scoped Entity identity.
func (s RecordStore) FindRecordByIdentity(ctx context.Context, appName string, entity string, match RecordInput) (Record, error) {
	if err := s.requireQueryer(); err != nil {
		return nil, err
	}
	match = normalizeRecordInput(match)
	identity := recordIdentityName(appName, entity)
	if len(match) == 0 {
		return nil, recordError(RecordErrorInvalidRequest, "at least one match field is required", map[string]any{"entity": identity}, nil)
	}
	layout, err := s.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return nil, err
	}
	return s.findRecordWithLayout(ctx, layout, identity, match)
}

// GetSingleRecord returns the framework-owned singleton Record for a single Entity.
func (s RecordStore) GetSingleRecord(ctx context.Context, entity string) (Record, error) {
	if err := s.requireQueryer(); err != nil {
		return nil, err
	}
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return nil, err
	}
	return s.getSingleRecordWithLayout(ctx, layout)
}

// UpdateSingleRecord partially updates the framework-owned singleton Record for a single Entity.
func (s RecordStore) UpdateSingleRecord(ctx context.Context, entity string, input RecordInput) (Record, error) {
	if err := s.requireQueryer(); err != nil {
		return nil, err
	}
	return s.withRecordMutation(ctx, func(store RecordStore) (Record, error) {
		layout, err := store.recordLayout(ctx, entity)
		if err != nil {
			return nil, err
		}
		if err := store.rejectSystemMutation(layout, "update"); err != nil {
			return nil, err
		}
		record, err := store.getSingleRecordWithLayout(ctx, layout)
		if err != nil {
			return nil, err
		}
		id, err := activityRecordID(record)
		if err != nil {
			return nil, err
		}
		return store.updateRecordWithLayout(ctx, layout, id, input)
	})
}

func (s RecordStore) findRecordWithLayout(ctx context.Context, layout recordLayout, entity string, match RecordInput) (Record, error) {
	if layout.IsCollection {
		return nil, collectionRecordOperationError(layout, "read")
	}
	if err := layout.validateMatchFields(match); err != nil {
		return nil, err
	}
	mutation, err := s.matchMutation(ctx, layout, match)
	if err != nil {
		return nil, err
	}
	clauses := make([]string, 0, len(mutation.Columns))
	for i, column := range mutation.Columns {
		clauses = append(clauses, fmt.Sprintf("%s = %s", quoteIdent(column), mutation.Placeholders[i]))
	}
	sql := fmt.Sprintf("SELECT %s FROM %s AS %s WHERE %s ORDER BY %s ASC LIMIT 2", layout.selectList(), quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias), strings.Join(clauses, " AND "), quoteIdent("id"))
	rows, err := s.queryer.Query(ctx, sql, mutation.Values...)
	if err != nil {
		return nil, classifyRecordDBError(err, entity)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, classifyRecordDBError(err, entity)
		}
		return nil, recordError(RecordErrorNotFound, "record not found", map[string]any{"entity": entity}, nil)
	}
	values, err := rows.Values()
	if err != nil {
		return nil, recordError(RecordErrorInternal, "read record row failed", map[string]any{"entity": entity}, err)
	}
	record, err := layout.recordFromValues(values)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		return nil, recordError(RecordErrorValidation, "record match is ambiguous", map[string]any{"entity": entity}, nil)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyRecordDBError(err, entity)
	}
	return s.loadRecordCollections(ctx, layout, record)
}

// CreateRecord inserts one Record using Entity metadata.
func (s RecordStore) CreateRecord(ctx context.Context, entity string, input RecordInput) (Record, error) {
	if err := s.requireQueryer(); err != nil {
		return nil, err
	}
	return s.withRecordMutation(ctx, func(store RecordStore) (Record, error) {
		return store.createRecord(ctx, entity, input)
	})
}

// CreateRecordByIdentity inserts one Record using app-scoped Entity identity.
func (s RecordStore) CreateRecordByIdentity(ctx context.Context, appName string, entity string, input RecordInput) (Record, error) {
	if err := s.requireQueryer(); err != nil {
		return nil, err
	}
	return s.withRecordMutation(ctx, func(store RecordStore) (Record, error) {
		return store.createRecordByIdentity(ctx, appName, entity, input)
	})
}

func (s RecordStore) createRecord(ctx context.Context, entity string, input RecordInput) (Record, error) {
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return nil, err
	}
	return s.createRecordWithLayout(ctx, layout, input)
}

func (s RecordStore) createRecordByIdentity(ctx context.Context, appName string, entity string, input RecordInput) (Record, error) {
	layout, err := s.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return nil, err
	}
	return s.createRecordWithLayout(ctx, layout, input)
}

func (s RecordStore) createRecordWithLayout(ctx context.Context, layout recordLayout, input RecordInput) (Record, error) {
	if layout.IsSingle {
		return nil, singleRecordOperationError(layout, "create")
	}
	if layout.IsCollection {
		return nil, collectionRecordOperationError(layout, "create")
	}
	if err := s.rejectSystemMutation(layout, "create"); err != nil {
		return nil, err
	}
	input = cloneRecordInput(input)
	hookCtx := newRecordHookContext(RecordBeforeValidate, layout)
	hookCtx.Operation = corevalues.ActivityOperationCreate
	hookCtx.Input = input
	if err := s.runRecordHooks(ctx, hookCtx); err != nil {
		return nil, err
	}
	if err := s.applyFetchedFields(ctx, layout, input, nil); err != nil {
		return nil, err
	}
	if err := layout.validateCreateInput(input); err != nil {
		return nil, err
	}
	hookCtx.Event = RecordValidate
	if err := s.runRecordHooks(ctx, hookCtx); err != nil {
		return nil, err
	}
	hookCtx.Event = RecordBeforeCreate
	if err := s.runRecordHooks(ctx, hookCtx); err != nil {
		return nil, err
	}

	record, err := s.insertRecordWithLayout(ctx, layout, input, true)
	if err != nil {
		return nil, err
	}
	recordID, err := activityRecordID(record)
	if err != nil {
		return nil, err
	}
	if err := s.saveSubmittedCollections(ctx, layout, recordID, input, true); err != nil {
		return nil, err
	}
	record, err = s.loadRecordCollections(ctx, layout, record)
	if err != nil {
		return nil, err
	}
	afterCtx := newRecordHookContext(RecordAfterCreate, layout)
	afterCtx.Operation = corevalues.ActivityOperationCreate
	afterCtx.RecordID = recordID
	afterCtx.Input = input
	afterCtx.NewRecord = record
	afterCtx.Snapshot = record
	if err := s.runRecordHooks(ctx, afterCtx); err != nil {
		return nil, err
	}
	return record, nil
}

func (s RecordStore) insertRecordWithLayout(ctx context.Context, layout recordLayout, input RecordInput, returning bool) (Record, error) {
	for attempt := 0; attempt <= randomNameRetries; attempt++ {
		mutation, err := s.createMutation(ctx, layout, input)
		if err != nil {
			return nil, err
		}
		sql := insertRecordSQL(layout, mutation, returning)
		if returning {
			record, err := s.queryReturningRecord(ctx, layout, sql, mutation.Values, false)
			if err == nil {
				return record, nil
			}
			if layout.Naming.Strategy != schema.NamingStrategyRandom || !isRecordNameCollision(err, layout) || attempt == randomNameRetries {
				return nil, err
			}
			continue
		}
		if _, err := s.queryer.Exec(ctx, sql, mutation.Values...); err == nil {
			return nil, nil
		} else {
			err = classifyRecordDBError(err, layout.Entity)
			if layout.Naming.Strategy != schema.NamingStrategyRandom || !isRecordNameCollision(err, layout) || attempt == randomNameRetries {
				return nil, err
			}
		}
	}
	return nil, recordError(RecordErrorInternal, "record insert failed", map[string]any{"entity": layout.Entity}, nil)
}

func insertRecordSQL(layout recordLayout, mutation recordMutation, returning bool) string {
	if len(mutation.Columns) == 0 {
		sql := fmt.Sprintf("INSERT INTO %s AS %s DEFAULT VALUES", quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias))
		if returning {
			sql += " RETURNING " + layout.selectList()
		}
		return sql
	}
	sql := fmt.Sprintf("INSERT INTO %s AS %s (%s) VALUES (%s)", quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias), quoteIdentList(mutation.Columns), strings.Join(mutation.Placeholders, ", "))
	if returning {
		sql += " RETURNING " + layout.selectList()
	}
	return sql
}

func isRecordNameCollision(err error, layout recordLayout) bool {
	var recordErr RecordError
	if !errors.As(err, &recordErr) || recordErr.Code != RecordErrorConstraintViolation {
		return false
	}
	constraint, _ := recordErr.Details["constraint"].(string)
	return constraint == constraintName(layout.Table, "name", "key")
}

// UpdateRecord partially updates one Record by id.
func (s RecordStore) UpdateRecord(ctx context.Context, entity string, id int64, input RecordInput) (Record, error) {
	if err := s.requireQueryer(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, invalidRecordIDError(entity)
	}
	return s.withRecordMutation(ctx, func(store RecordStore) (Record, error) {
		return store.updateRecord(ctx, entity, id, input)
	})
}

// UpdateRecordByIdentity partially updates one Record by app-scoped Entity identity and id.
func (s RecordStore) UpdateRecordByIdentity(ctx context.Context, appName string, entity string, id int64, input RecordInput) (Record, error) {
	if err := s.requireQueryer(); err != nil {
		return nil, err
	}
	if id <= 0 {
		return nil, invalidRecordIDError(recordIdentityName(appName, entity))
	}
	return s.withRecordMutation(ctx, func(store RecordStore) (Record, error) {
		return store.updateRecordByIdentity(ctx, appName, entity, id, input)
	})
}

func (s RecordStore) updateRecord(ctx context.Context, entity string, id int64, input RecordInput) (Record, error) {
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return nil, err
	}
	return s.updateRecordWithLayout(ctx, layout, id, input)
}

func (s RecordStore) updateRecordByIdentity(ctx context.Context, appName string, entity string, id int64, input RecordInput) (Record, error) {
	layout, err := s.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return nil, err
	}
	return s.updateRecordWithLayout(ctx, layout, id, input)
}

func (s RecordStore) updateRecordWithLayout(ctx context.Context, layout recordLayout, id int64, input RecordInput) (Record, error) {
	if err := s.rejectSystemMutation(layout, "update"); err != nil {
		return nil, err
	}
	input = cloneRecordInput(input)
	oldRecord, err := s.getRecordWithLayout(ctx, layout, id)
	if err != nil {
		return nil, err
	}
	hookCtx := newRecordHookContext(RecordBeforeValidate, layout)
	hookCtx.Operation = corevalues.ActivityOperationUpdate
	hookCtx.RecordID = id
	hookCtx.Input = input
	hookCtx.OldRecord = oldRecord
	if err := s.runRecordHooks(ctx, hookCtx); err != nil {
		return nil, err
	}
	if err := s.applyFetchedFields(ctx, layout, input, oldRecord); err != nil {
		return nil, err
	}
	if err := layout.validateUpdateFields(input); err != nil {
		return nil, err
	}
	hookCtx.Event = RecordValidate
	if err := s.runRecordHooks(ctx, hookCtx); err != nil {
		return nil, err
	}
	beforeCtx := newRecordHookContext(RecordBeforeUpdate, layout)
	beforeCtx.Operation = corevalues.ActivityOperationUpdate
	beforeCtx.RecordID = id
	beforeCtx.Input = input
	beforeCtx.OldRecord = oldRecord
	if err := s.runRecordHooks(ctx, beforeCtx); err != nil {
		return nil, err
	}
	mutation, err := s.updateMutation(ctx, layout, input)
	if err != nil {
		return nil, err
	}

	setClauses := make([]string, 0, len(mutation.Columns)+1)
	for i, column := range mutation.Columns {
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", quoteIdent(column), mutation.Placeholders[i]))
	}
	setClauses = append(setClauses, fmt.Sprintf("%s = now()", quoteIdent("updated_at")))
	args := append([]any(nil), mutation.Values...)
	args = append(args, id)
	sql := fmt.Sprintf("UPDATE %s AS %s SET %s WHERE %s = $%d RETURNING %s", quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias), strings.Join(setClauses, ", "), quoteIdent("id"), len(args), layout.selectList())
	record, err := s.queryReturningRecord(ctx, layout, sql, args, true)
	if err != nil {
		return nil, err
	}
	recordID, err := activityRecordID(record)
	if err != nil {
		return nil, err
	}
	if err := s.saveSubmittedCollections(ctx, layout, recordID, input, false); err != nil {
		return nil, err
	}
	record, err = s.loadRecordCollections(ctx, layout, record)
	if err != nil {
		return nil, err
	}
	changes := layout.activityChanges(input, oldRecord, record)
	recordID, err = activityRecordID(record)
	if err != nil {
		return nil, err
	}
	afterCtx := newRecordHookContext(RecordAfterUpdate, layout)
	afterCtx.Operation = corevalues.ActivityOperationUpdate
	afterCtx.RecordID = recordID
	afterCtx.Input = input
	afterCtx.OldRecord = oldRecord
	afterCtx.NewRecord = record
	afterCtx.Changes = changes
	if err := s.runRecordHooks(ctx, afterCtx); err != nil {
		return nil, err
	}
	return record, nil
}

// DeleteRecord hard-deletes one Record by id.
func (s RecordStore) DeleteRecord(ctx context.Context, entity string, id int64) error {
	if err := s.requireQueryer(); err != nil {
		return err
	}
	if id <= 0 {
		return invalidRecordIDError(entity)
	}
	_, err := s.withRecordMutation(ctx, func(store RecordStore) (Record, error) {
		return nil, store.deleteRecord(ctx, entity, id)
	})
	return err
}

// DeleteRecordByIdentity hard-deletes one Record by app-scoped Entity identity and id.
func (s RecordStore) DeleteRecordByIdentity(ctx context.Context, appName string, entity string, id int64) error {
	if err := s.requireQueryer(); err != nil {
		return err
	}
	if id <= 0 {
		return invalidRecordIDError(recordIdentityName(appName, entity))
	}
	_, err := s.withRecordMutation(ctx, func(store RecordStore) (Record, error) {
		return nil, store.deleteRecordByIdentity(ctx, appName, entity, id)
	})
	return err
}

func (s RecordStore) deleteRecord(ctx context.Context, entity string, id int64) error {
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return err
	}
	return s.deleteRecordWithLayout(ctx, layout, entity, id)
}

func (s RecordStore) deleteRecordByIdentity(ctx context.Context, appName string, entity string, id int64) error {
	layout, err := s.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return err
	}
	return s.deleteRecordWithLayout(ctx, layout, recordIdentityName(appName, entity), id)
}

func (s RecordStore) deleteRecordWithLayout(ctx context.Context, layout recordLayout, entity string, id int64) error {
	if layout.IsSingle {
		return singleRecordOperationError(layout, "delete")
	}
	if layout.IsCollection {
		return collectionRecordOperationError(layout, "delete")
	}
	if err := s.rejectSystemMutation(layout, "delete"); err != nil {
		return err
	}
	oldRecord, err := s.getRecordWithLayout(ctx, layout, id)
	if err != nil {
		return err
	}
	beforeCtx := newRecordHookContext(RecordBeforeDelete, layout)
	beforeCtx.Operation = corevalues.ActivityOperationDelete
	beforeCtx.RecordID = id
	beforeCtx.OldRecord = oldRecord
	if err := s.runRecordHooks(ctx, beforeCtx); err != nil {
		return err
	}
	if err := s.deleteRecordCollections(ctx, layout, id); err != nil {
		return err
	}
	tag, err := s.queryer.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE %s = $1", quoteIdent(layout.Table), quoteIdent("id")), id)
	if err != nil {
		return classifyRecordDBError(err, entity)
	}
	if tag.RowsAffected() == 0 {
		return recordError(RecordErrorNotFound, "record not found", map[string]any{"entity": entity, "id": id}, nil)
	}
	recordID, err := activityRecordID(oldRecord)
	if err != nil {
		return err
	}
	afterCtx := newRecordHookContext(RecordAfterDelete, layout)
	afterCtx.Operation = corevalues.ActivityOperationDelete
	afterCtx.RecordID = recordID
	afterCtx.OldRecord = oldRecord
	afterCtx.Snapshot = oldRecord
	if err := s.runRecordHooks(ctx, afterCtx); err != nil {
		return err
	}
	return nil
}

func (s RecordStore) queryOneRecord(ctx context.Context, layout recordLayout, sql string, args ...any) (Record, error) {
	return s.queryReturningRecord(ctx, layout, sql, args, true)
}

func (s RecordStore) getSingleRecordWithLayout(ctx context.Context, layout recordLayout) (Record, error) {
	if !layout.IsSingle {
		return nil, recordError(RecordErrorInvalidRequest, "entity is not single", map[string]any{"entity": layout.Slug}, nil)
	}
	sql := fmt.Sprintf("SELECT %s FROM %s AS %s WHERE %s = $1", layout.selectList(), quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias), quoteIdent("name"))
	record, err := s.queryOneRecord(ctx, layout, sql, SingleRecordName(layout.Entity))
	if err != nil {
		return nil, err
	}
	return s.loadRecordCollections(ctx, layout, record)
}

func (s RecordStore) getRecordWithLayout(ctx context.Context, layout recordLayout, id int64) (Record, error) {
	if layout.IsCollection {
		return nil, collectionRecordOperationError(layout, "read")
	}
	sql := fmt.Sprintf("SELECT %s FROM %s AS %s WHERE %s = $1", layout.selectList(), quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias), quoteIdent("id"))
	record, err := s.queryOneRecord(ctx, layout, sql, id)
	if err != nil {
		var recordErr RecordError
		if errors.As(err, &recordErr) && recordErr.Code == RecordErrorNotFound {
			recordErr.Details = map[string]any{"entity": layout.Entity, "id": id}
			return nil, recordErr
		}
		return nil, err
	}
	return s.loadRecordCollections(ctx, layout, record)
}

func singleRecordOperationError(layout recordLayout, operation string) RecordError {
	return recordError(RecordErrorInvalidRequest, fmt.Sprintf("single Entity records cannot use %s through this endpoint", operation), map[string]any{"entity": layout.Slug, "operation": operation}, nil)
}

func collectionRecordOperationError(layout recordLayout, operation string) RecordError {
	return recordError(RecordErrorInvalidRequest, fmt.Sprintf("collection Entity records cannot use %s through normal record endpoints", operation), map[string]any{"entity": layout.Slug, "operation": operation}, nil)
}

func (s RecordStore) rejectSystemMutation(layout recordLayout, operation string) error {
	if !layout.IsSystem || s.allowSystemMutations {
		return nil
	}
	return systemRecordOperationError(layout, operation)
}

func systemRecordOperationError(layout recordLayout, operation string) RecordError {
	return recordError(RecordErrorInvalidRequest, "system Entity records are framework-owned", map[string]any{"entity": layout.Slug, "operation": operation}, nil)
}

func (s RecordStore) queryReturningRecord(ctx context.Context, layout recordLayout, sql string, args []any, notFoundWhenEmpty bool) (Record, error) {
	rows, err := s.queryer.Query(ctx, sql, args...)
	if err != nil {
		return nil, classifyRecordDBError(err, layout.Entity)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, classifyRecordDBError(err, layout.Entity)
		}
		if notFoundWhenEmpty {
			return nil, recordError(RecordErrorNotFound, "record not found", map[string]any{"entity": layout.Entity}, nil)
		}
		return nil, recordError(RecordErrorInternal, "record query returned no rows", map[string]any{"entity": layout.Entity}, nil)
	}
	values, err := rows.Values()
	if err != nil {
		return nil, recordError(RecordErrorInternal, "read record row failed", map[string]any{"entity": layout.Entity}, err)
	}
	record, err := layout.recordFromValues(values)
	if err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, classifyRecordDBError(err, layout.Entity)
	}
	return record, nil
}

func (s RecordStore) recordLayout(ctx context.Context, entity string) (recordLayout, error) {
	meta, err := s.metadata.GetEntityMeta(ctx, entity)
	if err != nil {
		if IsMetadataNotFound(err) {
			return recordLayout{}, recordError(RecordErrorNotFound, "entity not found", map[string]any{"entity": entity}, err)
		}
		var classified RecordError
		if errors.As(classifyRecordDBError(err, entity), &classified) && classified.Code == RecordErrorSchemaNotReady {
			return recordLayout{}, classified
		}
		return recordLayout{}, recordError(RecordErrorInternal, "load entity metadata failed", map[string]any{"entity": entity}, err)
	}
	return newRecordLayout(meta)
}

func (s RecordStore) recordLayoutByIdentity(ctx context.Context, appName string, entity string) (recordLayout, error) {
	meta, err := s.metadata.GetEntityMetaByIdentity(ctx, appName, entity)
	if err != nil {
		if IsMetadataNotFound(err) {
			return recordLayout{}, recordError(RecordErrorNotFound, "entity not found", map[string]any{"app": appName, "entity": entity}, err)
		}
		identity := recordIdentityName(appName, entity)
		var classified RecordError
		if errors.As(classifyRecordDBError(err, identity), &classified) && classified.Code == RecordErrorSchemaNotReady {
			return recordLayout{}, classified
		}
		return recordLayout{}, recordError(RecordErrorInternal, "load entity metadata failed", map[string]any{"app": appName, "entity": entity}, err)
	}
	return newRecordLayout(meta)
}

func recordIdentityName(appName string, entity string) string {
	return appName + "/" + entity
}

func (s RecordStore) requireQueryer() error {
	if s.queryer == nil {
		return recordError(RecordErrorInternal, "record queryer is required", nil, nil)
	}
	return nil
}

func (s RecordStore) runRecordHooks(ctx context.Context, hookCtx RecordHookContext) error {
	if s.hooks == nil {
		return nil
	}
	hookCtx.Queryer = s.queryer
	return s.hooks.Run(ctx, hookCtx)
}

type recordLayout struct {
	EntityID     int64
	AppName      string
	Entity       string
	Slug         string
	Label        string
	IsSingle     bool
	IsSystem     bool
	IsCollection bool
	Table        string
	Naming       schema.Naming
	Fields       []recordField
	FieldByName  map[string]recordField
	Collections  map[string]recordCollection
}

type recordField struct {
	ID          int64
	Name        string
	Type        string
	Required    bool
	Default     json.RawMessage
	Fetch       recordFieldFetch
	Options     recordFieldOptions
	Column      string
	Storage     bool
	WriteOnly   bool
	Listable    bool
	Nameable    bool
	ValueKind   string
	SystemName  bool
	SelectOrder int
}

type recordFieldOptions struct {
	App        string   `json:"app,omitempty"`
	Values     []string `json:"values,omitempty"`
	Entity     string   `json:"entity,omitempty"`
	ForeignKey *bool    `json:"foreign-key,omitempty"`
}

type recordFieldFetch struct {
	From string `json:"from,omitempty"`
}

type recordCollection struct {
	Field          recordField
	ParentEntityID int64
	Layout         *recordLayout
}

type recordCollectionRowInput struct {
	ID      int64
	Ordinal int64
	Input   RecordInput
}

type recordMutation struct {
	Columns      []string
	Placeholders []string
	Values       []any
}

func newRecordLayout(meta MetadataEntityMeta) (recordLayout, error) {
	var naming schema.Naming
	var err error
	if meta.IsCollection {
		naming = schema.CollectionRowNaming()
	} else {
		naming, err = parseRecordNaming(meta.Naming)
	}
	routeSlug := meta.RouteSlug()
	if routeSlug == "" {
		routeSlug = meta.Key
	}
	if err != nil {
		return recordLayout{}, recordError(RecordErrorInternal, "entity naming metadata is invalid", map[string]any{"entity": routeSlug}, err)
	}
	layout := recordLayout{
		EntityID:     meta.ID,
		AppName:      meta.App.Name,
		Entity:       meta.Key,
		Slug:         routeSlug,
		Label:        meta.Label,
		IsSingle:     meta.IsSingle,
		IsSystem:     meta.IsSystem,
		IsCollection: meta.IsCollection,
		Table:        entityTableName(meta.App.Name, meta.Key),
		Naming:       naming,
		FieldByName:  map[string]recordField{},
		Collections:  map[string]recordCollection{},
	}
	if naming.Strategy == schema.NamingStrategyManual {
		nameField, ok := systemRecordFieldByName(systemFieldName)
		if !ok {
			return recordLayout{}, recordError(RecordErrorInternal, "system name field metadata is missing", map[string]any{"entity": routeSlug}, nil)
		}
		field := recordField{
			Name:       nameField.Name,
			Type:       nameField.Type,
			Required:   true,
			Storage:    true,
			Listable:   nameField.Listable,
			Nameable:   nameField.Nameable,
			ValueKind:  nameField.ValueKind,
			SystemName: true,
			Column:     nameField.Column,
		}
		layout.Fields = append(layout.Fields, field)
		layout.FieldByName[field.Name] = field
	}
	for _, metadataField := range meta.Fields {
		definition, ok := fieldtype.DefaultDefinition(metadataField.Type)
		if !ok {
			return recordLayout{}, recordError(RecordErrorInternal, "field type metadata is invalid", map[string]any{"entity": routeSlug, "field": metadataField.Name, "type": metadataField.Type}, nil)
		}
		field := recordField{
			ID:        metadataField.ID,
			Name:      metadataField.Name,
			Type:      metadataField.Type,
			Required:  metadataField.Required,
			Default:   metadataField.Default,
			Storage:   definition.Behavior.Stored,
			WriteOnly: definition.Behavior.WriteOnly,
			Listable:  definition.Behavior.Listable,
			Nameable:  definition.Behavior.NameRenderable,
			ValueKind: definition.Behavior.ValueKind,
		}
		if len(metadataField.Options) > 0 {
			if err := json.Unmarshal(metadataField.Options, &field.Options); err != nil {
				return recordLayout{}, recordError(RecordErrorInternal, "field options metadata is invalid", map[string]any{"entity": routeSlug, "field": metadataField.Name}, err)
			}
		}
		if len(metadataField.Fetch) > 0 {
			if err := json.Unmarshal(metadataField.Fetch, &field.Fetch); err != nil {
				return recordLayout{}, recordError(RecordErrorInternal, "field fetch metadata is invalid", map[string]any{"entity": routeSlug, "field": metadataField.Name}, err)
			}
		}
		if field.Storage && !field.SystemName {
			field.Column = recordColumnForField(field.Name, field.Type)
			field.SelectOrder = len(layout.Fields) + 3
		} else if field.SystemName {
			field.Column = "name"
		}
		layout.Fields = append(layout.Fields, field)
		layout.FieldByName[field.Name] = field
	}
	for fieldName, childMeta := range meta.Collections {
		field, ok := layout.FieldByName[fieldName]
		if !ok || field.Type != "collection" {
			return recordLayout{}, recordError(RecordErrorInternal, "collection metadata does not match parent field", map[string]any{"entity": routeSlug, "field": fieldName}, nil)
		}
		childLayout, err := newRecordLayout(childMeta)
		if err != nil {
			return recordLayout{}, err
		}
		if field.ID <= 0 {
			return recordLayout{}, recordError(RecordErrorInternal, "collection field metadata id is missing", map[string]any{"entity": routeSlug, "field": fieldName}, nil)
		}
		layout.Collections[fieldName] = recordCollection{Field: field, ParentEntityID: layout.EntityID, Layout: &childLayout}
	}
	return layout, nil
}

func (l recordLayout) selectList() string {
	systemColumns := systemRecordSelectColumns()
	columns := make([]string, 0, len(systemColumns)+len(l.Fields))
	for _, column := range systemColumns {
		columns = append(columns, recordSourceColumn(column))
	}
	for _, field := range l.Fields {
		if field.Storage && !field.WriteOnly && !field.SystemName {
			columns = append(columns, linkValueCodec{}.displaySQL(l, field))
		}
	}
	return strings.Join(columns, ", ")
}

func recordSourceColumn(column string) string {
	return quoteIdent(recordSelectSourceAlias) + "." + quoteIdent(column)
}

func (l recordLayout) recordValueCount() int {
	expected := 4
	for _, field := range l.Fields {
		if field.Storage && !field.WriteOnly && !field.SystemName {
			expected++
		}
	}
	return expected
}

func (l recordLayout) collectionNames() []string {
	names := make([]string, 0, len(l.Collections))
	seen := map[string]struct{}{}
	for _, field := range l.Fields {
		if _, ok := l.Collections[field.Name]; !ok {
			continue
		}
		names = append(names, field.Name)
		seen[field.Name] = struct{}{}
	}
	extra := []string{}
	for name := range l.Collections {
		if _, ok := seen[name]; ok {
			continue
		}
		extra = append(extra, name)
	}
	sortStrings(extra)
	return append(names, extra...)
}

func (l recordLayout) recordFromValues(values []any) (Record, error) {
	expected := l.recordValueCount()
	if len(values) != expected {
		return nil, recordError(RecordErrorInternal, "record column count did not match metadata", map[string]any{"entity": l.Entity, "expected": expected, "actual": len(values)}, nil)
	}
	record := Record{
		systemFieldID:        normalizeRecordValue("", values[0]),
		systemFieldName:      normalizeRecordValue("text", values[1]),
		systemFieldCreatedAt: normalizeRecordValue("datetime", values[2]),
		systemFieldUpdatedAt: normalizeRecordValue("datetime", values[3]),
	}
	index := 4
	for _, field := range l.Fields {
		if !field.Storage || field.WriteOnly || field.SystemName {
			continue
		}
		record[field.Name] = normalizeRecordValue(field.Type, values[index])
		index++
	}
	return record, nil
}

func scanRecordTotal(value any, entity string) (int, error) {
	switch total := value.(type) {
	case int:
		return total, nil
	case int32:
		return int(total), nil
	case int64:
		if total > int64(math.MaxInt) {
			return 0, recordError(RecordErrorInternal, "record total count is too large", map[string]any{"entity": entity}, nil)
		}
		return int(total), nil
	default:
		return 0, recordError(RecordErrorInternal, "record total count type was invalid", map[string]any{"entity": entity, "type": fmt.Sprintf("%T", value)}, nil)
	}
}

func (s RecordStore) listQuery(ctx context.Context, layout recordLayout, params RecordListParams) (recordListQuery, error) {
	where, args, err := s.listWhere(ctx, layout, params.Filters)
	if err != nil {
		return recordListQuery{}, err
	}
	orderBy, err := layout.listOrderBy(params.Sort)
	if err != nil {
		return recordListQuery{}, err
	}
	return recordListQuery{Where: where, OrderBy: orderBy, Args: args}, nil
}

func (s RecordStore) listWhere(ctx context.Context, layout recordLayout, filters []RecordFilter) (string, []any, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	filters = sortedRecordFilters(filters)
	clauses := make([]string, 0, len(filters))
	args := make([]any, 0, len(filters))
	for _, filter := range filters {
		fieldName := strings.TrimSpace(filter.Field)
		if fieldName == "" {
			return "", nil, recordError(RecordErrorInvalidRequest, "filter field is required", map[string]any{"entity": layout.Entity}, nil)
		}
		field, err := layout.listField(fieldName, "filter")
		if err != nil {
			return "", nil, err
		}
		clause, clauseArgs, err := s.recordFilterClause(ctx, layout, field, filter, len(args))
		if err != nil {
			return "", nil, err
		}
		args = append(args, clauseArgs...)
		clauses = append(clauses, clause)
	}
	return strings.Join(clauses, " AND "), args, nil
}

func (s RecordStore) recordFilterClause(ctx context.Context, layout recordLayout, field recordField, filter RecordFilter, argOffset int) (string, []any, error) {
	operator := strings.TrimSpace(filter.Operator)
	if operator == "" {
		return "", nil, recordError(RecordErrorInvalidRequest, "filter operator is required", map[string]any{"entity": layout.Entity, "field": field.Name}, nil)
	}
	if !recordfilter.Supports(field.Type, field.ValueKind, operator) {
		return "", nil, recordError(RecordErrorInvalidRequest, "filter operator is not supported for field", map[string]any{"entity": layout.Entity, "field": field.Name, "operator": operator}, nil)
	}

	column := quoteIdent(field.Column)
	switch operator {
	case recordfilter.OperatorEmpty:
		if strings.TrimSpace(filter.Value) != "" {
			return "", nil, recordError(RecordErrorInvalidRequest, "filter value is not supported by this operator", map[string]any{"entity": layout.Entity, "field": field.Name, "operator": operator}, nil)
		}
		if recordFilterUsesBlankString(field) {
			return fmt.Sprintf("(%s IS NULL OR %s = '')", column, column), nil, nil
		}
		return column + " IS NULL", nil, nil
	case recordfilter.OperatorNotEmpty:
		if strings.TrimSpace(filter.Value) != "" {
			return "", nil, recordError(RecordErrorInvalidRequest, "filter value is not supported by this operator", map[string]any{"entity": layout.Entity, "field": field.Name, "operator": operator}, nil)
		}
		if recordFilterUsesBlankString(field) {
			return fmt.Sprintf("(%s IS NOT NULL AND %s <> '')", column, column), nil, nil
		}
		return column + " IS NOT NULL", nil, nil
	case recordfilter.OperatorBetween:
		start, end, err := parseRangeFilterValue(layout, field, filter.Value)
		if err != nil {
			return "", nil, err
		}
		startValue, err := s.recordListValue(ctx, layout, field, start)
		if err != nil {
			return "", nil, err
		}
		endValue, err := s.recordListValue(ctx, layout, field, end)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s BETWEEN %s AND %s", column, recordPlaceholder(argOffset+1, field), recordPlaceholder(argOffset+2, field)), []any{startValue, endValue}, nil
	case recordfilter.OperatorContains, recordfilter.OperatorNotContains:
		value, err := s.recordListValue(ctx, layout, field, filter.Value)
		if err != nil {
			return "", nil, err
		}
		comparator := "ILIKE"
		if operator == recordfilter.OperatorNotContains {
			comparator = "NOT ILIKE"
		}
		return fmt.Sprintf("%s %s '%%' || %s || '%%'", column, comparator, recordPlaceholder(argOffset+1, field)), []any{value}, nil
	default:
		value, err := s.recordListValue(ctx, layout, field, filter.Value)
		if err != nil {
			return "", nil, err
		}
		comparator, ok := recordFilterComparator(operator)
		if !ok {
			return "", nil, recordError(RecordErrorInvalidRequest, "filter operator is not supported for field", map[string]any{"entity": layout.Entity, "field": field.Name, "operator": operator}, nil)
		}
		return fmt.Sprintf("%s %s %s", column, comparator, recordPlaceholder(argOffset+1, field)), []any{value}, nil
	}
}

func recordFilterComparator(operator string) (string, bool) {
	switch operator {
	case recordfilter.OperatorEqual:
		return "=", true
	case recordfilter.OperatorNotEqual:
		return "<>", true
	case recordfilter.OperatorGreaterThan, recordfilter.OperatorAfter:
		return ">", true
	case recordfilter.OperatorGreaterThanOrEqual:
		return ">=", true
	case recordfilter.OperatorLessThan, recordfilter.OperatorBefore:
		return "<", true
	case recordfilter.OperatorLessThanOrEqual:
		return "<=", true
	default:
		return "", false
	}
}

func parseRangeFilterValue(layout recordLayout, field recordField, value string) (string, string, error) {
	start, end, ok := strings.Cut(value, "..")
	start = strings.TrimSpace(start)
	end = strings.TrimSpace(end)
	if !ok || start == "" || end == "" {
		return "", "", recordError(RecordErrorInvalidRequest, "filter range must use start..end", map[string]any{"entity": layout.Entity, "field": field.Name}, nil)
	}
	return start, end, nil
}

func recordFilterUsesBlankString(field recordField) bool {
	return field.ValueKind == fieldtype.ValueString
}

func (l recordLayout) listOrderBy(sortTerms []RecordSort) (string, error) {
	if len(sortTerms) == 0 {
		return quoteIdent("id") + " ASC", nil
	}
	seen := map[string]struct{}{}
	terms := make([]string, 0, len(sortTerms)+1)
	hasID := false
	for _, sortTerm := range sortTerms {
		fieldName := strings.TrimSpace(sortTerm.Field)
		if fieldName == "" {
			return "", recordError(RecordErrorInvalidRequest, "sort field is required", map[string]any{"entity": l.Entity}, nil)
		}
		if _, ok := seen[fieldName]; ok {
			return "", recordError(RecordErrorInvalidRequest, "sort field is duplicated", map[string]any{"entity": l.Entity, "field": fieldName}, nil)
		}
		seen[fieldName] = struct{}{}
		field, err := l.listField(fieldName, "sort")
		if err != nil {
			return "", err
		}
		direction := "ASC"
		if sortTerm.Desc {
			direction = "DESC"
		}
		if field.Name == "id" {
			hasID = true
		}
		terms = append(terms, quoteIdent(field.Column)+" "+direction)
	}
	if !hasID {
		terms = append(terms, quoteIdent("id")+" ASC")
	}
	return strings.Join(terms, ", "), nil
}

func (l recordLayout) listField(name string, operation string) (recordField, error) {
	if field, ok := systemRecordFieldByName(name); ok && field.Listable {
		return recordField{
			Name:      field.Name,
			Type:      field.Type,
			Column:    field.Column,
			Storage:   true,
			Listable:  true,
			Nameable:  field.Nameable,
			ValueKind: field.ValueKind,
		}, nil
	}
	field, ok := l.FieldByName[name]
	if !ok {
		return recordField{}, recordError(RecordErrorInvalidRequest, "unknown field", map[string]any{"entity": l.Entity, "field": name, "operation": operation}, nil)
	}
	if !field.Storage {
		return recordField{}, recordError(RecordErrorInvalidRequest, "field is not supported by record runtime", map[string]any{"entity": l.Entity, "field": name, "operation": operation}, nil)
	}
	if field.WriteOnly {
		return recordField{}, recordError(RecordErrorInvalidRequest, "write-only field cannot be used in record lists", map[string]any{"entity": l.Entity, "field": name, "operation": operation}, nil)
	}
	if !field.Listable {
		return recordField{}, recordError(RecordErrorInvalidRequest, "field cannot be used in record lists", map[string]any{"entity": l.Entity, "field": name, "operation": operation}, nil)
	}
	return field, nil
}

func (s RecordStore) recordListValue(ctx context.Context, layout recordLayout, field recordField, value string) (any, error) {
	if field.Type == "link" {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, recordError(RecordErrorValidation, "field filter is invalid", map[string]any{"field": field.Name}, err)
		}
		return s.linkValueCodec().storageValue(ctx, layout, field, raw)
	}
	switch field.ValueKind {
	case fieldtype.ValueString:
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, recordError(RecordErrorValidation, "field filter is invalid", map[string]any{"field": field.Name}, err)
		}
		return recordDBValue(field, raw)
	case fieldtype.ValueDate:
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return nil, recordError(RecordErrorValidation, "field must be a date", map[string]any{"field": field.Name}, err)
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, recordError(RecordErrorValidation, "field filter is invalid", map[string]any{"field": field.Name}, err)
		}
		return recordDBValue(field, raw)
	case fieldtype.ValueDatetime:
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			return nil, recordError(RecordErrorValidation, "field must be a datetime", map[string]any{"field": field.Name}, err)
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, recordError(RecordErrorValidation, "field filter is invalid", map[string]any{"field": field.Name}, err)
		}
		return recordDBValue(field, raw)
	case fieldtype.ValueTime:
		if _, err := time.Parse("15:04:05", value); err != nil {
			return nil, recordError(RecordErrorValidation, "field must be a time", map[string]any{"field": field.Name}, err)
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, recordError(RecordErrorValidation, "field filter is invalid", map[string]any{"field": field.Name}, err)
		}
		return recordDBValue(field, raw)
	default:
		return recordDBValue(field, json.RawMessage(value))
	}
}

func (s RecordStore) createMutation(ctx context.Context, layout recordLayout, input RecordInput) (recordMutation, error) {
	input = normalizeRecordInput(input)
	if err := layout.validateCreateInput(input); err != nil {
		return recordMutation{}, err
	}
	mutation, err := s.writeMutation(ctx, layout, input)
	if err != nil {
		return recordMutation{}, err
	}
	if mutation.hasColumn("name") {
		return mutation, nil
	}
	name, err := s.generateRecordName(ctx, layout, input)
	if err != nil {
		return recordMutation{}, err
	}
	nameField := recordField{Name: "name", Type: "text", Column: "name", Storage: true, Nameable: true, ValueKind: fieldtype.ValueString}
	mutation.Columns = append(mutation.Columns, "name")
	mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, nameField))
	mutation.Values = append(mutation.Values, name)
	return mutation, nil
}

func (s RecordStore) updateMutation(ctx context.Context, layout recordLayout, input RecordInput) (recordMutation, error) {
	input = normalizeRecordInput(input)
	if len(input) == 0 {
		return recordMutation{}, recordError(RecordErrorValidation, "at least one field is required", map[string]any{"entity": layout.Entity}, nil)
	}
	if err := layout.validateUpdateFields(input); err != nil {
		return recordMutation{}, err
	}
	return s.writeMutation(ctx, layout, input)
}

func (l recordLayout) validateCreateFields(input RecordInput) error {
	return l.validateInputFields(input, true, false)
}

func (l recordLayout) validateCreateInput(input RecordInput) error {
	if err := l.validateCreateFields(input); err != nil {
		return err
	}
	for _, field := range l.Fields {
		if !field.Required || len(field.Default) > 0 {
			continue
		}
		if field.SystemName {
			// The generated system name column enforces this field.
			continue
		}
		if _, ok := input[field.Name]; !ok {
			return recordError(RecordErrorValidation, "required field is missing", map[string]any{"entity": l.Entity, "field": field.Name}, nil)
		}
	}
	return nil
}

func (l recordLayout) validateUpdateFields(input RecordInput) error {
	return l.validateInputFields(input, false, false)
}

func (l recordLayout) validateMatchFields(input RecordInput) error {
	return l.validateInputFields(input, false, true)
}

func (l recordLayout) validateCollectionInput(fieldName string, raw json.RawMessage, allowIDs bool) error {
	_, err := l.collectionRowInputs(fieldName, raw, allowIDs)
	return err
}

func (l recordLayout) collectionRowInputs(fieldName string, raw json.RawMessage, allowIDs bool) ([]recordCollectionRowInput, error) {
	collection, ok := l.Collections[fieldName]
	if !ok {
		return nil, recordError(RecordErrorInternal, "collection metadata is missing for field", map[string]any{"entity": l.Entity, "field": fieldName}, nil)
	}
	if rawIsNull(raw) {
		return nil, recordError(RecordErrorValidation, "collection field must be an array", map[string]any{"entity": l.Entity, "field": fieldName}, nil)
	}
	var payload []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, recordError(RecordErrorValidation, "collection field must be an array", map[string]any{"entity": l.Entity, "field": fieldName}, err)
	}
	if len(payload) > recordCollectionMaxRows {
		return nil, recordError(RecordErrorValidation, "collection field has too many rows", map[string]any{"entity": l.Entity, "field": fieldName, "max": recordCollectionMaxRows}, nil)
	}
	if collection.Field.Required && len(payload) == 0 {
		return nil, recordError(RecordErrorValidation, "required collection field must include at least one row", map[string]any{"entity": l.Entity, "field": fieldName}, nil)
	}
	rows := make([]recordCollectionRowInput, 0, len(payload))
	seenIDs := map[int64]struct{}{}
	for index, row := range payload {
		if row == nil {
			return nil, recordError(RecordErrorValidation, "collection row must be an object", map[string]any{"entity": l.Entity, "field": fieldName, "row": index + 1}, nil)
		}
		input := make(RecordInput, len(row))
		var rowID int64
		for name, value := range row {
			if name == systemFieldID {
				if rawIsNull(value) {
					continue
				}
				if !allowIDs {
					return nil, recordError(RecordErrorValidation, "collection row id cannot be written on create", map[string]any{"entity": l.Entity, "field": fieldName, "row": index + 1}, nil)
				}
				id, err := collectionRowIDFromRaw(fieldName, value)
				if err != nil {
					return nil, err
				}
				rowID = id
				continue
			}
			if isCollectionRowSystemInput(name) {
				continue
			}
			cloned := make([]byte, len(value))
			copy(cloned, value)
			input[name] = cloned
		}
		if rowID > 0 {
			if _, ok := seenIDs[rowID]; ok {
				return nil, recordError(RecordErrorValidation, "collection row id is duplicated", map[string]any{"entity": l.Entity, "field": fieldName, "row-id": rowID}, nil)
			}
			seenIDs[rowID] = struct{}{}
		}
		if rowID > 0 {
			if err := collection.Layout.validateUpdateFields(input); err != nil {
				return nil, err
			}
		} else if err := collection.Layout.validateCreateInput(input); err != nil {
			return nil, err
		}
		rows = append(rows, recordCollectionRowInput{ID: rowID, Ordinal: int64(index + 1), Input: input})
	}
	return rows, nil
}

func (l recordLayout) validateInputFields(input RecordInput, create bool, match bool) error {
	for name, raw := range input {
		if isSystemRecordField(name) && !(match && name == "name") && !(create && l.allowsNameCreateInput()) {
			return recordError(RecordErrorValidation, "system fields cannot be written", map[string]any{"entity": l.Entity, "field": name}, nil)
		}
		if match && name == "name" {
			if rawIsNull(raw) {
				return recordError(RecordErrorValidation, "match field cannot be null", map[string]any{"entity": l.Entity, "field": name}, nil)
			}
			continue
		}
		field, ok := l.FieldByName[name]
		if !ok {
			return recordError(RecordErrorValidation, "unknown field", map[string]any{"entity": l.Entity, "field": name}, nil)
		}
		if !field.Storage {
			if field.Type == "collection" && !match {
				if err := l.validateCollectionInput(name, raw, !create); err != nil {
					return err
				}
				continue
			}
			return recordError(RecordErrorValidation, "field is not supported by record runtime", map[string]any{"entity": l.Entity, "field": name}, nil)
		}
		if rawIsNull(raw) && field.Required {
			return recordError(RecordErrorValidation, "required field cannot be null", map[string]any{"entity": l.Entity, "field": name}, nil)
		}
	}
	return nil
}

func (l recordLayout) allowsNameCreateInput() bool {
	return l.Naming.Strategy == schema.NamingStrategyManual
}

func (s RecordStore) writeMutation(ctx context.Context, layout recordLayout, input RecordInput) (recordMutation, error) {
	mutation := recordMutation{}
	codec := s.linkValueCodec()
	names := sortedRecordInputNames(input)
	for _, name := range names {
		field, ok := layout.FieldByName[name]
		if !ok {
			return recordMutation{}, recordError(RecordErrorValidation, "unknown field", map[string]any{"entity": layout.Entity, "field": name}, nil)
		}
		if field.Type == "collection" {
			continue
		}
		if !field.Storage {
			return recordMutation{}, recordError(RecordErrorValidation, "field is not supported by record runtime", map[string]any{"entity": layout.Entity, "field": name}, nil)
		}
		value, err := codec.storageValue(ctx, layout, field, input[name])
		if err != nil {
			return recordMutation{}, err
		}
		mutation.Columns = append(mutation.Columns, field.Column)
		mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, field))
		mutation.Values = append(mutation.Values, value)
	}
	return mutation, nil
}

func (s RecordStore) matchMutation(ctx context.Context, layout recordLayout, input RecordInput) (recordMutation, error) {
	mutation := recordMutation{}
	codec := s.linkValueCodec()
	names := sortedRecordInputNames(input)
	for _, name := range names {
		if name == "name" {
			field := recordField{Name: "name", Type: "text", Column: "name", Storage: true, Nameable: true, ValueKind: fieldtype.ValueString}
			value, err := recordDBValue(field, input[name])
			if err != nil {
				return recordMutation{}, err
			}
			mutation.Columns = append(mutation.Columns, "name")
			mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, field))
			mutation.Values = append(mutation.Values, value)
			continue
		}
		field := layout.FieldByName[name]
		value, err := codec.storageValue(ctx, layout, field, input[name])
		if err != nil {
			return recordMutation{}, err
		}
		mutation.Columns = append(mutation.Columns, field.Column)
		mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, field))
		mutation.Values = append(mutation.Values, value)
	}
	return mutation, nil
}

func (m *recordMutation) hasColumn(column string) bool {
	for _, existing := range m.Columns {
		if existing == column {
			return true
		}
	}
	return false
}

func normalizeRecordListParams(params RecordListParams) (RecordListParams, error) {
	normalized, err := recordquery.Normalize(params)
	if err != nil {
		return RecordListParams{}, recordQueryError(err)
	}
	return normalized, nil
}

func recordQueryError(err error) error {
	var queryErr recordquery.Error
	if errors.As(err, &queryErr) {
		return recordError(RecordErrorInvalidRequest, queryErr.Message, queryErr.Details, queryErr.Err)
	}
	return recordError(RecordErrorInvalidRequest, err.Error(), nil, err)
}

func normalizeRecordInput(input RecordInput) RecordInput {
	if input == nil {
		return RecordInput{}
	}
	return input
}

func recordDBValue(field recordField, raw json.RawMessage) (any, error) {
	if rawIsNull(raw) {
		return nil, nil
	}
	if len(field.Options.Values) > 0 {
		value, err := jsonStringValue(field, raw)
		if err != nil {
			return nil, err
		}
		if !stringInSlice(value, field.Options.Values) {
			return nil, recordError(RecordErrorValidation, "select value is not allowed", map[string]any{"field": field.Name, "value": value}, nil)
		}
		return value, nil
	}
	switch field.ValueKind {
	case fieldtype.ValueString:
		return jsonStringValue(field, raw)
	case fieldtype.ValuePassword:
		return passwordHashValue(field, raw)
	case fieldtype.ValueInteger:
		return jsonIntValue(field, raw)
	case fieldtype.ValueNumber:
		return jsonNumberStringValue(field, raw)
	case fieldtype.ValueBoolean:
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, recordError(RecordErrorValidation, "field must be a boolean", map[string]any{"field": field.Name}, err)
		}
		return value, nil
	case fieldtype.ValueDate, fieldtype.ValueDatetime, fieldtype.ValueTime:
		return jsonStringValue(field, raw)
	case fieldtype.ValueJSON:
		if !json.Valid(raw) {
			return nil, recordError(RecordErrorValidation, "field must be valid JSON", map[string]any{"field": field.Name}, nil)
		}
		return string(raw), nil
	default:
		return nil, recordError(RecordErrorValidation, "field type is not supported by record runtime", map[string]any{"field": field.Name, "type": field.Type}, nil)
	}
}

func jsonStringValue(field recordField, raw json.RawMessage) (string, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", recordError(RecordErrorValidation, "field must be a string", map[string]any{"field": field.Name}, err)
	}
	return value, nil
}

func passwordHashValue(field recordField, raw json.RawMessage) (string, error) {
	value, err := jsonStringValue(field, raw)
	if err != nil {
		return "", err
	}
	hash, err := auth.HashPassword(value)
	if err != nil {
		message := "password is invalid"
		if errors.Is(err, auth.ErrPasswordEmpty) {
			message = "password must not be empty"
		} else if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			message = "password is too long"
		}
		return "", recordError(RecordErrorValidation, message, map[string]any{"field": field.Name}, err)
	}
	return hash, nil
}

func jsonIntValue(field recordField, raw json.RawMessage) (int64, error) {
	number, err := jsonNumberValue(field, raw)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil {
		return 0, recordError(RecordErrorValidation, "field must be an integer", map[string]any{"field": field.Name}, err)
	}
	return value, nil
}

func jsonNumberStringValue(field recordField, raw json.RawMessage) (string, error) {
	number, err := jsonNumberValue(field, raw)
	if err != nil {
		return "", err
	}
	return number.String(), nil
}

func jsonNumberValue(field recordField, raw json.RawMessage) (json.Number, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", recordError(RecordErrorValidation, "field must be a number", map[string]any{"field": field.Name}, err)
	}
	number, ok := value.(json.Number)
	if !ok {
		return "", recordError(RecordErrorValidation, "field must be a number", map[string]any{"field": field.Name}, nil)
	}
	if _, err := strconv.ParseFloat(number.String(), 64); err != nil {
		return "", recordError(RecordErrorValidation, "field must be a number", map[string]any{"field": field.Name}, err)
	}
	return number, nil
}

func recordPlaceholder(index int, field recordField) string {
	placeholder := "$" + strconv.Itoa(index)
	if definition, ok := fieldtype.DefaultDefinition(field.Type); ok && definition.Behavior.PlaceholderCast != "" {
		return placeholder + "::" + definition.Behavior.PlaceholderCast
	}
	return placeholder
}

func collectionSystemMutationField(name string, column string) recordField {
	return recordField{Name: name, Type: "bigint", Column: column, Storage: true, ValueKind: fieldtype.ValueInteger}
}

func collectionRowIDFromRaw(fieldName string, raw json.RawMessage) (int64, error) {
	id, err := jsonIntValue(recordField{Name: systemFieldID, Type: "bigint", ValueKind: fieldtype.ValueInteger}, raw)
	if err != nil {
		return 0, err
	}
	if id <= 0 {
		return 0, recordError(RecordErrorValidation, "collection row id must be a positive integer", map[string]any{"field": fieldName, "row-id": id}, nil)
	}
	return id, nil
}

func recordIDFromDBValue(value any, entity string) (int64, error) {
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
	return 0, recordError(RecordErrorInternal, "record id column has invalid type", map[string]any{"entity": entity, "type": fmt.Sprintf("%T", value)}, nil)
}

func isCollectionRowSystemInput(name string) bool {
	switch name {
	case systemFieldName, systemFieldCreatedAt, systemFieldUpdatedAt,
		"created_at", "updated_at",
		"parent-entity-id", systemColumnParentEntityID,
		"parent-record-id", systemColumnParentRecordID,
		"parent-field-id", systemColumnParentFieldID,
		systemColumnOrdinal:
		return true
	default:
		return false
	}
}

func normalizeRecordValue(fieldType string, value any) any {
	if value == nil {
		return nil
	}
	valueKind := ""
	if definition, ok := fieldtype.DefaultDefinition(fieldType); ok {
		valueKind = definition.Behavior.ValueKind
	}
	switch typed := value.(type) {
	case []byte:
		if valueKind == fieldtype.ValueJSON {
			var decoded any
			if err := json.Unmarshal(typed, &decoded); err == nil {
				return decoded
			}
		}
		return string(typed)
	case string:
		if valueKind == fieldtype.ValueJSON {
			var decoded any
			if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
				return decoded
			}
		}
		return typed
	case time.Time:
		switch fieldType {
		case "date":
			return typed.Format("2006-01-02")
		case "time":
			return typed.Format("15:04:05")
		case "datetime":
			return normalizeDatetimeValue(typed)
		default:
			if valueKind == fieldtype.ValueDatetime {
				return normalizeDatetimeValue(typed)
			}
			return typed
		}
	case float64:
		if math.Trunc(typed) == typed {
			return typed
		}
		return typed
	default:
		return typed
	}
}

func normalizeDatetimeValue(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func sortedRecordInputNames(input RecordInput) []string {
	names := make([]string, 0, len(input))
	for name := range input {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

func sortedRecordFilters(filters []RecordFilter) []RecordFilter {
	sorted := append([]RecordFilter(nil), filters...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && compareRecordFilters(sorted[j], sorted[j-1]) < 0; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}

func compareRecordFilters(left RecordFilter, right RecordFilter) int {
	if left.Field < right.Field {
		return -1
	}
	if left.Field > right.Field {
		return 1
	}
	if left.Operator < right.Operator {
		return -1
	}
	if left.Operator > right.Operator {
		return 1
	}
	if left.Value < right.Value {
		return -1
	}
	if left.Value > right.Value {
		return 1
	}
	return 0
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func rawIsNull(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func invalidRecordIDError(entity string) error {
	return recordError(RecordErrorInvalidRequest, "record id must be a positive integer", map[string]any{"entity": entity}, nil)
}

func classifyRecordDBError(err error, entity string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "42P01", "42703":
			return recordError(RecordErrorSchemaNotReady, "schema is not ready; run dygo db migrate", map[string]any{"entity": entity}, err)
		case "23505", "23503", "23514", "23502":
			details := map[string]any{"entity": entity}
			if pgErr.ConstraintName != "" {
				details["constraint"] = pgErr.ConstraintName
			}
			return recordError(RecordErrorConstraintViolation, "record violates a database constraint", details, err)
		}
	}
	return recordError(RecordErrorInternal, "record query failed", map[string]any{"entity": entity}, err)
}

func recordError(code string, message string, details map[string]any, err error) RecordError {
	return RecordError{Code: code, Message: message, Details: details, Err: err}
}

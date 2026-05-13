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

	"github.com/dygo-dev/dygo/internal/auth"
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
	defaultRecordLimit = 50
	maxRecordLimit     = 100
	randomNameRetries  = 5
)

// RecordQueryer is the database behavior needed by the Record store.
type RecordQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// RecordStore reads and mutates Records through persisted Entity metadata.
type RecordStore struct {
	queryer  RecordQueryer
	metadata MetadataReader
	hooks    *RecordHookRegistry
}

// Record is one metadata-backed saved Entity instance.
type Record map[string]any

// RecordInput is the decoded request data for creating or updating a Record.
type RecordInput map[string]json.RawMessage

// RecordListParams controls Record list pagination.
type RecordListParams struct {
	Limit  int
	Offset int
}

// RecordListResult is a page of Records.
type RecordListResult struct {
	Records []Record
	Limit   int
	Offset  int
	Count   int
}

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

// NewRecordStore returns a Record store backed by queryer.
func NewRecordStore(queryer RecordQueryer) RecordStore {
	return NewRecordStoreWithHooks(queryer, DefaultRecordHookRegistry())
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

	sql := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s ASC LIMIT $1 OFFSET $2", layout.selectList(), quoteIdent(layout.Table), quoteIdent("id"))
	rows, err := s.queryer.Query(ctx, sql, params.Limit, params.Offset)
	if err != nil {
		return RecordListResult{}, classifyRecordDBError(err, entity)
	}
	defer rows.Close()

	records := []Record{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return RecordListResult{}, recordError(RecordErrorInternal, "read record row failed", map[string]any{"entity": entity}, err)
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
	return RecordListResult{Records: records, Limit: params.Limit, Offset: params.Offset, Count: len(records)}, nil
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
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1", layout.selectList(), quoteIdent(layout.Table), quoteIdent("id"))
	return s.queryOneRecord(ctx, layout, sql, id)
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
	if err := layout.validateMatchFields(match); err != nil {
		return nil, err
	}
	mutation, err := layout.matchMutation(match)
	if err != nil {
		return nil, err
	}
	clauses := make([]string, 0, len(mutation.Columns))
	for i, column := range mutation.Columns {
		clauses = append(clauses, fmt.Sprintf("%s = %s", quoteIdent(column), mutation.Placeholders[i]))
	}
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY %s ASC LIMIT 2", layout.selectList(), quoteIdent(layout.Table), strings.Join(clauses, " AND "), quoteIdent("id"))
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
	return record, nil
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

func (s RecordStore) createRecord(ctx context.Context, entity string, input RecordInput) (Record, error) {
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return nil, err
	}
	input = cloneRecordInput(input)
	hookCtx := newRecordHookContext(RecordBeforeValidate, layout)
	hookCtx.Operation = activityOperationCreate
	hookCtx.Input = input
	if err := s.runRecordHooks(ctx, hookCtx); err != nil {
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

	var record Record
	for attempt := 0; attempt <= randomNameRetries; attempt++ {
		mutation, err := s.createMutation(ctx, layout, input)
		if err != nil {
			return nil, err
		}
		var sql string
		if len(mutation.Columns) == 0 {
			sql = fmt.Sprintf("INSERT INTO %s DEFAULT VALUES RETURNING %s", quoteIdent(layout.Table), layout.selectList())
		} else {
			sql = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s", quoteIdent(layout.Table), quoteIdentList(mutation.Columns), strings.Join(mutation.Placeholders, ", "), layout.selectList())
		}
		record, err = s.queryReturningRecord(ctx, layout, sql, mutation.Values, false)
		if err == nil {
			break
		}
		if layout.Naming.Strategy != "random" || !isRecordNameCollision(err, layout) || attempt == randomNameRetries {
			return nil, err
		}
	}
	recordID, err := activityRecordID(record)
	if err != nil {
		return nil, err
	}
	afterCtx := newRecordHookContext(RecordAfterCreate, layout)
	afterCtx.Operation = activityOperationCreate
	afterCtx.RecordID = recordID
	afterCtx.Input = input
	afterCtx.NewRecord = record
	afterCtx.Snapshot = record
	if err := s.runRecordHooks(ctx, afterCtx); err != nil {
		return nil, err
	}
	return record, nil
}

func (s RecordStore) generateRecordName(ctx context.Context, layout recordLayout, input RecordInput) (string, error) {
	switch layout.Naming.Strategy {
	case "random":
		name, err := randomRecordName(layout.Naming.Length)
		if err != nil {
			return "", recordError(RecordErrorInternal, "generate random record name failed", map[string]any{"entity": layout.Entity}, err)
		}
		return name, nil
	case "field":
		field, ok := layout.FieldByName[layout.Naming.Field]
		if !ok {
			return "", recordError(RecordErrorInternal, "naming field metadata is missing", map[string]any{"entity": layout.Entity, "field": layout.Naming.Field}, nil)
		}
		raw, ok := input[layout.Naming.Field]
		if !ok {
			return "", recordError(RecordErrorValidation, "naming field is required", map[string]any{"entity": layout.Entity, "field": layout.Naming.Field}, nil)
		}
		return recordNameValue(field, raw)
	case "series":
		return s.seriesRecordName(ctx, layout, layout.Naming.Pattern, time.Now().UTC())
	default:
		return "", recordError(RecordErrorInternal, "naming strategy metadata is invalid", map[string]any{"entity": layout.Entity, "strategy": layout.Naming.Strategy}, nil)
	}
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

func (s RecordStore) updateRecord(ctx context.Context, entity string, id int64, input RecordInput) (Record, error) {
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return nil, err
	}
	input = cloneRecordInput(input)
	oldRecord, err := s.getRecordWithLayout(ctx, layout, id)
	if err != nil {
		return nil, err
	}
	hookCtx := newRecordHookContext(RecordBeforeValidate, layout)
	hookCtx.Operation = activityOperationUpdate
	hookCtx.RecordID = id
	hookCtx.Input = input
	hookCtx.OldRecord = oldRecord
	if err := s.runRecordHooks(ctx, hookCtx); err != nil {
		return nil, err
	}
	if err := layout.validateUpdateFields(input); err != nil {
		return nil, err
	}
	hookCtx.Event = RecordValidate
	if err := s.runRecordHooks(ctx, hookCtx); err != nil {
		return nil, err
	}
	mutation, err := layout.updateMutation(input)
	if err != nil {
		return nil, err
	}
	beforeCtx := newRecordHookContext(RecordBeforeUpdate, layout)
	beforeCtx.Operation = activityOperationUpdate
	beforeCtx.RecordID = id
	beforeCtx.Input = input
	beforeCtx.OldRecord = oldRecord
	if err := s.runRecordHooks(ctx, beforeCtx); err != nil {
		return nil, err
	}

	setClauses := make([]string, 0, len(mutation.Columns)+1)
	for i, column := range mutation.Columns {
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", quoteIdent(column), mutation.Placeholders[i]))
	}
	setClauses = append(setClauses, fmt.Sprintf("%s = now()", quoteIdent("updated_at")))
	args := append([]any(nil), mutation.Values...)
	args = append(args, id)
	sql := fmt.Sprintf("UPDATE %s SET %s WHERE %s = $%d RETURNING %s", quoteIdent(layout.Table), strings.Join(setClauses, ", "), quoteIdent("id"), len(args), layout.selectList())
	record, err := s.queryReturningRecord(ctx, layout, sql, args, true)
	if err != nil {
		return nil, err
	}
	changes := layout.activityChanges(input, oldRecord, record)
	if len(changes) == 0 {
		return record, nil
	}
	recordID, err := activityRecordID(record)
	if err != nil {
		return nil, err
	}
	afterCtx := newRecordHookContext(RecordAfterUpdate, layout)
	afterCtx.Operation = activityOperationUpdate
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

func (s RecordStore) deleteRecord(ctx context.Context, entity string, id int64) error {
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return err
	}
	oldRecord, err := s.getRecordWithLayout(ctx, layout, id)
	if err != nil {
		return err
	}
	beforeCtx := newRecordHookContext(RecordBeforeDelete, layout)
	beforeCtx.Operation = activityOperationDelete
	beforeCtx.RecordID = id
	beforeCtx.OldRecord = oldRecord
	if err := s.runRecordHooks(ctx, beforeCtx); err != nil {
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
	afterCtx.Operation = activityOperationDelete
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

func (s RecordStore) getRecordWithLayout(ctx context.Context, layout recordLayout, id int64) (Record, error) {
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1", layout.selectList(), quoteIdent(layout.Table), quoteIdent("id"))
	record, err := s.queryOneRecord(ctx, layout, sql, id)
	if err != nil {
		var recordErr RecordError
		if errors.As(err, &recordErr) && recordErr.Code == RecordErrorNotFound {
			recordErr.Details = map[string]any{"entity": layout.Entity, "id": id}
			return nil, recordErr
		}
		return nil, err
	}
	return record, nil
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
	EntityID    int64
	AppName     string
	Entity      string
	RouteSlug   string
	Label       string
	Table       string
	Naming      recordNaming
	Fields      []recordField
	FieldByName map[string]recordField
}

type recordField struct {
	Name        string
	Type        string
	Required    bool
	Default     json.RawMessage
	Options     recordFieldOptions
	Column      string
	Storage     bool
	WriteOnly   bool
	SystemName  bool
	SelectOrder int
}

type recordFieldOptions struct {
	App    string   `json:"app,omitempty"`
	Values []string `json:"values,omitempty"`
	Entity string   `json:"entity,omitempty"`
}

type recordMutation struct {
	Columns      []string
	Placeholders []string
	Values       []any
}

func newRecordLayout(meta MetadataEntityMeta) (recordLayout, error) {
	naming, err := parseRecordNaming(meta.Naming)
	if err != nil {
		return recordLayout{}, recordError(RecordErrorInternal, "entity naming metadata is invalid", map[string]any{"entity": meta.Name}, err)
	}
	layout := recordLayout{
		EntityID:    meta.ID,
		AppName:     meta.App.Name,
		Entity:      meta.Name,
		RouteSlug:   meta.RouteSlug,
		Label:       meta.Label,
		Table:       entityTableName(meta.App.Name, meta.Name),
		Naming:      naming,
		FieldByName: map[string]recordField{},
	}
	for _, metadataField := range meta.Fields {
		field := recordField{
			Name:      metadataField.Name,
			Type:      metadataField.Type,
			Required:  metadataField.Required,
			Default:   metadataField.Default,
			Storage:   metadataField.Type != "child-table",
			WriteOnly: metadataField.Type == "password",
			SystemName: metadataField.Name == "name" &&
				naming.Strategy == "field" &&
				naming.Field == "name",
		}
		if len(metadataField.Options) > 0 {
			if err := json.Unmarshal(metadataField.Options, &field.Options); err != nil {
				return recordLayout{}, recordError(RecordErrorInternal, "field options metadata is invalid", map[string]any{"entity": meta.Name, "field": metadataField.Name}, err)
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
	return layout, nil
}

func (l recordLayout) selectList() string {
	columns := []string{quoteIdent("id"), quoteIdent("name"), quoteIdent("created_at"), quoteIdent("updated_at")}
	for _, field := range l.Fields {
		if field.Storage && !field.WriteOnly && !field.SystemName {
			columns = append(columns, quoteIdent(field.Column))
		}
	}
	return strings.Join(columns, ", ")
}

func (l recordLayout) recordFromValues(values []any) (Record, error) {
	expected := 4
	for _, field := range l.Fields {
		if field.Storage && !field.WriteOnly && !field.SystemName {
			expected++
		}
	}
	if len(values) != expected {
		return nil, recordError(RecordErrorInternal, "record column count did not match metadata", map[string]any{"entity": l.Entity, "expected": expected, "actual": len(values)}, nil)
	}
	record := Record{
		"id":         normalizeRecordValue("", values[0]),
		"name":       normalizeRecordValue("text", values[1]),
		"created-at": normalizeRecordValue("datetime", values[2]),
		"updated-at": normalizeRecordValue("datetime", values[3]),
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

func (s RecordStore) createMutation(ctx context.Context, layout recordLayout, input RecordInput) (recordMutation, error) {
	input = normalizeRecordInput(input)
	if err := layout.validateCreateInput(input); err != nil {
		return recordMutation{}, err
	}
	mutation, err := layout.writeMutation(input)
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
	nameField := recordField{Name: "name", Type: "text", Column: "name", Storage: true}
	mutation.Columns = append(mutation.Columns, "name")
	mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, nameField))
	mutation.Values = append(mutation.Values, name)
	return mutation, nil
}

func (l recordLayout) updateMutation(input RecordInput) (recordMutation, error) {
	input = normalizeRecordInput(input)
	if len(input) == 0 {
		return recordMutation{}, recordError(RecordErrorValidation, "at least one field is required", map[string]any{"entity": l.Entity}, nil)
	}
	if err := l.validateUpdateFields(input); err != nil {
		return recordMutation{}, err
	}
	return l.writeMutation(input)
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
			return recordError(RecordErrorValidation, "field is not supported by record runtime", map[string]any{"entity": l.Entity, "field": name}, nil)
		}
		if rawIsNull(raw) && field.Required {
			return recordError(RecordErrorValidation, "required field cannot be null", map[string]any{"entity": l.Entity, "field": name}, nil)
		}
	}
	return nil
}

func (l recordLayout) allowsNameCreateInput() bool {
	return l.Naming.Strategy == "field" && l.Naming.Field == "name"
}

func (l recordLayout) writeMutation(input RecordInput) (recordMutation, error) {
	mutation := recordMutation{}
	names := sortedRecordInputNames(input)
	for _, name := range names {
		field := l.FieldByName[name]
		value, err := recordDBValue(field, input[name])
		if err != nil {
			return recordMutation{}, err
		}
		mutation.Columns = append(mutation.Columns, field.Column)
		mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, field))
		mutation.Values = append(mutation.Values, value)
	}
	return mutation, nil
}

func (l recordLayout) matchMutation(input RecordInput) (recordMutation, error) {
	mutation := recordMutation{}
	names := sortedRecordInputNames(input)
	for _, name := range names {
		if name == "name" {
			field := recordField{Name: "name", Type: "text", Column: "name", Storage: true}
			value, err := recordDBValue(field, input[name])
			if err != nil {
				return recordMutation{}, err
			}
			mutation.Columns = append(mutation.Columns, "name")
			mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, field))
			mutation.Values = append(mutation.Values, value)
			continue
		}
		field := l.FieldByName[name]
		value, err := recordDBValue(field, input[name])
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
	if params.Limit == 0 {
		params.Limit = defaultRecordLimit
	}
	if params.Limit < 0 || params.Limit > maxRecordLimit {
		return RecordListParams{}, recordError(RecordErrorInvalidRequest, "limit must be between 1 and 100", map[string]any{"limit": params.Limit}, nil)
	}
	if params.Offset < 0 {
		return RecordListParams{}, recordError(RecordErrorInvalidRequest, "offset must be greater than or equal to 0", map[string]any{"offset": params.Offset}, nil)
	}
	return params, nil
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
	switch field.Type {
	case "text", "email", "phone", "long-text", "attachment":
		return jsonStringValue(field, raw)
	case "password":
		return passwordHashValue(field, raw)
	case "select":
		value, err := jsonStringValue(field, raw)
		if err != nil {
			return nil, err
		}
		if len(field.Options.Values) > 0 && !stringInSlice(value, field.Options.Values) {
			return nil, recordError(RecordErrorValidation, "select value is not allowed", map[string]any{"field": field.Name, "value": value}, nil)
		}
		return value, nil
	case "int", "bigint", "link":
		return jsonIntValue(field, raw)
	case "decimal", "currency":
		return jsonNumberStringValue(field, raw)
	case "boolean":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, recordError(RecordErrorValidation, "field must be a boolean", map[string]any{"field": field.Name}, err)
		}
		return value, nil
	case "date", "datetime", "time":
		return jsonStringValue(field, raw)
	case "json":
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
	switch field.Type {
	case "int":
		return placeholder + "::integer"
	case "bigint", "link":
		return placeholder + "::bigint"
	case "decimal", "currency":
		return placeholder + "::numeric"
	case "boolean":
		return placeholder + "::boolean"
	case "date":
		return placeholder + "::date"
	case "datetime":
		return placeholder + "::timestamptz"
	case "time":
		return placeholder + "::time"
	case "json":
		return placeholder + "::jsonb"
	default:
		return placeholder
	}
}

func recordColumnForField(name string, fieldType string) string {
	column := storageName(name)
	switch fieldType {
	case "link":
		column += "_id"
	case "password":
		column += "_hash"
	}
	return column
}

func normalizeRecordValue(fieldType string, value any) any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []byte:
		if fieldType == "json" {
			var decoded any
			if err := json.Unmarshal(typed, &decoded); err == nil {
				return decoded
			}
		}
		return string(typed)
	case string:
		if fieldType == "json" {
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
		default:
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

func sortedRecordInputNames(input RecordInput) []string {
	names := make([]string, 0, len(input))
	for name := range input {
		names = append(names, name)
	}
	sortStrings(names)
	return names
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

func isSystemRecordField(name string) bool {
	switch name {
	case "id", "name", "created-at", "updated-at":
		return true
	default:
		return false
	}
}

func invalidRecordIDError(entity string) error {
	return recordError(RecordErrorInvalidRequest, "record id must be a positive integer", map[string]any{"entity": entity}, nil)
}

func classifyRecordDBError(err error, entity string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "42P01", "42703":
			return recordError(RecordErrorSchemaNotReady, "schema is not ready; run dygo migrate", map[string]any{"entity": entity}, err)
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

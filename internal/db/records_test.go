package db

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRecordStoreListRecords(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(1), now, now, "a@example.com", "A User", true},
		{int64(2), now, now, "b@example.com", "B User", false},
	}))

	result, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{})
	if err != nil {
		t.Fatalf("ListRecords() error = %v, want nil", err)
	}
	if result.Limit != 50 || result.Offset != 0 || result.Count != 2 {
		t.Fatalf("ListRecords() result = %+v, want default pagination and two records", result)
	}
	if result.Records[0]["email"] != "a@example.com" || result.Records[0]["full-name"] != "A User" {
		t.Fatalf("ListRecords() first record = %+v, want metadata field names", result.Records[0])
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	if !strings.Contains(lastQuery, `FROM "user"`) || !strings.Contains(lastQuery, `ORDER BY "id" ASC LIMIT $1 OFFSET $2`) {
		t.Fatalf("list query = %q, want deterministic paginated query", lastQuery)
	}
	if got := queryer.args[len(queryer.args)-1]; len(got) != 2 || got[0] != 50 || got[1] != 0 {
		t.Fatalf("list args = %#v, want default limit/offset", got)
	}
}

func TestRecordStoreGetRecord(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), now, now, "a@example.com", "A User", true},
	}))

	record, err := NewRecordStore(queryer).GetRecord(context.Background(), "user", 7)
	if err != nil {
		t.Fatalf("GetRecord() error = %v, want nil", err)
	}
	if record["id"] != int64(7) || record["email"] != "a@example.com" {
		t.Fatalf("GetRecord() = %+v, want record by id", record)
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	if !strings.Contains(lastQuery, `WHERE "id" = $1`) {
		t.Fatalf("get query = %q, want id predicate", lastQuery)
	}
}

func TestRecordStoreCreateRecord(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), now, now, "a@example.com", "A User", true},
	}))

	record, err := NewRecordStore(queryer).CreateRecord(context.Background(), "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"A User"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	if record["id"] != int64(7) || record["enabled"] != true {
		t.Fatalf("CreateRecord() = %+v, want returned record", record)
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{`INSERT INTO "user"`, `"email", "full_name"`, `RETURNING "id", "created_at", "updated_at"`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("create query = %q, want %q", lastQuery, want)
		}
	}
}

func TestRecordStoreUpdateRecord(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), now, now, "a@example.com", "Renamed User", true},
	}))

	record, err := NewRecordStore(queryer).UpdateRecord(context.Background(), "user", 7, recordInput(map[string]string{
		"full-name": `"Renamed User"`,
	}))
	if err != nil {
		t.Fatalf("UpdateRecord() error = %v, want nil", err)
	}
	if record["full-name"] != "Renamed User" {
		t.Fatalf("UpdateRecord() = %+v, want patched record", record)
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{`UPDATE "user" SET "full_name" = $1`, `"updated_at" = now()`, `WHERE "id" = $2`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("update query = %q, want %q", lastQuery, want)
		}
	}
}

func TestRecordStoreDeleteRecord(t *testing.T) {
	queryer := newUserRecordQueryer()
	queryer.execTags = []pgconn.CommandTag{pgconn.NewCommandTag("DELETE 1")}

	err := NewRecordStore(queryer).DeleteRecord(context.Background(), "user", 7)
	if err != nil {
		t.Fatalf("DeleteRecord() error = %v, want nil", err)
	}
	if !strings.Contains(queryer.execSQL[0], `DELETE FROM "user" WHERE "id" = $1`) {
		t.Fatalf("delete SQL = %q, want hard delete by id", queryer.execSQL[0])
	}
}

func TestRecordStoreValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		entity    string
		queryer   *fakeRecordQueryer
		input     RecordInput
		wantCode  string
		wantField string
	}{
		{
			name:      "missing required field",
			queryer:   newUserRecordQueryer(),
			input:     recordInput(map[string]string{"email": `"a@example.com"`}),
			wantCode:  RecordErrorValidation,
			wantField: "full-name",
		},
		{
			name:      "unknown field",
			queryer:   newUserRecordQueryer(),
			input:     recordInput(map[string]string{"email": `"a@example.com"`, "full-name": `"A User"`, "legacy": `"x"`}),
			wantCode:  RecordErrorValidation,
			wantField: "legacy",
		},
		{
			name:      "system field",
			queryer:   newUserRecordQueryer(),
			input:     recordInput(map[string]string{"email": `"a@example.com"`, "full-name": `"A User"`, "id": `1`}),
			wantCode:  RecordErrorValidation,
			wantField: "id",
		},
		{
			name:      "invalid select value",
			entity:    "lead",
			queryer:   newLeadRecordQueryer(),
			input:     recordInput(map[string]string{"status": `"Archived"`}),
			wantCode:  RecordErrorValidation,
			wantField: "status",
		},
		{
			name:      "child table write",
			entity:    "lead",
			queryer:   newLeadRecordQueryer(),
			input:     recordInput(map[string]string{"status": `"New"`, "contacts": `[]`}),
			wantCode:  RecordErrorValidation,
			wantField: "contacts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := tt.entity
			if entity == "" {
				entity = "user"
			}
			_, err := NewRecordStore(tt.queryer).CreateRecord(context.Background(), entity, tt.input)
			if tt.queryer == nil {
				t.Fatal("test queryer is nil")
			}
			assertRecordError(t, err, tt.wantCode, tt.wantField)
		})
	}
}

func TestRecordStoreMapsDatabaseErrors(t *testing.T) {
	tests := []struct {
		name     string
		pgCode   string
		wantCode string
	}{
		{name: "schema not ready", pgCode: "42P01", wantCode: RecordErrorSchemaNotReady},
		{name: "constraint violation", pgCode: "23505", wantCode: RecordErrorConstraintViolation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryer := newUserRecordQueryer()
			queryer.queryErrs = append(queryer.queryErrs, nil, nil, nil, &pgconn.PgError{Code: tt.pgCode, ConstraintName: "user_email_key"})

			_, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{})
			assertRecordError(t, err, tt.wantCode, "")
		})
	}
}

func TestRecordStoreMapsMetadataSchemaErrors(t *testing.T) {
	queryer := newUserRecordQueryer()
	queryer.row = fakeRow{err: &pgconn.PgError{Code: "42P01"}}

	_, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{})
	assertRecordError(t, err, RecordErrorSchemaNotReady, "")
}

func TestRecordStoreInvalidPaginationAndIDs(t *testing.T) {
	_, err := NewRecordStore(newUserRecordQueryer()).ListRecords(context.Background(), "user", RecordListParams{Limit: 101})
	assertRecordError(t, err, RecordErrorInvalidRequest, "")

	_, err = NewRecordStore(newUserRecordQueryer()).GetRecord(context.Background(), "user", 0)
	assertRecordError(t, err, RecordErrorInvalidRequest, "")
}

func newUserRecordQueryer() *fakeRecordQueryer {
	return &fakeRecordQueryer{
		row: newFakeRow(int64(10), "user", "User", "User identity", "core", "Core"),
		rows: []pgx.Rows{
			newFakeRows([][]any{
				{"email", "Email", "email", true, true, false, nil, 1, nil},
				{"full-name", "Full Name", "text", true, false, false, nil, 2, nil},
				{"enabled", "Enabled", "boolean", false, false, true, []byte("true"), 3, nil},
			}),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}
}

func newLeadRecordQueryer() *fakeRecordQueryer {
	return &fakeRecordQueryer{
		row: newFakeRow(int64(20), "lead", "Lead", "Sales lead", "crm", "CRM"),
		rows: []pgx.Rows{
			newFakeRows([][]any{
				{"status", "Status", "select", true, false, false, nil, 1, []byte(`{"values":["New","Qualified"]}`)},
				{"contacts", "Contacts", "child-table", false, false, false, nil, 2, []byte(`{"entity":"lead-contact"}`)},
			}),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}
}

type fakeRecordQueryer struct {
	row       pgx.Row
	rows      []pgx.Rows
	queryErrs []error
	execTags  []pgconn.CommandTag
	execErrs  []error

	queries []string
	args    [][]any
	rowSQL  []string
	rowArgs [][]any
	execSQL []string
	execArg [][]any
}

func (q *fakeRecordQueryer) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.queries = append(q.queries, sql)
	q.args = append(q.args, args)
	if len(q.queryErrs) > 0 {
		err := q.queryErrs[0]
		q.queryErrs = q.queryErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(q.rows) == 0 {
		return newFakeRows(nil), nil
	}
	rows := q.rows[0]
	q.rows = q.rows[1:]
	return rows, nil
}

func (q *fakeRecordQueryer) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.rowSQL = append(q.rowSQL, sql)
	q.rowArgs = append(q.rowArgs, args)
	if q.row == nil {
		return fakeRow{err: pgx.ErrNoRows}
	}
	return q.row
}

func (q *fakeRecordQueryer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execSQL = append(q.execSQL, sql)
	q.execArg = append(q.execArg, args)
	if len(q.execErrs) > 0 {
		err := q.execErrs[0]
		q.execErrs = q.execErrs[1:]
		if err != nil {
			return pgconn.CommandTag{}, err
		}
	}
	if len(q.execTags) == 0 {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}
	tag := q.execTags[0]
	q.execTags = q.execTags[1:]
	return tag, nil
}

func recordInput(values map[string]string) RecordInput {
	input := RecordInput{}
	for key, value := range values {
		input[key] = json.RawMessage(value)
	}
	return input
}

func assertRecordError(t *testing.T, err error, code string, field string) {
	t.Helper()

	var recordErr RecordError
	if !errors.As(err, &recordErr) {
		t.Fatalf("error = %v, want RecordError", err)
	}
	if recordErr.Code != code {
		t.Fatalf("RecordError code = %q, want %q", recordErr.Code, code)
	}
	if field != "" && recordErr.Details["field"] != field {
		t.Fatalf("RecordError details = %#v, want field %q", recordErr.Details, field)
	}
}

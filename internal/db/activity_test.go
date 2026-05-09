package db

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestActivityReaderListRecordActivity(t *testing.T) {
	newer := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	older := newer.Add(-time.Hour)
	queryer := &fakeMetadataQueryer{
		row: newFakeRow(int64(10)),
		rows: []pgx.Rows{newFakeRows([][]any{
			{
				int64(2), newer, "user", int64(42), "record", "update", "success", "Updated User", "changed email",
				[]byte(`[{"field":"email","old":"a@example.com","new":"b@example.com"}]`),
				[]byte(`{"id":42,"email":"b@example.com"}`),
				[]byte(`{"source":"api"}`),
				int64(7), "admin@example.com", "Admin User",
			},
			{
				int64(1), older, "user", int64(42), "record", "create", "success", "Created User", "",
				nil,
				[]byte(`{"id":42,"email":"a@example.com"}`),
				[]byte(`{"source":"fixtures"}`),
				int64(0), "", "",
			},
		})},
	}

	result, err := NewActivityReader(queryer).ListRecordActivity(context.Background(), "user", 42, RecordListParams{})
	if err != nil {
		t.Fatalf("ListRecordActivity() error = %v, want nil", err)
	}
	if result.Count != 2 || result.Limit != 50 || result.Offset != 0 {
		t.Fatalf("ListRecordActivity() meta = count %d limit %d offset %d, want 2/50/0", result.Count, result.Limit, result.Offset)
	}
	if len(result.Activities) != 2 || result.Activities[0].ID != 2 || result.Activities[1].ID != 1 {
		t.Fatalf("ListRecordActivity() activities = %+v, want newest first", result.Activities)
	}
	if !strings.Contains(queryer.queries[0], "ORDER BY a.created_at DESC, a.id DESC") {
		t.Fatalf("ListRecordActivity() query = %q, want newest-first ordering", queryer.queries[0])
	}
	if !reflect.DeepEqual(queryer.args[0], []any{int64(10), int64(42), 50, 0}) {
		t.Fatalf("ListRecordActivity() args = %#v, want entity id, record id, limit, offset", queryer.args[0])
	}

	first := result.Activities[0]
	if first.Entity != "user" || first.RecordID != 42 || first.Operation != "update" || first.Status != "success" {
		t.Fatalf("first activity = %+v, want user/update/success", first)
	}
	if first.Actor == nil || first.Actor.ID != 7 || first.Actor.Email != "admin@example.com" || first.Actor.FullName != "Admin User" {
		t.Fatalf("first actor = %+v, want admin actor", first.Actor)
	}
	changes, ok := first.Changes.([]any)
	if !ok || len(changes) != 1 {
		t.Fatalf("changes = %#v, want one decoded JSON change", first.Changes)
	}
	change, ok := changes[0].(map[string]any)
	if !ok || change["field"] != "email" || change["new"] != "b@example.com" {
		t.Fatalf("change = %#v, want decoded email change", changes[0])
	}
	snapshot, ok := first.Snapshot.(map[string]any)
	if !ok || snapshot["email"] != "b@example.com" {
		t.Fatalf("snapshot = %#v, want decoded visible record", first.Snapshot)
	}
	details, ok := first.Details.(map[string]any)
	if !ok || details["source"] != "api" {
		t.Fatalf("details = %#v, want decoded source details", first.Details)
	}
	if result.Activities[1].Actor != nil {
		t.Fatalf("second actor = %+v, want nil", result.Activities[1].Actor)
	}
}

func TestActivityReaderListRecordActivityEmpty(t *testing.T) {
	queryer := &fakeMetadataQueryer{
		row:  newFakeRow(int64(10)),
		rows: []pgx.Rows{newFakeRows(nil)},
	}

	result, err := NewActivityReader(queryer).ListRecordActivity(context.Background(), "lead", 99, RecordListParams{Limit: 25, Offset: 5})
	if err != nil {
		t.Fatalf("ListRecordActivity() error = %v, want nil", err)
	}
	if result.Count != 0 || len(result.Activities) != 0 || result.Limit != 25 || result.Offset != 5 {
		t.Fatalf("ListRecordActivity() = %+v, want empty page with requested pagination", result)
	}
	if strings.Contains(queryer.queries[0], `FROM "lead"`) || strings.Contains(queryer.queries[0], `JOIN "lead"`) {
		t.Fatalf("ListRecordActivity() query = %q, should not require live target record table", queryer.queries[0])
	}
}

func TestActivityReaderValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		recordID int64
		params   RecordListParams
		wantCode string
	}{
		{name: "invalid id", recordID: 0, wantCode: RecordErrorInvalidRequest},
		{name: "invalid limit", recordID: 1, params: RecordListParams{Limit: 101}, wantCode: RecordErrorInvalidRequest},
		{name: "invalid offset", recordID: 1, params: RecordListParams{Offset: -1}, wantCode: RecordErrorInvalidRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryer := &fakeMetadataQueryer{row: newFakeRow(int64(10))}
			_, err := NewActivityReader(queryer).ListRecordActivity(context.Background(), "user", tt.recordID, tt.params)
			assertRecordErrorCode(t, err, tt.wantCode)
		})
	}
}

func TestActivityReaderMapsEntityAndSchemaErrors(t *testing.T) {
	tests := []struct {
		name     string
		queryer  *fakeMetadataQueryer
		wantCode string
	}{
		{
			name:     "missing entity",
			queryer:  &fakeMetadataQueryer{row: fakeRow{err: pgx.ErrNoRows}},
			wantCode: RecordErrorNotFound,
		},
		{
			name:     "missing entity table",
			queryer:  &fakeMetadataQueryer{row: fakeRow{err: &pgconn.PgError{Code: "42P01"}}},
			wantCode: RecordErrorSchemaNotReady,
		},
		{
			name: "missing activity table",
			queryer: &fakeMetadataQueryer{
				row:      newFakeRow(int64(10)),
				queryErr: &pgconn.PgError{Code: "42P01"},
			},
			wantCode: RecordErrorSchemaNotReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewActivityReader(tt.queryer).ListRecordActivity(context.Background(), "user", 1, RecordListParams{})
			assertRecordErrorCode(t, err, tt.wantCode)
		})
	}
}

func assertRecordErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	var recordErr RecordError
	if !errors.As(err, &recordErr) {
		t.Fatalf("error = %v, want RecordError code %s", err, code)
	}
	if recordErr.Code != code {
		t.Fatalf("RecordError.Code = %q, want %q (err = %v)", recordErr.Code, code, err)
	}
}

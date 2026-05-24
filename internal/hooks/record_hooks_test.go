package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dygo-dev/dygo/internal/db"
	"github.com/dygo-dev/dygo/internal/hookevents"
	"github.com/dygo-dev/dygo/pkg/sdk"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestNewRecordHookRegistryAppliesRegistrarsInOrder(t *testing.T) {
	t.Parallel()

	var order []string
	registry, err := NewRecordHookRegistry([]sdk.RecordHookRegistrar{
		func(registry sdk.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", sdk.RecordBeforeCreate, "first", func(_ context.Context, dygo sdk.RecordHook) error {
				order = append(order, "first:"+dygo.Entity)
				dygo.Input["status"] = json.RawMessage(`"qualified"`)
				return nil
			})
		},
		func(registry sdk.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", sdk.RecordBeforeCreate, "second", func(_ context.Context, dygo sdk.RecordHook) error {
				order = append(order, "second:"+string(dygo.Event))
				return nil
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRecordHookRegistry() error = %v, want nil", err)
	}

	input := db.RecordInput{"status": json.RawMessage(`"new"`)}
	err = registry.Run(context.Background(), db.RecordHookContext{
		Event:       db.RecordBeforeCreate,
		Operation:   sdk.RecordOperationCreate,
		EntityID:    7,
		AppName:     "sales",
		Entity:      "lead",
		RouteSlug:   "lead",
		EntityLabel: "Lead",
		Input:       input,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if want := []string{"first:lead", "second:before-create"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("hook order = %#v, want %#v", order, want)
	}
	if string(input["status"]) != `"qualified"` {
		t.Fatalf("input status = %s, want hook mutation", input["status"])
	}
}

func TestRecordHookEventMapsAllSDKEvents(t *testing.T) {
	t.Parallel()
	sdkEvents := sdk.SupportedRecordHookEvents()
	specs := hookevents.Specs()
	if len(sdkEvents) != len(specs) {
		t.Fatalf("SDK events = %#v, specs = %#v; want matching counts", sdkEvents, specs)
	}
	for index, spec := range specs {
		spec := spec
		t.Run(spec.Name, func(t *testing.T) {
			t.Parallel()

			if got := sdkEvents[index]; got != sdk.RecordHookEvent(spec.Name) {
				t.Fatalf("SDK event %d = %q, want %q", index, got, spec.Name)
			}
			got, err := recordHookEvent(sdk.RecordHookEvent(spec.Name))
			if err != nil {
				t.Fatalf("recordHookEvent(%q) error = %v, want nil", spec.Name, err)
			}
			if got != db.RecordHookEvent(spec.Name) {
				t.Fatalf("recordHookEvent(%q) = %q, want %q", spec.Name, got, spec.Name)
			}
		})
	}
}

func TestNewRecordHookRegistryRejectsUnsupportedSDKEvent(t *testing.T) {
	t.Parallel()

	_, err := NewRecordHookRegistry([]sdk.RecordHookRegistrar{
		func(registry sdk.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", sdk.RecordHookEvent("after-save"), "legacy-save", func(context.Context, sdk.RecordHook) error {
				return nil
			})
		},
	})
	if err == nil {
		t.Fatal("NewRecordHookRegistry() error = nil, want unsupported SDK event error")
	}
	for _, want := range []string{
		"register record hook registrar 1",
		`record hook event "after-save" is not supported`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("NewRecordHookRegistry() error = %q, want %q", err.Error(), want)
		}
	}
}

func TestNewRecordHookRegistryReturnsRegistrarErrors(t *testing.T) {
	t.Parallel()

	_, err := NewRecordHookRegistry([]sdk.RecordHookRegistrar{
		func(sdk.RecordHookRegistry) error {
			return errors.New("boom")
		},
	})
	if err == nil {
		t.Fatal("NewRecordHookRegistry() error = nil, want registrar error")
	}
	if !strings.Contains(err.Error(), "register record hook registrar 1: boom") {
		t.Fatalf("NewRecordHookRegistry() error = %q, want registrar context", err.Error())
	}
}

func TestNewRecordHookRegistryExposesTransactionalRecordData(t *testing.T) {
	t.Parallel()

	queryer := &failingRecordQueryer{err: errors.New("metadata lookup failed")}
	var dataErr error
	registry, err := NewRecordHookRegistry([]sdk.RecordHookRegistrar{
		func(registry sdk.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", sdk.RecordBeforeCreate, "read-related-record", func(ctx context.Context, dygo sdk.RecordHook) error {
				if dygo.Records == nil {
					return errors.New("record data service is nil")
				}
				_, dataErr = dygo.Records.Get(ctx, "sales", "lead", 42)
				return nil
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRecordHookRegistry() error = %v, want nil", err)
	}

	err = registry.Run(context.Background(), db.RecordHookContext{
		Event:     db.RecordBeforeCreate,
		Operation: sdk.RecordOperationCreate,
		AppName:   "sales",
		Entity:    "lead",
		Queryer:   queryer,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if dataErr == nil || !strings.Contains(dataErr.Error(), "metadata lookup failed") {
		t.Fatalf("Records.Get() error = %v, want queryer-backed metadata error", dataErr)
	}
	if queryer.rowCalls != 1 {
		t.Fatalf("queryer row calls = %d, want Records.Get to use hook queryer", queryer.rowCalls)
	}
	if !strings.Contains(queryer.rowSQL[0], `WHERE a.name = $1 AND e.key = $2`) {
		t.Fatalf("Records.Get() metadata query = %q, want app/entity lookup", queryer.rowSQL[0])
	}
	if !reflect.DeepEqual(queryer.rowArgs[0], []any{"sales", "lead"}) {
		t.Fatalf("Records.Get() metadata args = %#v, want sales/lead", queryer.rowArgs[0])
	}
}

func TestRecordDataListPassesFiltersAndSortThroughHookQueryer(t *testing.T) {
	t.Parallel()

	queryer := newUserRecordDataMutationQueryer(userRecordRow("a@example.com", "A User", true))
	var result sdk.RecordListResult
	var listErr error
	registry, err := NewRecordHookRegistry([]sdk.RecordHookRegistrar{
		func(registry sdk.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", sdk.RecordBeforeCreate, "list-users", func(ctx context.Context, dygo sdk.RecordHook) error {
				result, listErr = dygo.Records.List(ctx, "core", "user", sdk.RecordListParams{
					Filters: []sdk.RecordFilter{{Field: "enabled", Value: "true"}},
					Sort:    []sdk.RecordSort{{Field: "full-name", Desc: true}},
				})
				return nil
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRecordHookRegistry() error = %v, want nil", err)
	}

	err = registry.Run(context.Background(), db.RecordHookContext{
		Event:     db.RecordBeforeCreate,
		Operation: sdk.RecordOperationCreate,
		AppName:   "sales",
		Entity:    "lead",
		Queryer:   queryer,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if listErr != nil {
		t.Fatalf("Records.List() error = %v, want nil", listErr)
	}
	if result.Count != 1 || len(result.Records) != 1 {
		t.Fatalf("Records.List() result = %+v, want one record", result)
	}
	if !strings.Contains(queryer.rowSQL[0], `WHERE a.name = $1 AND e.key = $2`) {
		t.Fatalf("Records.List() metadata query = %q, want app/entity lookup", queryer.rowSQL[0])
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{`FROM "user"`, `WHERE "enabled" = $1::boolean`, `ORDER BY "full_name" DESC, "id" ASC`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("Records.List() query = %q, want %q", lastQuery, want)
		}
	}
	if got := queryer.args[len(queryer.args)-1]; !reflect.DeepEqual(got, []any{true, 20, 0}) {
		t.Fatalf("Records.List() args = %#v, want filter and pagination args", got)
	}
}

func TestRecordDataMutationsDoNotReenterAppHooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		queryer func() *recordDataMutationQueryer
		mutate  func(context.Context, sdk.RecordHook) error
	}{
		{
			name: "create",
			queryer: func() *recordDataMutationQueryer {
				return newUserRecordDataMutationQueryer(userRecordRow("a@example.com", "A User", true))
			},
			mutate: func(ctx context.Context, dygo sdk.RecordHook) error {
				_, err := dygo.Records.Create(ctx, "core", "user", sdk.RecordInput{
					"email":     json.RawMessage(`"a@example.com"`),
					"full-name": json.RawMessage(`"A User"`),
				})
				return err
			},
		},
		{
			name: "update",
			queryer: func() *recordDataMutationQueryer {
				return newUserRecordDataMutationQueryer(
					userRecordRow("a@example.com", "A User", true),
					userRecordRow("a@example.com", "Renamed User", true),
				)
			},
			mutate: func(ctx context.Context, dygo sdk.RecordHook) error {
				_, err := dygo.Records.Update(ctx, "core", "user", 7, sdk.RecordInput{
					"full-name": json.RawMessage(`"Renamed User"`),
				})
				return err
			},
		},
		{
			name: "delete",
			queryer: func() *recordDataMutationQueryer {
				queryer := newUserRecordDataMutationQueryer(userRecordRow("a@example.com", "A User", true))
				queryer.execTags = []pgconn.CommandTag{pgconn.NewCommandTag("DELETE 1")}
				return queryer
			},
			mutate: func(ctx context.Context, dygo sdk.RecordHook) error {
				return dygo.Records.Delete(ctx, "core", "user", 7)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			queryer := tt.queryer()
			var outerCalls int
			var reentryCalls int
			registry, err := NewRecordHookRegistry([]sdk.RecordHookRegistrar{
				func(registry sdk.RecordHookRegistry) error {
					if err := registry.RegisterEntity("sales", "lead", sdk.RecordBeforeCreate, "outer-"+tt.name, func(ctx context.Context, dygo sdk.RecordHook) error {
						outerCalls++
						return tt.mutate(ctx, dygo)
					}); err != nil {
						return err
					}
					for _, event := range allRecordHookEvents() {
						event := event
						if err := registry.RegisterEntity("core", "user", event, "reentry-"+string(event), func(context.Context, sdk.RecordHook) error {
							reentryCalls++
							return nil
						}); err != nil {
							return err
						}
					}
					return nil
				},
			})
			if err != nil {
				t.Fatalf("NewRecordHookRegistry() error = %v, want nil", err)
			}

			err = registry.Run(context.Background(), db.RecordHookContext{
				Event:     db.RecordBeforeCreate,
				Operation: sdk.RecordOperationCreate,
				AppName:   "sales",
				Entity:    "lead",
				Queryer:   queryer,
			})
			if err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if outerCalls != 1 {
				t.Fatalf("outer hook calls = %d, want 1", outerCalls)
			}
			if reentryCalls != 0 {
				t.Fatalf("inner app hook calls = %d, want dygo.Records mutation without app hook re-entry", reentryCalls)
			}
			if !queryer.executedSQLContaining(`INSERT INTO "activity"`) {
				t.Fatalf("exec SQL = %#v, want framework Activity hook to stay active", queryer.execSQL)
			}
		})
	}
}

func allRecordHookEvents() []sdk.RecordHookEvent {
	return sdk.SupportedRecordHookEvents()
}

func TestNewRecordHookRegistryRejectsNilRegistrarAndHook(t *testing.T) {
	t.Parallel()

	_, err := NewRecordHookRegistry([]sdk.RecordHookRegistrar{nil})
	if err == nil {
		t.Fatal("NewRecordHookRegistry(nil registrar) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "record hook registrar 1 is required") {
		t.Fatalf("NewRecordHookRegistry(nil registrar) error = %q, want nil registrar context", err.Error())
	}

	_, err = NewRecordHookRegistry([]sdk.RecordHookRegistrar{
		func(registry sdk.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", sdk.RecordBeforeCreate, "missing", nil)
		},
	})
	if err == nil {
		t.Fatal("NewRecordHookRegistry(nil hook) error = nil, want error")
	}
	if !strings.Contains(err.Error(), `record hook "missing" function is required`) {
		t.Fatalf("NewRecordHookRegistry(nil hook) error = %q, want nil hook context", err.Error())
	}
}

type failingRecordQueryer struct {
	err      error
	rowCalls int
	rowSQL   []string
	rowArgs  [][]any
}

func (q *failingRecordQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, q.err
}

func (q *failingRecordQueryer) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.rowCalls++
	q.rowSQL = append(q.rowSQL, sql)
	q.rowArgs = append(q.rowArgs, args)
	return failingRow{err: q.err}
}

func (q *failingRecordQueryer) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, q.err
}

type failingRow struct {
	err error
}

func (r failingRow) Scan(...any) error {
	return r.err
}

type recordDataMutationQueryer struct {
	row      pgx.Row
	rows     []pgx.Rows
	execTags []pgconn.CommandTag

	queries []string
	args    [][]any
	rowSQL  []string
	rowArgs [][]any
	execSQL []string
	execArg [][]any
}

func newUserRecordDataMutationQueryer(recordRows ...[]any) *recordDataMutationQueryer {
	queryer := &recordDataMutationQueryer{
		row: newRecordDataMutationRow(int64(10), "core.user", "user", "user", "User", "User identity", "user", false, false, false, []byte(`{"strategy":"format","format":"{email}"}`), "core", "Core"),
		rows: []pgx.Rows{
			newRecordDataMutationRows([][]any{
				{"email", "Email", "email", true, true, false, nil, nil, 1, nil},
				{"full-name", "Full Name", "text", true, false, false, nil, nil, 2, nil},
				{"password", "Password", "password", false, false, false, nil, nil, 3, nil},
				{"enabled", "Enabled", "boolean", false, false, true, []byte("true"), nil, 4, nil},
			}),
			newRecordDataMutationRows(nil),
			newRecordDataMutationRows(nil),
		},
	}
	for _, row := range recordRows {
		queryer.rows = append(queryer.rows, newRecordDataMutationRows([][]any{row}))
	}
	return queryer
}

func userRecordRow(email string, fullName string, enabled bool) []any {
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	return []any{int64(7), email, now, now, email, fullName, enabled}
}

func (q *recordDataMutationQueryer) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.queries = append(q.queries, sql)
	q.args = append(q.args, args)
	if rows, ok := hookActivityMetadataRows(sql, args...); ok {
		return rows, nil
	}
	if len(q.rows) == 0 {
		return newRecordDataMutationRows(nil), nil
	}
	rows := q.rows[0]
	q.rows = q.rows[1:]
	return rows, nil
}

func (q *recordDataMutationQueryer) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.rowSQL = append(q.rowSQL, sql)
	q.rowArgs = append(q.rowArgs, args)
	if isHookActivityMetadataQuery(sql, args...) {
		return newRecordDataMutationRow(int64(1), "core.activity", "activity", "activity", "Activity", "Timeline entry", "activity", false, false, false, []byte(`{"strategy":"random","length":16}`), "core", "Core")
	}
	if strings.Contains(sql, `SELECT "id" FROM "entity"`) && len(args) == 1 && args[0] == "core.user" {
		return newRecordDataMutationRow(int64(10))
	}
	if strings.Contains(sql, `SELECT "name" FROM "entity"`) && len(args) == 1 && args[0] == int64(10) {
		return newRecordDataMutationRow("core.user")
	}
	if q.row == nil {
		return recordDataMutationRow{err: pgx.ErrNoRows}
	}
	return q.row
}

func (q *recordDataMutationQueryer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execSQL = append(q.execSQL, sql)
	q.execArg = append(q.execArg, args)
	if len(q.execTags) == 0 {
		return pgconn.CommandTag{}, nil
	}
	tag := q.execTags[0]
	q.execTags = q.execTags[1:]
	return tag, nil
}

func (q *recordDataMutationQueryer) executedSQLContaining(fragment string) bool {
	for _, sql := range q.execSQL {
		if strings.Contains(sql, fragment) {
			return true
		}
	}
	return false
}

func isHookActivityMetadataQuery(sql string, args ...any) bool {
	return strings.Contains(sql, `WHERE a.name = $1 AND e.key = $2`) &&
		len(args) == 2 &&
		args[0] == "core" &&
		args[1] == "activity"
}

func hookActivityMetadataRows(sql string, args ...any) (pgx.Rows, bool) {
	if len(args) != 1 || args[0] != int64(1) {
		return nil, false
	}
	switch {
	case strings.Contains(sql, `FROM "field"`):
		return newRecordDataMutationRows([][]any{
			{"kind", "Kind", "select", true, false, true, nil, nil, 1, []byte(`{"values":["record"]}`)},
			{"operation", "Operation", "select", true, false, true, nil, nil, 2, []byte(`{"values":["create","update","delete"]}`)},
			{"status", "Status", "select", true, false, true, nil, nil, 3, []byte(`{"values":["success"]}`)},
			{"entity", "Entity", "link", false, false, true, nil, nil, 4, []byte(`{"entity":"entity"}`)},
			{"record-id", "Record ID", "bigint", false, false, true, nil, nil, 5, nil},
			{"title", "Title", "text", true, false, false, nil, nil, 6, nil},
			{"changes", "Changes", "json", false, false, false, nil, nil, 7, nil},
			{"snapshot", "Snapshot", "json", false, false, false, nil, nil, 8, nil},
		}), true
	case strings.Contains(sql, `FROM "index"`), strings.Contains(sql, `FROM "constraint"`):
		return newRecordDataMutationRows(nil), true
	default:
		return nil, false
	}
}

type recordDataMutationRow struct {
	values []any
	err    error
}

func newRecordDataMutationRow(values ...any) recordDataMutationRow {
	return recordDataMutationRow{values: values}
}

func (r recordDataMutationRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignRecordDataMutationScanValues(r.values, dest)
}

type recordDataMutationRows struct {
	values [][]any
	index  int
	err    error
	closed bool
}

func newRecordDataMutationRows(values [][]any) *recordDataMutationRows {
	return &recordDataMutationRows{values: values, index: -1}
}

func (r *recordDataMutationRows) Close() {
	r.closed = true
}

func (r *recordDataMutationRows) Err() error {
	return r.err
}

func (r *recordDataMutationRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *recordDataMutationRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *recordDataMutationRows) Next() bool {
	if r.index+1 >= len(r.values) {
		r.closed = true
		return false
	}
	r.index++
	return true
}

func (r *recordDataMutationRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.values) {
		return errors.New("scan without current row")
	}
	return assignRecordDataMutationScanValues(r.values[r.index], dest)
}

func (r *recordDataMutationRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.values) {
		return nil, errors.New("values without current row")
	}
	return r.values[r.index], nil
}

func (r *recordDataMutationRows) RawValues() [][]byte {
	return nil
}

func (r *recordDataMutationRows) Conn() *pgx.Conn {
	return nil
}

func assignRecordDataMutationScanValues(values []any, dest []any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("scan value count %d does not match destination count %d", len(values), len(dest))
	}
	for i, value := range values {
		switch target := dest[i].(type) {
		case *int:
			v, ok := value.(int)
			if !ok {
				return fmt.Errorf("scan value %d has type %T, want int", i, value)
			}
			*target = v
		case *int64:
			v, ok := value.(int64)
			if !ok {
				return fmt.Errorf("scan value %d has type %T, want int64", i, value)
			}
			*target = v
		case *string:
			if value == nil {
				*target = ""
				continue
			}
			v, ok := value.(string)
			if !ok {
				return fmt.Errorf("scan value %d has type %T, want string", i, value)
			}
			*target = v
		case *bool:
			v, ok := value.(bool)
			if !ok {
				return fmt.Errorf("scan value %d has type %T, want bool", i, value)
			}
			*target = v
		case *[]byte:
			if value == nil {
				*target = nil
				continue
			}
			v, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("scan value %d has type %T, want []byte", i, value)
			}
			*target = append([]byte(nil), v...)
		default:
			return fmt.Errorf("unsupported scan destination %T", dest[i])
		}
	}
	return nil
}

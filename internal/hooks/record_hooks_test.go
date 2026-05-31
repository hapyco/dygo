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

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/hookevents"
	"github.com/hapyco/dygo/pkg/dygo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestNewRecordHookRegistryAppliesRegistrarsInOrder(t *testing.T) {
	t.Parallel()

	var order []string
	registry, err := NewRecordHookRegistry([]dygo.RecordHookRegistrar{
		func(registry dygo.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", dygo.RecordBeforeCreate, "first", func(_ context.Context, hook dygo.RecordHook) error {
				order = append(order, "first:"+hook.Entity)
				hook.Input["status"] = json.RawMessage(`"qualified"`)
				return nil
			})
		},
		func(registry dygo.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", dygo.RecordBeforeCreate, "second", func(_ context.Context, hook dygo.RecordHook) error {
				order = append(order, "second:"+string(hook.Event))
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
		Operation:   dygo.RecordOperationCreate,
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
	sdkEvents := dygo.SupportedRecordHookEvents()
	specs := hookevents.Specs()
	if len(sdkEvents) != len(specs) {
		t.Fatalf("SDK events = %#v, specs = %#v; want matching counts", sdkEvents, specs)
	}
	for index, spec := range specs {
		spec := spec
		t.Run(spec.Name, func(t *testing.T) {
			t.Parallel()

			if got := sdkEvents[index]; got != dygo.RecordHookEvent(spec.Name) {
				t.Fatalf("SDK event %d = %q, want %q", index, got, spec.Name)
			}
			got, err := recordHookEvent(dygo.RecordHookEvent(spec.Name))
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

	_, err := NewRecordHookRegistry([]dygo.RecordHookRegistrar{
		func(registry dygo.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", dygo.RecordHookEvent("after-save"), "legacy-save", func(context.Context, dygo.RecordHook) error {
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

	_, err := NewRecordHookRegistry([]dygo.RecordHookRegistrar{
		func(dygo.RecordHookRegistry) error {
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
	registry, err := NewRecordHookRegistry([]dygo.RecordHookRegistrar{
		func(registry dygo.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", dygo.RecordBeforeCreate, "read-related-record", func(ctx context.Context, hook dygo.RecordHook) error {
				if hook.Records == nil {
					return errors.New("record data service is nil")
				}
				_, dataErr = hook.Records.Get(ctx, "sales", "lead", 42)
				return nil
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRecordHookRegistry() error = %v, want nil", err)
	}

	err = registry.Run(context.Background(), db.RecordHookContext{
		Event:     db.RecordBeforeCreate,
		Operation: dygo.RecordOperationCreate,
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

func TestNewRecordHookRegistryExposesJobDataForTransactionalHooks(t *testing.T) {
	t.Parallel()

	queryer := &beginnerRecordQueryer{failingRecordQueryer: &failingRecordQueryer{err: errors.New("unused")}}
	var hasJobs bool
	registry, err := NewRecordHookRegistry([]dygo.RecordHookRegistrar{
		func(registry dygo.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", dygo.RecordBeforeCreate, "enqueue-background-work", func(_ context.Context, hook dygo.RecordHook) error {
				hasJobs = hook.Jobs != nil
				return nil
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRecordHookRegistry() error = %v, want nil", err)
	}

	err = registry.Run(context.Background(), db.RecordHookContext{
		Event:   db.RecordBeforeCreate,
		AppName: "sales",
		Entity:  "lead",
		Queryer: queryer,
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !hasJobs {
		t.Fatal("RecordHook.Jobs = nil, want transactional job data")
	}
}

func TestRecordDataListPassesFiltersAndSortThroughHookQueryer(t *testing.T) {
	t.Parallel()

	queryer := newUserRecordDataMutationQueryer(userRecordRow("a@example.com", "A User", true))
	var result dygo.RecordListResult
	var listErr error
	registry, err := NewRecordHookRegistry([]dygo.RecordHookRegistrar{
		func(registry dygo.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", dygo.RecordBeforeCreate, "list-users", func(ctx context.Context, hook dygo.RecordHook) error {
				result, listErr = hook.Records.List(ctx, "core", "user", dygo.RecordListParams{
					Filters: []dygo.RecordFilter{{Field: "enabled", Operator: "eq", Value: "true"}},
					Sort:    []dygo.RecordSort{{Field: "full-name", Desc: true}},
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
		Operation: dygo.RecordOperationCreate,
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
		mutate  func(context.Context, dygo.RecordHook) error
	}{
		{
			name: "create",
			queryer: func() *recordDataMutationQueryer {
				return newUserRecordDataMutationQueryer(userRecordRow("a@example.com", "A User", true))
			},
			mutate: func(ctx context.Context, hook dygo.RecordHook) error {
				_, err := hook.Records.Create(ctx, "core", "user", dygo.RecordInput{
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
			mutate: func(ctx context.Context, hook dygo.RecordHook) error {
				_, err := hook.Records.Update(ctx, "core", "user", 7, dygo.RecordInput{
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
			mutate: func(ctx context.Context, hook dygo.RecordHook) error {
				return hook.Records.Delete(ctx, "core", "user", 7)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			queryer := tt.queryer()
			var outerCalls int
			var reentryCalls int
			registry, err := NewRecordHookRegistry([]dygo.RecordHookRegistrar{
				func(registry dygo.RecordHookRegistry) error {
					if err := registry.RegisterEntity("sales", "lead", dygo.RecordBeforeCreate, "outer-"+tt.name, func(ctx context.Context, hook dygo.RecordHook) error {
						outerCalls++
						return tt.mutate(ctx, hook)
					}); err != nil {
						return err
					}
					for _, event := range allRecordHookEvents() {
						event := event
						if err := registry.RegisterEntity("core", "user", event, "reentry-"+string(event), func(context.Context, dygo.RecordHook) error {
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
				Operation: dygo.RecordOperationCreate,
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
				t.Fatalf("inner app hook calls = %d, want hook.Records mutation without app hook re-entry", reentryCalls)
			}
			if !queryer.executedSQLContaining(`INSERT INTO "activity"`) {
				t.Fatalf("exec SQL = %#v, want framework Activity hook to stay active", queryer.execSQL)
			}
		})
	}
}

func TestRecordHookLogsUseLogQueryerOutsideRecordQueryer(t *testing.T) {
	t.Parallel()

	recordQueryer := newUserRecordDataMutationQueryer(userRecordRow("a@example.com", "A User", true))
	logQueryer := newLogRecordDataMutationQueryer()
	hookErr := errors.New("blocked by hook")
	var recordErr error
	registry, err := NewRecordHookRegistry([]dygo.RecordHookRegistrar{
		func(registry dygo.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", dygo.RecordBeforeCreate, "log-and-reject", func(ctx context.Context, hook dygo.RecordHook) error {
				_, recordErr = hook.Records.Create(ctx, "core", "user", dygo.RecordInput{
					"email":     json.RawMessage(`"a@example.com"`),
					"full-name": json.RawMessage(`"A User"`),
				})
				dygo.Error(ctx, "Lead hook failed", hookErr)
				return hookErr
			})
		},
	})
	if err != nil {
		t.Fatalf("NewRecordHookRegistry() error = %v, want nil", err)
	}

	err = registry.Run(context.Background(), db.RecordHookContext{
		Event:      db.RecordBeforeCreate,
		Operation:  dygo.RecordOperationCreate,
		AppName:    "sales",
		Entity:     "lead",
		Queryer:    recordQueryer,
		LogQueryer: logQueryer,
	})
	if err == nil || !strings.Contains(err.Error(), hookErr.Error()) {
		t.Fatalf("Run() error = %v, want hook error", err)
	}
	if recordErr != nil {
		t.Fatalf("hook.Records.Create() error = %v, want nil", recordErr)
	}
	if !recordQueryer.queriedSQLContaining(`INSERT INTO "user"`) {
		t.Fatalf("record queries = %#v, want hook record write through record queryer", recordQueryer.queries)
	}
	if recordQueryer.executedSQLContaining(`INSERT INTO "log"`) {
		t.Fatalf("record exec SQL = %#v, want no Log insert through record queryer", recordQueryer.execSQL)
	}
	if !logQueryer.executedSQLContaining(`INSERT INTO "log"`) {
		t.Fatalf("log exec SQL = %#v, want Log insert through log queryer", logQueryer.execSQL)
	}
	for _, want := range []any{"Error", "Hook", "Lead hook failed", hookErr.Error()} {
		if !logQueryer.executedArg(want) {
			t.Fatalf("log exec args = %#v, want %q", logQueryer.execArg, want)
		}
	}
}

func allRecordHookEvents() []dygo.RecordHookEvent {
	return dygo.SupportedRecordHookEvents()
}

func TestNewRecordHookRegistryRejectsNilRegistrarAndHook(t *testing.T) {
	t.Parallel()

	_, err := NewRecordHookRegistry([]dygo.RecordHookRegistrar{nil})
	if err == nil {
		t.Fatal("NewRecordHookRegistry(nil registrar) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "record hook registrar 1 is required") {
		t.Fatalf("NewRecordHookRegistry(nil registrar) error = %q, want nil registrar context", err.Error())
	}

	_, err = NewRecordHookRegistry([]dygo.RecordHookRegistrar{
		func(registry dygo.RecordHookRegistry) error {
			return registry.RegisterEntity("sales", "lead", dygo.RecordBeforeCreate, "missing", nil)
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

type beginnerRecordQueryer struct {
	*failingRecordQueryer
}

func (q *beginnerRecordQueryer) Begin(context.Context) (pgx.Tx, error) {
	return nil, nil
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
				{int64(101), "email", "Email", "email", true, true, false, nil, nil, nil, 1, nil},
				{int64(102), "full-name", "Full Name", "text", true, false, false, nil, nil, nil, 2, nil},
				{int64(103), "password", "Password", "password", false, false, false, nil, nil, nil, 3, nil},
				{int64(104), "enabled", "Enabled", "boolean", false, false, true, []byte("true"), nil, nil, 4, nil},
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

func newLogRecordDataMutationQueryer() *recordDataMutationQueryer {
	return &recordDataMutationQueryer{
		row: newRecordDataMutationRow(int64(2), "core.log", "log", "log", "Log", "Diagnostic log", "file-text", false, false, false, []byte(`{"strategy":"random","length":16}`), "core", "Core"),
		rows: []pgx.Rows{
			newRecordDataMutationRows(hookLogFieldRows()),
			newRecordDataMutationRows(nil),
			newRecordDataMutationRows(nil),
		},
	}
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
	if rows, ok := hookLogMetadataRows(sql, args...); ok {
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
	if isHookLogMetadataQuery(sql, args...) {
		return newRecordDataMutationRow(int64(2), "core.log", "log", "log", "Log", "Diagnostic log", "file-text", false, false, false, []byte(`{"strategy":"random","length":16}`), "core", "Core")
	}
	if strings.Contains(sql, `SELECT "id" FROM "app"`) && len(args) == 1 && args[0] == "sales" {
		return newRecordDataMutationRow(int64(20))
	}
	if strings.Contains(sql, `SELECT "id" FROM "entity"`) && len(args) == 1 && args[0] == "sales.lead" {
		return newRecordDataMutationRow(int64(30))
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

func (q *recordDataMutationQueryer) queriedSQLContaining(fragment string) bool {
	for _, sql := range q.queries {
		if strings.Contains(sql, fragment) {
			return true
		}
	}
	return false
}

func (q *recordDataMutationQueryer) executedArg(want any) bool {
	for _, args := range q.execArg {
		for _, got := range args {
			if got == want {
				return true
			}
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

func isHookLogMetadataQuery(sql string, args ...any) bool {
	return strings.Contains(sql, `WHERE a.name = $1 AND e.key = $2`) &&
		len(args) == 2 &&
		args[0] == "core" &&
		args[1] == "log"
}

func hookActivityMetadataRows(sql string, args ...any) (pgx.Rows, bool) {
	if len(args) != 1 || args[0] != int64(1) {
		return nil, false
	}
	switch {
	case strings.Contains(sql, `FROM "field"`):
		return newRecordDataMutationRows([][]any{
			{int64(201), "kind", "Kind", "select", true, false, true, nil, nil, nil, 1, []byte(`{"values":["record"]}`)},
			{int64(202), "operation", "Operation", "select", true, false, true, nil, nil, nil, 2, []byte(`{"values":["create","update","delete"]}`)},
			{int64(203), "status", "Status", "select", true, false, true, nil, nil, nil, 3, []byte(`{"values":["success"]}`)},
			{int64(204), "entity", "Entity", "link", false, false, true, nil, nil, nil, 4, []byte(`{"entity":"entity"}`)},
			{int64(205), "record-id", "Record ID", "bigint", false, false, true, nil, nil, nil, 5, nil},
			{int64(206), "title", "Title", "text", true, false, false, nil, nil, nil, 6, nil},
			{int64(207), "changes", "Changes", "json", false, false, false, nil, nil, nil, 7, nil},
			{int64(208), "snapshot", "Snapshot", "json", false, false, false, nil, nil, nil, 8, nil},
		}), true
	case strings.Contains(sql, `FROM "index"`), strings.Contains(sql, `FROM "constraint"`):
		return newRecordDataMutationRows(nil), true
	default:
		return nil, false
	}
}

func hookLogMetadataRows(sql string, args ...any) (pgx.Rows, bool) {
	if len(args) != 1 || args[0] != int64(2) {
		return nil, false
	}
	switch {
	case strings.Contains(sql, `FROM "field"`):
		return newRecordDataMutationRows(hookLogFieldRows()), true
	case strings.Contains(sql, `FROM "index"`), strings.Contains(sql, `FROM "constraint"`):
		return newRecordDataMutationRows(nil), true
	default:
		return nil, false
	}
}

func hookLogFieldRows() [][]any {
	return [][]any{
		{int64(301), "type", "Type", "select", true, false, false, nil, nil, nil, 1, []byte(`{"values":["Debug","Info","Warning","Error","Panic"]}`)},
		{int64(302), "source", "Source", "select", true, false, false, nil, nil, nil, 2, []byte(`{"values":["Framework","SDK","HTTP","Job","Hook","CLI","Studio"]}`)},
		{int64(303), "app", "App", "link", false, false, false, nil, nil, nil, 3, []byte(`{"entity":"app","foreign-key":false}`)},
		{int64(304), "title", "Title", "text", true, false, false, nil, nil, nil, 4, nil},
		{int64(305), "message", "Message", "long-text", false, false, false, nil, nil, nil, 5, nil},
		{int64(306), "reference-entity", "Reference Entity", "link", false, false, false, nil, nil, nil, 6, []byte(`{"entity":"entity","foreign-key":false}`)},
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

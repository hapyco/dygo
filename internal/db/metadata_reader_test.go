package db

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMetadataReaderListApps(t *testing.T) {
	queryer := &fakeMetadataQueryer{
		rows: []pgx.Rows{newFakeRows([][]any{
			{"core", "Core", "0.1.0", "active"},
			{"studio", "Studio", "0.1.0", "active"},
		})},
	}

	apps, err := NewMetadataReader(queryer).ListApps(context.Background())
	if err != nil {
		t.Fatalf("ListApps() error = %v, want nil", err)
	}
	if len(apps) != 2 || apps[0].Name != "core" || apps[1].Name != "studio" {
		t.Fatalf("ListApps() = %+v, want core and studio", apps)
	}
	if !strings.Contains(queryer.queries[0], `FROM "app"`) || !strings.Contains(queryer.queries[0], "ORDER BY name") {
		t.Fatalf("ListApps() query = %q, want app ordering", queryer.queries[0])
	}
}

func TestMetadataReaderGetApp(t *testing.T) {
	queryer := &fakeMetadataQueryer{
		row: newFakeRow("core", "Core", "0.1.0", "active"),
	}

	app, err := NewMetadataReader(queryer).GetApp(context.Background(), "core")
	if err != nil {
		t.Fatalf("GetApp() error = %v, want nil", err)
	}
	if app.Name != "core" || app.Label != "Core" || app.Status != "active" {
		t.Fatalf("GetApp() = %+v, want core app", app)
	}
	if !reflect.DeepEqual(queryer.rowArgs[0], []any{"core"}) {
		t.Fatalf("GetApp() args = %#v, want core", queryer.rowArgs[0])
	}
}

func TestMetadataReaderListEntities(t *testing.T) {
	queryer := &fakeMetadataQueryer{
		rows: []pgx.Rows{newFakeRows([][]any{
			{"app", "App", "Runtime state", "core", "Core"},
			{"user", "User", "User identity", "core", "Core"},
		})},
	}

	entities, err := NewMetadataReader(queryer).ListEntities(context.Background())
	if err != nil {
		t.Fatalf("ListEntities() error = %v, want nil", err)
	}
	if len(entities) != 2 || entities[0].Name != "app" || entities[0].App.Name != "core" || entities[1].Name != "user" {
		t.Fatalf("ListEntities() = %+v, want core entities", entities)
	}
	if !strings.Contains(queryer.queries[0], `JOIN "app"`) || !strings.Contains(queryer.queries[0], "ORDER BY a.name, e.name") {
		t.Fatalf("ListEntities() query = %q, want app/entity ordering", queryer.queries[0])
	}
}

func TestMetadataReaderGetEntityMeta(t *testing.T) {
	queryer := &fakeMetadataQueryer{
		row: newFakeRow(int64(10), "user", "User", "User identity", "core", "Core"),
		rows: []pgx.Rows{
			newFakeRows([][]any{
				{"email", "Email", "email", true, true, true, nil, 1, []byte(`{"entity":"user"}`)},
				{"enabled", "Enabled", "boolean", false, false, true, []byte(`true`), 2, nil},
			}),
			newFakeRows([][]any{
				{"by-enabled", []byte(`["enabled"]`), 1},
			}),
			newFakeRows([][]any{
				{"user_email_key", "unique", []byte(`["email"]`), "", "", nil, 1},
				{"enabled_check", "check", nil, "enabled", "eq", []byte(`true`), 2},
			}),
		},
	}

	meta, err := NewMetadataReader(queryer).GetEntityMeta(context.Background(), "user")
	if err != nil {
		t.Fatalf("GetEntityMeta() error = %v, want nil", err)
	}
	if meta.Name != "user" || meta.App.Name != "core" {
		t.Fatalf("GetEntityMeta() = %+v, want core/user", meta.MetadataEntity)
	}
	if len(meta.Fields) != 2 || meta.Fields[0].Name != "email" || string(meta.Fields[0].Options) != `{"entity":"user"}` {
		t.Fatalf("GetEntityMeta() fields = %+v, want ordered fields", meta.Fields)
	}
	if string(meta.Fields[1].Default) != "true" {
		t.Fatalf("enabled default = %q, want true", string(meta.Fields[1].Default))
	}
	if len(meta.Indexes) != 1 || meta.Indexes[0].Name != "by-enabled" || string(meta.Indexes[0].Fields) != `["enabled"]` {
		t.Fatalf("GetEntityMeta() indexes = %+v, want by-enabled", meta.Indexes)
	}
	if len(meta.Constraints) != 2 || meta.Constraints[1].Field != "enabled" || string(meta.Constraints[1].Value) != "true" {
		t.Fatalf("GetEntityMeta() constraints = %+v, want check constraint", meta.Constraints)
	}
}

func TestMetadataReaderNotFound(t *testing.T) {
	queryer := &fakeMetadataQueryer{row: fakeRow{err: pgx.ErrNoRows}}

	_, err := NewMetadataReader(queryer).GetEntityMeta(context.Background(), "missing")
	if !IsMetadataNotFound(err) {
		t.Fatalf("GetEntityMeta(missing) error = %v, want metadata not found", err)
	}
}

func TestMetadataReaderQueryFailure(t *testing.T) {
	queryer := &fakeMetadataQueryer{queryErr: errors.New("database failed")}

	_, err := NewMetadataReader(queryer).ListApps(context.Background())
	if err == nil {
		t.Fatal("ListApps() error = nil, want query error")
	}
	if !strings.Contains(err.Error(), "query metadata apps") {
		t.Fatalf("ListApps() error = %q, want query context", err.Error())
	}
}

type fakeMetadataQueryer struct {
	rows     []pgx.Rows
	row      pgx.Row
	queryErr error

	queries []string
	args    [][]any
	rowSQL  []string
	rowArgs [][]any
}

func (q *fakeMetadataQueryer) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.queries = append(q.queries, sql)
	q.args = append(q.args, args)
	if q.queryErr != nil {
		return nil, q.queryErr
	}
	if len(q.rows) == 0 {
		return newFakeRows(nil), nil
	}
	rows := q.rows[0]
	q.rows = q.rows[1:]
	return rows, nil
}

func (q *fakeMetadataQueryer) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.rowSQL = append(q.rowSQL, sql)
	q.rowArgs = append(q.rowArgs, args)
	if q.row == nil {
		return fakeRow{err: pgx.ErrNoRows}
	}
	return q.row
}

type fakeRow struct {
	values []any
	err    error
}

func newFakeRow(values ...any) fakeRow {
	return fakeRow{values: values}
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignScanValues(r.values, dest)
}

type fakeRows struct {
	values [][]any
	index  int
	err    error
	closed bool
}

func newFakeRows(values [][]any) *fakeRows {
	return &fakeRows{values: values, index: -1}
}

func (r *fakeRows) Close() {
	r.closed = true
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	if r.index+1 >= len(r.values) {
		r.closed = true
		return false
	}
	r.index++
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.values) {
		return errors.New("scan without current row")
	}
	return assignScanValues(r.values[r.index], dest)
}

func (r *fakeRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.values) {
		return nil, errors.New("values without current row")
	}
	return r.values[r.index], nil
}

func (r *fakeRows) RawValues() [][]byte {
	return nil
}

func (r *fakeRows) Conn() *pgx.Conn {
	return nil
}

func assignScanValues(values []any, dest []any) error {
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
		case *time.Time:
			v, ok := value.(time.Time)
			if !ok {
				return fmt.Errorf("scan value %d has type %T, want time.Time", i, value)
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

package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
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
			{"core.app", "app", "app", "App", "Runtime state", "package", false, false, false, []byte(`{"strategy":"manual","label":"Name"}`), "core", "Core"},
			{"core.user", "user", "user", "User", "User identity", "user", true, true, false, []byte(`{"strategy":"format","format":"{email}"}`), "core", "Core"},
			{"core.user-role", "user-role", nil, "User Role", "Collection row", "users", false, false, true, nil, "core", "Core"},
		})},
	}

	entities, err := NewMetadataReader(queryer).ListEntities(context.Background())
	if err != nil {
		t.Fatalf("ListEntities() error = %v, want nil", err)
	}
	if len(entities) != 3 || entities[0].Name != "core.app" || entities[0].Key != "app" || entities[0].Icon != "package" || entities[0].App.Name != "core" || entities[0].IsSingle || entities[1].Name != "core.user" || entities[1].RouteSlug() != "user" || !entities[1].IsSingle || !entities[1].IsSystem || entities[2].Slug != nil || !entities[2].IsCollection {
		t.Fatalf("ListEntities() = %+v, want core entities", entities)
	}
	if !strings.Contains(queryer.queries[0], `JOIN "app"`) || !strings.Contains(queryer.queries[0], "ORDER BY a.name, e.key") {
		t.Fatalf("ListEntities() query = %q, want app/entity ordering", queryer.queries[0])
	}
}

func TestMetadataReaderGetEntityMeta(t *testing.T) {
	queryer := &fakeMetadataQueryer{
		row: newFakeRow(int64(10), "core.user", "user", "user", "User", "User identity", "user", true, true, false, []byte(`{"strategy":"format","format":"{email}"}`), "core", "Core"),
		rows: []pgx.Rows{
			newFakeRows([][]any{
				{int64(1), "email", "Email", "email", true, true, true, nil, nil, 1, []byte(`{"entity":"user"}`)},
				{int64(2), "enabled", "Enabled", "boolean", false, false, true, []byte(`true`), []byte(`{"operator":"eq","value":true}`), 2, nil},
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
	if meta.Name != "core.user" || meta.Key != "user" || meta.RouteSlug() != "user" || meta.Icon != "user" || meta.App.Name != "core" || !meta.IsSingle || !meta.IsSystem {
		t.Fatalf("GetEntityMeta() = %+v, want core/user", meta.MetadataEntity)
	}
	if len(meta.Fields) != 2 || meta.Fields[0].Name != "email" || string(meta.Fields[0].Options) != `{"entity":"user"}` {
		t.Fatalf("GetEntityMeta() fields = %+v, want ordered fields", meta.Fields)
	}
	if len(meta.SystemFields) != 4 || meta.SystemFields[1].Name != "name" || !meta.SystemFields[1].NameRenderable {
		t.Fatalf("GetEntityMeta() system fields = %+v, want framework system field metadata", meta.SystemFields)
	}
	if string(meta.Fields[1].Default) != "true" {
		t.Fatalf("enabled default = %q, want true", string(meta.Fields[1].Default))
	}
	if string(meta.Fields[1].Check) != `{"operator":"eq","value":true}` {
		t.Fatalf("enabled check = %q, want field check metadata", string(meta.Fields[1].Check))
	}
	if len(meta.Indexes) != 1 || meta.Indexes[0].Name != "by-enabled" || string(meta.Indexes[0].Fields) != `["enabled"]` {
		t.Fatalf("GetEntityMeta() indexes = %+v, want by-enabled", meta.Indexes)
	}
	if len(meta.Constraints) != 2 || meta.Constraints[1].Field != "enabled" || string(meta.Constraints[1].Value) != "true" {
		t.Fatalf("GetEntityMeta() constraints = %+v, want check constraint", meta.Constraints)
	}
}

func TestMetadataReaderEmbedsCollectionMetadata(t *testing.T) {
	queryer := &fakeMetadataQueryer{
		row: newFakeRow(int64(20), "crm.lead", "lead", "lead", "Lead", "Sales lead", "contact", false, false, false, []byte(`{"strategy":"random","length":16}`), "crm", "CRM"),
		identityRows: map[string]pgx.Row{
			"crm/lead-contact": newFakeRow(int64(21), "crm.lead-contact", "lead-contact", "", "Lead Contact", "Child row", "contact", false, false, true, nil, "crm", "CRM"),
		},
		rows: []pgx.Rows{
			newFakeRows([][]any{
				{int64(1), "status", "Status", "select", true, false, false, nil, nil, 1, []byte(`{"values":["New"]}`)},
				{int64(2), "contacts", "Contacts", "collection", false, false, false, nil, nil, 2, []byte(`{"entity":"lead-contact"}`)},
			}),
			newFakeRows(nil),
			newFakeRows(nil),
			newFakeRows([][]any{
				{int64(1), "email", "Email", "email", true, false, false, nil, nil, 1, nil},
			}),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}

	meta, err := NewMetadataReader(queryer).GetEntityMeta(context.Background(), "lead")
	if err != nil {
		t.Fatalf("GetEntityMeta(collection) error = %v, want nil", err)
	}
	child, ok := meta.Collections["contacts"]
	if !ok {
		t.Fatalf("GetEntityMeta(collection) collections = %#v, want contacts child metadata", meta.Collections)
	}
	if child.Key != "lead-contact" || !child.IsCollection || len(child.Fields) != 1 || child.Fields[0].Name != "email" {
		t.Fatalf("embedded child metadata = %+v fields %+v, want lead-contact email field", child.MetadataEntity, child.Fields)
	}
	if child.Naming != nil {
		t.Fatalf("embedded child naming = %s, want nil framework-owned naming metadata", string(child.Naming))
	}
}

func TestMetadataReaderGetEntityMetaByIdentity(t *testing.T) {
	queryer := &fakeMetadataQueryer{
		row: newFakeRow(int64(20), "crm.lead", "lead", "crm-lead", "Lead", "Sales lead", "contact", false, false, false, []byte(`{"strategy":"random","length":16}`), "crm", "CRM"),
		rows: []pgx.Rows{
			newFakeRows([][]any{
				{int64(1), "status", "Status", "select", true, false, false, nil, nil, 1, []byte(`{"values":["New"]}`)},
			}),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}

	meta, err := NewMetadataReader(queryer).GetEntityMetaByIdentity(context.Background(), "crm", "lead")
	if err != nil {
		t.Fatalf("GetEntityMetaByIdentity() error = %v, want nil", err)
	}
	if meta.Name != "crm.lead" || meta.Key != "lead" || meta.RouteSlug() != "crm-lead" || meta.Icon != "contact" || meta.App.Name != "crm" {
		t.Fatalf("GetEntityMetaByIdentity() = %+v, want crm/lead with crm-lead route slug", meta.MetadataEntity)
	}
	if !strings.Contains(queryer.rowSQL[0], `WHERE a.name = $1 AND e.key = $2`) {
		t.Fatalf("GetEntityMetaByIdentity() query = %q, want app/entity lookup", queryer.rowSQL[0])
	}
	if !reflect.DeepEqual(queryer.rowArgs[0], []any{"crm", "lead"}) {
		t.Fatalf("GetEntityMetaByIdentity() args = %#v, want app/entity", queryer.rowArgs[0])
	}
}

func TestMetadataAPIJSONFieldNames(t *testing.T) {
	meta := MetadataEntityMeta{
		MetadataEntity: MetadataEntity{
			Name:         "core.user",
			Key:          "user",
			Slug:         stringPointerOrNil("user"),
			Label:        "User",
			Description:  "User identity",
			Icon:         "user",
			IsSingle:     false,
			IsSystem:     false,
			IsCollection: false,
			Naming:       json.RawMessage(`{"strategy":"format","format":"{email}"}`),
			App:          MetadataAppRef{Name: "core", Label: "Core"},
		},
		Fields: []MetadataField{{
			Name:           "email",
			Label:          "Email",
			Type:           "email",
			Required:       true,
			Unique:         true,
			Index:          true,
			Stored:         true,
			WriteOnly:      false,
			Listable:       true,
			NameRenderable: true,
			ValueKind:      "string",
			Studio:         MetadataFieldStudio{Editor: "text", Display: "email"},
			Default:        json.RawMessage(`"admin@example.com"`),
			Check:          json.RawMessage(`{"operator":"neq","value":""}`),
			Position:       1,
			Options:        json.RawMessage(`{"values":["admin@example.com"]}`),
		}},
		SystemFields: metadataSystemFields(),
		Indexes: []MetadataIndex{{
			Name:     "by-email",
			Fields:   json.RawMessage(`["email"]`),
			Position: 1,
		}},
		Constraints: []MetadataConstraint{{
			Name:     "user_email_key",
			Type:     "unique",
			Fields:   json.RawMessage(`["email"]`),
			Position: 1,
		}},
	}

	encoded, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Marshal(MetadataEntityMeta) error = %v, want nil", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("Unmarshal(metadata json) error = %v, want nil", err)
	}

	assertJSONKeys(t, payload, []string{
		"app",
		"constraints",
		"description",
		"fields",
		"icon",
		"indexes",
		"is-collection",
		"is-single",
		"is-system",
		"key",
		"label",
		"name",
		"naming",
		"slug",
		"system-fields",
	})
	for _, legacy := range []string{"route-slug", "route_slug", "is_single", "is_system", "is_collection"} {
		if _, ok := payload[legacy]; ok {
			t.Fatalf("metadata payload has legacy key %q: %s", legacy, encoded)
		}
	}

	fields := payload["fields"].([]any)
	field := fields[0].(map[string]any)
	assertJSONKeys(t, field, []string{
		"check",
		"default",
		"index",
		"label",
		"listable",
		"name",
		"name-renderable",
		"options",
		"position",
		"required",
		"stored",
		"studio",
		"type",
		"unique",
		"value-kind",
		"write-only",
	})
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
	rows         []pgx.Rows
	row          pgx.Row
	identityRows map[string]pgx.Row
	queryErr     error

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
	if len(args) == 2 {
		key := fmt.Sprintf("%v/%v", args[0], args[1])
		if row, ok := q.identityRows[key]; ok {
			return row
		}
	}
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

func assertJSONKeys(t *testing.T, payload map[string]any, want []string) {
	t.Helper()
	got := make([]string, 0, len(payload))
	for key := range payload {
		got = append(got, key)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON keys = %#v, want %#v", got, want)
	}
}

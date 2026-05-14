package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/db"
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
	sdkEvents := sdkRecordHookEventConstants(t)

	tests := []struct {
		name      string
		constName string
		sdk       sdk.RecordHookEvent
		db        db.RecordHookEvent
	}{
		{name: "before validate", constName: "RecordBeforeValidate", sdk: sdk.RecordBeforeValidate, db: db.RecordBeforeValidate},
		{name: "validate", constName: "RecordValidate", sdk: sdk.RecordValidate, db: db.RecordValidate},
		{name: "before create", constName: "RecordBeforeCreate", sdk: sdk.RecordBeforeCreate, db: db.RecordBeforeCreate},
		{name: "after create", constName: "RecordAfterCreate", sdk: sdk.RecordAfterCreate, db: db.RecordAfterCreate},
		{name: "before update", constName: "RecordBeforeUpdate", sdk: sdk.RecordBeforeUpdate, db: db.RecordBeforeUpdate},
		{name: "after update", constName: "RecordAfterUpdate", sdk: sdk.RecordAfterUpdate, db: db.RecordAfterUpdate},
		{name: "before delete", constName: "RecordBeforeDelete", sdk: sdk.RecordBeforeDelete, db: db.RecordBeforeDelete},
		{name: "after delete", constName: "RecordAfterDelete", sdk: sdk.RecordAfterDelete, db: db.RecordAfterDelete},
	}
	if len(tests) != len(sdkEvents) {
		t.Fatalf("record hook mapping tests cover %d SDK events, SDK defines %d: %#v", len(tests), len(sdkEvents), sdkEvents)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := sdkEvents[tt.constName]; got != tt.sdk {
				t.Fatalf("SDK %s = %q, want %q", tt.constName, got, tt.sdk)
			}
			got, err := recordHookEvent(tt.sdk)
			if err != nil {
				t.Fatalf("recordHookEvent(%q) error = %v, want nil", tt.sdk, err)
			}
			if got != tt.db {
				t.Fatalf("recordHookEvent(%q) = %q, want %q", tt.sdk, got, tt.db)
			}
		})
	}
}

func sdkRecordHookEventConstants(t *testing.T) map[string]sdk.RecordHookEvent {
	t.Helper()

	events := map[string]sdk.RecordHookEvent{}
	fset := token.NewFileSet()
	dir := filepath.Join("..", "..", "pkg", "sdk")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read SDK package directory: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.ValueSpec)
			if !ok || spec.Type == nil {
				return true
			}
			ident, ok := spec.Type.(*ast.Ident)
			if !ok || ident.Name != "RecordHookEvent" {
				return true
			}
			for i, name := range spec.Names {
				if i >= len(spec.Values) {
					continue
				}
				lit, ok := spec.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("unquote %s: %v", lit.Value, err)
				}
				events[name.Name] = sdk.RecordHookEvent(value)
			}
			return true
		})
	}
	if len(events) == 0 {
		t.Fatal("no SDK RecordHookEvent constants found")
	}
	return events
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
	if !strings.Contains(queryer.rowSQL[0], `WHERE a.name = $1 AND e.name = $2`) {
		t.Fatalf("Records.Get() metadata query = %q, want app/entity lookup", queryer.rowSQL[0])
	}
	if !reflect.DeepEqual(queryer.rowArgs[0], []any{"sales", "lead"}) {
		t.Fatalf("Records.Get() metadata args = %#v, want sales/lead", queryer.rowArgs[0])
	}
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

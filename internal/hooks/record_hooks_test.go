package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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

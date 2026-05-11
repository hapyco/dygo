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
)

func TestNewRecordHookRegistryAppliesRegistrarsInOrder(t *testing.T) {
	t.Parallel()

	var order []string
	registry, err := NewRecordHookRegistry([]sdk.RecordHookRegistrar{
		func(registry sdk.RecordHookRegistry) error {
			return registry.RegisterEntity("lead", sdk.RecordBeforeCreate, "first", func(_ context.Context, hookCtx sdk.RecordHookContext) error {
				order = append(order, "first:"+hookCtx.Entity)
				hookCtx.Input["status"] = json.RawMessage(`"qualified"`)
				return nil
			})
		},
		func(registry sdk.RecordHookRegistry) error {
			return registry.RegisterEntity("lead", sdk.RecordBeforeCreate, "second", func(_ context.Context, hookCtx sdk.RecordHookContext) error {
				order = append(order, "second:"+string(hookCtx.Event))
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
		Entity:      "lead",
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
			return registry.RegisterEntity("lead", sdk.RecordBeforeCreate, "missing", nil)
		},
	})
	if err == nil {
		t.Fatal("NewRecordHookRegistry(nil hook) error = nil, want error")
	}
	if !strings.Contains(err.Error(), `record hook "missing" function is required`) {
		t.Fatalf("NewRecordHookRegistry(nil hook) error = %q, want nil hook context", err.Error())
	}
}

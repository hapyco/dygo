package db

import (
	"context"
	"fmt"
	"strings"
)

// RecordHookEvent names a Record lifecycle hook phase.
type RecordHookEvent string

const (
	RecordBeforeValidate RecordHookEvent = "before-validate"
	RecordValidate       RecordHookEvent = "validate"
	RecordBeforeCreate   RecordHookEvent = "before-create"
	RecordAfterCreate    RecordHookEvent = "after-create"
	RecordBeforeUpdate   RecordHookEvent = "before-update"
	RecordAfterUpdate    RecordHookEvent = "after-update"
	RecordBeforeDelete   RecordHookEvent = "before-delete"
	RecordAfterDelete    RecordHookEvent = "after-delete"
)

// RecordHookFunc handles one Record lifecycle hook.
type RecordHookFunc func(context.Context, RecordHookContext) error

// RecordHookContext contains the Record lifecycle state visible to hooks.
type RecordHookContext struct {
	Event     RecordHookEvent
	Operation string

	EntityID    int64
	Entity      string
	EntityLabel string
	Table       string
	RecordID    int64

	Input     RecordInput
	OldRecord Record
	NewRecord Record
	Changes   []map[string]any
	Snapshot  Record

	// Queryer is the current mutation queryer, usually the active transaction.
	Queryer RecordQueryer
}

// RecordHookRegistry stores global and Entity-scoped Record lifecycle hooks.
type RecordHookRegistry struct {
	global map[RecordHookEvent][]recordHookDefinition
	entity map[string]map[RecordHookEvent][]recordHookDefinition
}

type recordHookDefinition struct {
	Name string
	Fn   RecordHookFunc
}

// NewRecordHookRegistry returns an empty Record hook registry.
func NewRecordHookRegistry() *RecordHookRegistry {
	return &RecordHookRegistry{
		global: map[RecordHookEvent][]recordHookDefinition{},
		entity: map[string]map[RecordHookEvent][]recordHookDefinition{},
	}
}

// DefaultRecordHookRegistry returns dygo's built-in framework Record hooks.
func DefaultRecordHookRegistry() *RecordHookRegistry {
	registry := NewRecordHookRegistry()
	mustRegisterRecordHook(registry.RegisterGlobal(RecordAfterCreate, "activity-history", recordActivityHook))
	mustRegisterRecordHook(registry.RegisterGlobal(RecordAfterUpdate, "activity-history", recordActivityHook))
	mustRegisterRecordHook(registry.RegisterGlobal(RecordAfterDelete, "activity-history", recordActivityHook))
	return registry
}

// RegisterGlobal registers one framework/global hook for event.
func (r *RecordHookRegistry) RegisterGlobal(event RecordHookEvent, name string, fn RecordHookFunc) error {
	if err := validateRecordHook(event, name, fn); err != nil {
		return err
	}
	r.ensure()
	r.global[event] = append(r.global[event], recordHookDefinition{Name: name, Fn: fn})
	return nil
}

// RegisterEntity registers one Entity-scoped hook for event.
func (r *RecordHookRegistry) RegisterEntity(entity string, event RecordHookEvent, name string, fn RecordHookFunc) error {
	if strings.TrimSpace(entity) == "" {
		return fmt.Errorf("record hook entity is required")
	}
	if err := validateRecordHook(event, name, fn); err != nil {
		return err
	}
	r.ensure()
	if r.entity[entity] == nil {
		r.entity[entity] = map[RecordHookEvent][]recordHookDefinition{}
	}
	r.entity[entity][event] = append(r.entity[entity][event], recordHookDefinition{Name: name, Fn: fn})
	return nil
}

// Run runs global hooks, then Entity-scoped hooks, for ctx.Event.
func (r *RecordHookRegistry) Run(ctx context.Context, hookCtx RecordHookContext) error {
	if r == nil {
		return nil
	}
	r.ensure()
	for _, hook := range r.global[hookCtx.Event] {
		if err := runRecordHook(ctx, hookCtx, hook); err != nil {
			return err
		}
	}
	for _, hook := range r.entity[hookCtx.Entity][hookCtx.Event] {
		if err := runRecordHook(ctx, hookCtx, hook); err != nil {
			return err
		}
	}
	return nil
}

func (r *RecordHookRegistry) ensure() {
	if r.global == nil {
		r.global = map[RecordHookEvent][]recordHookDefinition{}
	}
	if r.entity == nil {
		r.entity = map[string]map[RecordHookEvent][]recordHookDefinition{}
	}
}

func validateRecordHook(event RecordHookEvent, name string, fn RecordHookFunc) error {
	if !isRecordHookEvent(event) {
		return fmt.Errorf("record hook event %q is not supported", event)
	}
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("record hook name is required")
	}
	if fn == nil {
		return fmt.Errorf("record hook %q function is required", name)
	}
	return nil
}

func isRecordHookEvent(event RecordHookEvent) bool {
	switch event {
	case RecordBeforeValidate,
		RecordValidate,
		RecordBeforeCreate,
		RecordAfterCreate,
		RecordBeforeUpdate,
		RecordAfterUpdate,
		RecordBeforeDelete,
		RecordAfterDelete:
		return true
	default:
		return false
	}
}

func runRecordHook(ctx context.Context, hookCtx RecordHookContext, hook recordHookDefinition) error {
	if err := hook.Fn(ctx, hookCtx); err != nil {
		if IsRecordError(err) {
			return err
		}
		return recordError(RecordErrorValidation, "record hook failed", map[string]any{
			"entity": hookCtx.Entity,
			"event":  string(hookCtx.Event),
			"hook":   hook.Name,
		}, err)
	}
	return nil
}

func mustRegisterRecordHook(err error) {
	if err != nil {
		panic(err)
	}
}

func newRecordHookContext(event RecordHookEvent, layout recordLayout) RecordHookContext {
	return RecordHookContext{
		Event:       event,
		EntityID:    layout.EntityID,
		Entity:      layout.Entity,
		EntityLabel: layout.Label,
		Table:       layout.Table,
	}
}

func cloneRecordInput(input RecordInput) RecordInput {
	if input == nil {
		return RecordInput{}
	}
	cloned := make(RecordInput, len(input))
	for key, value := range input {
		clonedValue := make([]byte, len(value))
		copy(clonedValue, value)
		cloned[key] = clonedValue
	}
	return cloned
}

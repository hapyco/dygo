package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/hapyco/dygo/internal/hookevents"
)

// RecordHookEvent names a Record lifecycle hook phase.
type RecordHookEvent string

const (
	RecordBeforeValidate RecordHookEvent = hookevents.BeforeValidate
	RecordValidate       RecordHookEvent = hookevents.Validate
	RecordBeforeCreate   RecordHookEvent = hookevents.BeforeCreate
	RecordAfterCreate    RecordHookEvent = hookevents.AfterCreate
	RecordBeforeUpdate   RecordHookEvent = hookevents.BeforeUpdate
	RecordAfterUpdate    RecordHookEvent = hookevents.AfterUpdate
	RecordBeforeDelete   RecordHookEvent = hookevents.BeforeDelete
	RecordAfterDelete    RecordHookEvent = hookevents.AfterDelete
)

// RecordHookFunc handles one Record lifecycle hook.
type RecordHookFunc func(context.Context, RecordHookContext) error

// RecordHookContext contains the Record lifecycle state visible to hooks.
type RecordHookContext struct {
	Event     RecordHookEvent
	Operation string

	EntityID    int64
	AppName     string
	Entity      string
	RouteSlug   string
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
func (r *RecordHookRegistry) RegisterEntity(appName string, entity string, event RecordHookEvent, name string, fn RecordHookFunc) error {
	if strings.TrimSpace(appName) == "" {
		return fmt.Errorf("record hook app is required")
	}
	if strings.TrimSpace(entity) == "" {
		return fmt.Errorf("record hook entity is required")
	}
	if err := validateRecordHook(event, name, fn); err != nil {
		return err
	}
	r.ensure()
	key := entityKey(appName, entity)
	if r.entity[key] == nil {
		r.entity[key] = map[RecordHookEvent][]recordHookDefinition{}
	}
	r.entity[key][event] = append(r.entity[key][event], recordHookDefinition{Name: name, Fn: fn})
	return nil
}

// Run runs global hooks, then Entity-scoped hooks, for ctx.Event.
func (r *RecordHookRegistry) Run(ctx context.Context, hookCtx RecordHookContext) error {
	if r == nil {
		return nil
	}
	r.ensure()
	for _, hook := range r.global[hookCtx.Event] {
		if err := runRecordHook(ctx, recordHookContextForRun(hookCtx), hook); err != nil {
			return err
		}
	}
	for _, hook := range r.entity[entityKey(hookCtx.AppName, hookCtx.Entity)][hookCtx.Event] {
		if err := runRecordHook(ctx, recordHookContextForRun(hookCtx), hook); err != nil {
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

func (r *RecordHookRegistry) withoutHook(name string) *RecordHookRegistry {
	if r == nil {
		return nil
	}
	r.ensure()
	filtered := NewRecordHookRegistry()
	for event, hooks := range r.global {
		filtered.global[event] = recordHooksWithoutName(hooks, name)
	}
	for entity, events := range r.entity {
		filtered.entity[entity] = map[RecordHookEvent][]recordHookDefinition{}
		for event, hooks := range events {
			filtered.entity[entity][event] = recordHooksWithoutName(hooks, name)
		}
	}
	return filtered
}

func recordHooksWithoutName(hooks []recordHookDefinition, name string) []recordHookDefinition {
	filtered := make([]recordHookDefinition, 0, len(hooks))
	for _, hook := range hooks {
		if hook.Name != name {
			filtered = append(filtered, hook)
		}
	}
	return filtered
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
	return hookevents.Supported(string(event))
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

func recordHookContextForRun(hookCtx RecordHookContext) RecordHookContext {
	hookCtx.OldRecord = cloneRecord(hookCtx.OldRecord)
	hookCtx.NewRecord = cloneRecord(hookCtx.NewRecord)
	hookCtx.Changes = cloneRecordChanges(hookCtx.Changes)
	hookCtx.Snapshot = cloneRecord(hookCtx.Snapshot)
	if !recordHookEventMutatesTargetInput(hookCtx.Event) {
		hookCtx.Input = cloneRecordInput(hookCtx.Input)
	}
	return hookCtx
}

func recordHookEventMutatesTargetInput(event RecordHookEvent) bool {
	return hookevents.MutatesInput(string(event))
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
		AppName:     layout.AppName,
		Entity:      layout.Entity,
		RouteSlug:   layout.Slug,
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

func cloneRecord(record Record) Record {
	if record == nil {
		return nil
	}
	cloned := make(Record, len(record))
	for key, value := range record {
		cloned[key] = value
	}
	return cloned
}

func cloneRecordChanges(changes []map[string]any) []map[string]any {
	if changes == nil {
		return nil
	}
	cloned := make([]map[string]any, len(changes))
	for i, change := range changes {
		clonedChange := make(map[string]any, len(change))
		for key, value := range change {
			clonedChange[key] = value
		}
		cloned[i] = clonedChange
	}
	return cloned
}

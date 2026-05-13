// Package sdk exposes the public extension points app code can compile against.
package sdk

import (
	"context"
	"encoding/json"
)

// RecordHookEvent names a Record lifecycle hook phase.
type RecordHookEvent string

const (
	// RecordBeforeValidate runs before dygo validates Record input.
	RecordBeforeValidate RecordHookEvent = "before-validate"
	// RecordValidate runs during dygo Record validation.
	RecordValidate RecordHookEvent = "validate"
	// RecordBeforeCreate runs before dygo creates a Record.
	RecordBeforeCreate RecordHookEvent = "before-create"
	// RecordAfterCreate runs after dygo creates a Record inside the same transaction.
	RecordAfterCreate RecordHookEvent = "after-create"
	// RecordBeforeUpdate runs before dygo updates a Record.
	RecordBeforeUpdate RecordHookEvent = "before-update"
	// RecordAfterUpdate runs after dygo updates a Record inside the same transaction.
	RecordAfterUpdate RecordHookEvent = "after-update"
	// RecordBeforeDelete runs before dygo deletes a Record.
	RecordBeforeDelete RecordHookEvent = "before-delete"
	// RecordAfterDelete runs after dygo deletes a Record inside the same transaction.
	RecordAfterDelete RecordHookEvent = "after-delete"
)

const (
	// RecordOperationCreate marks a Record create mutation.
	RecordOperationCreate = "create"
	// RecordOperationUpdate marks a Record update mutation.
	RecordOperationUpdate = "update"
	// RecordOperationDelete marks a Record delete mutation.
	RecordOperationDelete = "delete"
)

// RecordInput is decoded create or update input visible to Record hooks.
type RecordInput map[string]json.RawMessage

// Record is one metadata-backed saved Entity instance visible to Record hooks.
type Record map[string]any

// RecordHookContext contains the Record lifecycle state visible to app hooks.
type RecordHookContext struct {
	Event     RecordHookEvent
	Operation string

	EntityID    int64
	AppName     string
	Entity      string
	RouteSlug   string
	EntityLabel string
	RecordID    int64

	Input     RecordInput
	OldRecord Record
	NewRecord Record
	Changes   []map[string]any
	Snapshot  Record
}

// RecordHookFunc handles one Record lifecycle hook.
type RecordHookFunc func(context.Context, RecordHookContext) error

// RecordHookRegistry is the public app-facing Record hook registration API.
type RecordHookRegistry interface {
	RegisterEntity(appName string, entity string, event RecordHookEvent, name string, fn RecordHookFunc) error
}

// RecordHookRegistrar registers compiled app hooks with dygo.
type RecordHookRegistrar func(RecordHookRegistry) error

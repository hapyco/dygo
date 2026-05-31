// Package dygo exposes the public extension points app code can compile against.
package dygo

import (
	"context"
	"encoding/json"

	"github.com/hapyco/dygo/internal/corevalues"
	"github.com/hapyco/dygo/internal/hookevents"
)

// RecordHookEvent names a Record lifecycle hook phase.
type RecordHookEvent string

const (
	// RecordBeforeValidate runs before dygo validates Record input. Mutating Input changes the input dygo validates.
	RecordBeforeValidate RecordHookEvent = hookevents.BeforeValidate
	// RecordValidate runs after framework input validation. Return an error to reject; mutate other Records through Records.
	RecordValidate RecordHookEvent = hookevents.Validate
	// RecordBeforeCreate runs before dygo creates a Record. Mutating Input changes the row dygo inserts.
	RecordBeforeCreate RecordHookEvent = hookevents.BeforeCreate
	// RecordAfterCreate runs after dygo creates a Record inside the same transaction.
	RecordAfterCreate RecordHookEvent = hookevents.AfterCreate
	// RecordBeforeUpdate runs before dygo updates a Record. Mutating Input changes the row dygo updates.
	RecordBeforeUpdate RecordHookEvent = hookevents.BeforeUpdate
	// RecordAfterUpdate runs after dygo updates a Record inside the same transaction.
	RecordAfterUpdate RecordHookEvent = hookevents.AfterUpdate
	// RecordBeforeDelete runs before dygo deletes a Record.
	RecordBeforeDelete RecordHookEvent = hookevents.BeforeDelete
	// RecordAfterDelete runs after dygo deletes a Record inside the same transaction.
	RecordAfterDelete RecordHookEvent = hookevents.AfterDelete
)

// SupportedRecordHookEvents returns supported Record hook events in lifecycle order.
func SupportedRecordHookEvents() []RecordHookEvent {
	specs := hookevents.Specs()
	events := make([]RecordHookEvent, len(specs))
	for index, spec := range specs {
		events[index] = RecordHookEvent(spec.Name)
	}
	return events
}

const (
	// RecordOperationCreate marks a Record create mutation.
	RecordOperationCreate = corevalues.ActivityOperationCreate
	// RecordOperationUpdate marks a Record update mutation.
	RecordOperationUpdate = corevalues.ActivityOperationUpdate
	// RecordOperationDelete marks a Record delete mutation.
	RecordOperationDelete = corevalues.ActivityOperationDelete
)

// RecordInput is decoded create or update input visible to Record hooks.
type RecordInput map[string]json.RawMessage

// Record is one metadata-backed saved Entity instance visible to Record hooks.
type Record map[string]any

// RecordListParams controls Record list pagination in hook code.
type RecordListParams struct {
	Limit   int
	Offset  int
	Filters []RecordFilter
	Sort    []RecordSort
}

// RecordFilter is one operator-based Record list filter on a metadata or system field.
type RecordFilter struct {
	Field    string
	Operator string
	Value    string
}

// RecordSort is one Record list sort term on a metadata or system field.
type RecordSort struct {
	Field string
	Desc  bool
}

// RecordListResult is a page of Records returned to hook code.
type RecordListResult struct {
	Records []Record
	Limit   int
	Offset  int
	Count   int
}

// RecordData gives hooks transactional access to metadata-backed Records by app/entity identity.
// Writes run dygo framework hooks, such as Activity, but do not re-enter app hooks.
type RecordData interface {
	List(ctx context.Context, appName string, entity string, params RecordListParams) (RecordListResult, error)
	Get(ctx context.Context, appName string, entity string, id int64) (Record, error)
	Find(ctx context.Context, appName string, entity string, match RecordInput) (Record, error)
	Create(ctx context.Context, appName string, entity string, input RecordInput) (Record, error)
	Update(ctx context.Context, appName string, entity string, id int64, input RecordInput) (Record, error)
	Delete(ctx context.Context, appName string, entity string, id int64) error
}

// RecordHook contains the Record lifecycle state and services visible to app hooks.
type RecordHook struct {
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

	// Records performs metadata-backed Record reads and writes by app/entity identity in the current hook transaction.
	Records RecordData
	// Jobs enqueues durable background work in the current hook transaction when available.
	Jobs JobData
}

// RecordHookContext is kept as an alias for older hook code. Prefer RecordHook.
type RecordHookContext = RecordHook

// RecordHookFunc handles one Record lifecycle hook.
type RecordHookFunc func(context.Context, RecordHook) error

// RecordHookRegistry is the public app-facing Record hook registration API.
type RecordHookRegistry interface {
	RegisterEntity(appName string, entity string, event RecordHookEvent, name string, fn RecordHookFunc) error
}

// RecordHookRegistrar registers compiled app hooks with dygo.
type RecordHookRegistrar func(RecordHookRegistry) error

// Package actions registers and executes app-owned Entity actions.
package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/dygodata"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/pkg/dygo"
	"github.com/jackc/pgx/v5"
)

type registeredAction struct {
	definition dygo.EntityActionDefinition
	fn         dygo.EntityActionFunc
}

// Registry stores compiled Entity action handlers.
type Registry struct {
	actions map[string]registeredAction
}

// NewRegistry returns a Registry populated by app registrars.
func NewRegistry(registrars []dygo.EntityActionRegistrar) (*Registry, error) {
	registry := &Registry{actions: map[string]registeredAction{}}
	for index, registrar := range registrars {
		if registrar == nil {
			return nil, fmt.Errorf("Entity action registrar %d is required", index+1)
		}
		if err := registrar(registry); err != nil {
			return nil, fmt.Errorf("register Entity action registrar %d: %w", index+1, err)
		}
	}
	return registry, nil
}

// RegisterEntity registers one app-owned Entity action.
func (r *Registry) RegisterEntity(appName string, entity string, definition dygo.EntityActionDefinition, fn dygo.EntityActionFunc) error {
	appName = strings.TrimSpace(appName)
	entity = strings.TrimSpace(entity)
	definition.Name = strings.TrimSpace(definition.Name)
	definition.Label = strings.TrimSpace(definition.Label)
	if appName == "" || entity == "" || definition.Name == "" {
		return fmt.Errorf("Entity action app, Entity, and name are required")
	}
	if definition.Label == "" {
		return fmt.Errorf("Entity action %s/%s/%s label is required", appName, entity, definition.Name)
	}
	if !validSelection(definition.Selection) {
		return fmt.Errorf("Entity action %s/%s/%s selection %q is not supported", appName, entity, definition.Name, definition.Selection)
	}
	if fn == nil {
		return fmt.Errorf("Entity action %s/%s/%s function is required", appName, entity, definition.Name)
	}
	if r.actions == nil {
		r.actions = map[string]registeredAction{}
	}
	key := actionKey(appName, entity, definition.Name)
	if _, exists := r.actions[key]; exists {
		return fmt.Errorf("Entity action %s/%s/%s is already registered", appName, entity, definition.Name)
	}
	r.actions[key] = registeredAction{definition: definition, fn: fn}
	return nil
}

// Definitions returns registered action metadata for one Entity.
func (r *Registry) Definitions(appName string, entity string) []dygo.EntityActionDefinition {
	definitions := []dygo.EntityActionDefinition{}
	prefix := actionKey(appName, entity, "")
	for key, action := range r.actions {
		if strings.HasPrefix(key, prefix) {
			definitions = append(definitions, action.definition)
		}
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Name < definitions[j].Name })
	return definitions
}

func (r *Registry) action(appName string, entity string, name string) (registeredAction, bool) {
	if r == nil {
		return registeredAction{}, false
	}
	action, ok := r.actions[actionKey(appName, entity, name)]
	return action, ok
}

func actionKey(appName string, entity string, name string) string {
	return appName + "\x00" + entity + "\x00" + name
}

func validSelection(selection dygo.ActionSelection) bool {
	return selection == dygo.ActionSelectionRecord || selection == dygo.ActionSelectionSelection || selection == dygo.ActionSelectionCollection
}

// Beginner is the transaction behavior needed by the action executor.
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Executor runs registered Entity actions in one transaction.
type Executor struct {
	DB          Beginner
	Registry    *Registry
	RecordHooks *db.RecordHookRegistry
	Authorizer  dygo.Authorizer
	Files       dygo.FileData
}

// Execute runs one Entity action addressed by public Entity slug.
func (e Executor) Execute(ctx context.Context, slug string, name string, actor dygo.Actor, recordIDs []int64, input json.RawMessage) (any, error) {
	if e.DB == nil || e.Registry == nil || e.Authorizer == nil {
		return nil, dygo.ActionError{Code: "internal_error", Message: "Entity action service is unavailable"}
	}
	tx, err := e.DB.Begin(ctx)
	if err != nil {
		return nil, dygo.ActionError{Code: "internal_error", Message: "Entity action transaction could not start"}
	}
	defer tx.Rollback(ctx)

	meta, err := db.NewMetadataReader(tx).GetEntityMeta(ctx, slug)
	if err != nil {
		return nil, err
	}
	action, ok := e.Registry.action(meta.App.Name, meta.Key, name)
	if !ok {
		return nil, dygo.ActionError{Code: "not_found", Message: "Entity action not found", Details: map[string]any{"entity": slug, "action": name}}
	}
	if err := validateRecordIDs(action.definition.Selection, recordIDs); err != nil {
		return nil, err
	}
	if action.definition.Selection == dygo.ActionSelectionCollection {
		if err := e.Authorizer.Authorize(ctx, dygo.PermissionRequest{
			Actor:    actor,
			Resource: dygo.Resource{Kind: dygo.ResourceEntity, App: meta.App.Name, Name: meta.Key},
			Action:   dygo.Action(name),
		}); err != nil {
			return nil, err
		}
	} else {
		ids := append([]int64(nil), recordIDs...)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		locker := dygodata.NewRecordData(tx, e.RecordHooks).WithLockAction(actor, dygo.Action(name))
		for _, recordID := range ids {
			locked, err := locker.Lock(ctx, meta.App.Name, meta.Key, dygo.RecordListParams{Limit: 1, Filters: []dygo.RecordFilter{{Field: "id", Operator: "eq", Value: strconv.FormatInt(recordID, 10)}}})
			if err != nil {
				return nil, err
			}
			if len(locked.Records) != 1 {
				return nil, dygo.ActionError{Code: "permission_denied", Message: "permission denied"}
			}
		}
	}

	actionCtx := db.WithActivityActor(ctx, actor.UserID, actor.Email, actor.Administrator)
	jobs, err := dygodata.NewJobDataFromBeginner(tx)
	if err != nil {
		return nil, err
	}
	actionFiles, rollbackFiles := actorFiles(e.Files, tx, actor)
	keepFiles := false
	defer func() {
		if !keepFiles {
			rollbackFiles()
		}
	}()
	result, err := action.fn(actionCtx, dygo.EntityActionCall{
		Actor:     actor,
		RecordIDs: append([]int64(nil), recordIDs...),
		Input:     append(json.RawMessage(nil), input...),
		Records:   dygodata.NewRecordData(tx, e.RecordHooks).WithAppScope(meta.App.Name).WithActionActor(actor),
		Jobs:      jobs,
		Files:     actionFiles,
		Timeline:  dygodata.NewTimelineDataAsActor(tx, actor),
		Notifications: dygodata.NewNotificationData(
			dygodata.NewRecordData(tx, e.RecordHooks).AsSystem("notification-send"),
			jobs,
		),
	})
	if err != nil {
		return nil, err
	}
	for _, recordID := range recordIDs {
		if err := db.WriteActionActivity(actionCtx, tx, meta, recordID, name, action.definition.Label); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		keepFiles = true
		return nil, dygo.ActionError{Code: "internal_error", Message: "Entity action transaction could not commit"}
	}
	keepFiles = true
	return result, nil
}

type transactionalFileData interface {
	WithQueryer(db.RecordQueryer) dygo.FileData
}

type rollbackFileData interface {
	Rollback(context.Context)
}

func actorFiles(data dygo.FileData, queryer db.RecordQueryer, actor dygo.Actor) (dygo.FileData, func()) {
	if data == nil {
		return nil, func() {}
	}
	if transactional, ok := data.(transactionalFileData); ok {
		data = transactional.WithQueryer(queryer)
	}
	data = data.AsActor(actor)
	return data, func() {
		if rollback, ok := data.(rollbackFileData); ok {
			rollback.Rollback(context.Background())
		}
	}
}

func validateRecordIDs(selection dygo.ActionSelection, recordIDs []int64) error {
	for _, id := range recordIDs {
		if id <= 0 {
			return dygo.ActionError{Code: "invalid_request", Message: "record IDs must be positive integers"}
		}
	}
	switch selection {
	case dygo.ActionSelectionRecord:
		if len(recordIDs) != 1 {
			return dygo.ActionError{Code: "invalid_request", Message: "Entity action requires one record"}
		}
	case dygo.ActionSelectionSelection:
		if len(recordIDs) == 0 {
			return dygo.ActionError{Code: "invalid_request", Message: "Entity action requires selected records"}
		}
	case dygo.ActionSelectionCollection:
		if len(recordIDs) != 0 {
			return dygo.ActionError{Code: "invalid_request", Message: "collection Entity action does not accept record IDs"}
		}
	}
	return nil
}

var _ jobstore.Beginner = (pgx.Tx)(nil)

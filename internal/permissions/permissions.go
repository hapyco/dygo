// Package permissions resolves dygo Core permission records.
package permissions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hapyco/dygo/pkg/dygo"
	"github.com/jackc/pgx/v5"
)

const (
	// ActionRead allows reading a resource.
	ActionRead = dygo.ActionRead
	// ActionCreate allows creating a resource.
	ActionCreate = dygo.ActionCreate
	// ActionUpdate allows updating a resource.
	ActionUpdate = dygo.ActionUpdate
	// ActionDelete allows deleting a resource.
	ActionDelete = dygo.ActionDelete
	// ActionExport allows exporting a resource.
	ActionExport = dygo.ActionExport
	// ActionPrint allows printing a resource.
	ActionPrint = dygo.ActionPrint
)

const (
	// ErrorInvalidRequest reports a malformed permission check request.
	ErrorInvalidRequest = "invalid_request"
	// ErrorDenied reports that a valid request is not allowed.
	ErrorDenied = "permission_denied"
	// ErrorInternal reports a permission engine failure.
	ErrorInternal = "internal_error"
)

const (
	// ReasonAllowed means the user has the requested permission.
	ReasonAllowed = "allowed"
	// ReasonDenied means the user does not have the requested permission.
	ReasonDenied = "denied"
)

// Action is a supported permission action.
type Action = dygo.Action

// Actor identifies the user asking a permission question.
type Actor = dygo.Actor

// ResourceKind identifies the namespace of a permission target.
type ResourceKind = dygo.ResourceKind

const (
	ResourceEntity = dygo.ResourceEntity
	ResourcePage   = dygo.ResourcePage
)

// Resource identifies an app-scoped Entity or Page permission target.
type Resource = dygo.Resource

// ResourceRequest is the resource-oriented permission contract used by new
// callers. Request remains available as the legacy Entity-compatible shape.
type ResourceRequest = dygo.PermissionRequest

// Request identifies the permission question being asked.
type Request struct {
	Actor    Actor
	Entity   string
	Resource Resource
	Action   Action
}

// Decision is the result of a permission check.
type Decision struct {
	Allowed  bool
	Actor    Actor
	Entity   string
	Resource Resource
	Action   Action
	Reason   string
}

// RoleDecision is the result of checking one role grant.
type RoleDecision struct {
	Allowed bool
	Role    string
	Entity  string
	Action  Action
	Reason  string
}

// Error reports stable permission engine failures.
type Error struct {
	Code    string
	Message string
	Details map[string]any
	Err     error
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) Unwrap() error {
	return e.Err
}

// Is reports whether err has the same stable permission error code as target.
func (e Error) Is(target error) bool {
	targetErr, ok := target.(Error)
	return ok && e.Code == targetErr.Code
}

// Queryer is the database behavior needed by the permission checker.
type Queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// Checker evaluates flat Core RBAC permissions.
type Checker struct {
	queryer Queryer
}

// NewChecker returns a permission checker backed by queryer.
func NewChecker(queryer Queryer) Checker {
	return Checker{queryer: queryer}
}

// Check evaluates whether a user has an Entity permission action.
func (c Checker) Check(ctx context.Context, request Request) (Decision, error) {
	normalized, resource, err := normalizeRequest(request)
	if err != nil {
		return Decision{}, err
	}
	if normalized.Actor.Administrator {
		return allowedDecision(normalized, resource), nil
	}
	if c.queryer == nil {
		return Decision{}, permissionError(ErrorInternal, "permission queryer is required", nil, nil)
	}

	targetSQL, targetArgs := permissionTargetSQL(resource)
	args := append([]any{normalized.Actor.UserID}, targetArgs...)
	actionSQL, args, err := permissionActionSQL(normalized.Action, resource, args)
	if err != nil {
		return Decision{}, err
	}

	sql := fmt.Sprintf(`
SELECT EXISTS (
	SELECT 1
	FROM "user" u
	JOIN user_role ur ON ur.user_id = u.id
	JOIN "role" r ON r.id = ur.role_id AND COALESCE(r.enabled, false) = true
	JOIN "permission" p ON p.role_id = r.id
	LEFT JOIN entity e ON e.id = p.entity_id AND e.retired = false
	LEFT JOIN app a ON a.id = e.app_id
	LEFT JOIN page pg ON pg.id = p.page_id
	LEFT JOIN app pa ON pa.id = pg.app_id
	WHERE u.id = $1
		AND COALESCE(u.enabled, false) = true
		AND %s
		AND COALESCE(p.retired, false) = false
		AND %s
	LIMIT 1
)`, targetSQL, actionSQL)

	var allowed bool
	if err := c.queryer.QueryRow(ctx, sql, args...).Scan(&allowed); err != nil {
		return Decision{}, permissionError(ErrorInternal, "permission check failed", decisionDetails(normalized), err)
	}
	if allowed {
		return allowedDecision(normalized, resource), nil
	}
	decision := allowedDecision(normalized, resource)
	decision.Allowed = false
	decision.Reason = ReasonDenied
	return decision, nil
}

// CheckResource evaluates a permission against an app-scoped resource.
func (c Checker) CheckResource(ctx context.Context, request ResourceRequest) (Decision, error) {
	return c.Check(ctx, Request{Actor: request.Actor, Resource: request.Resource, Action: request.Action})
}

// Can returns nil only when the requested permission is allowed.
func (c Checker) Can(ctx context.Context, request Request) error {
	decision, err := c.Check(ctx, request)
	if err != nil {
		return err
	}
	if decision.Allowed {
		return nil
	}
	return permissionError(ErrorDenied, "permission denied", decisionDetails(Request{
		Actor:    decision.Actor,
		Entity:   decision.Entity,
		Resource: decision.Resource,
		Action:   decision.Action,
	}), nil)
}

// CanResource returns nil only when the resource permission is allowed.
func (c Checker) CanResource(ctx context.Context, request ResourceRequest) error {
	return c.Can(ctx, Request{Actor: request.Actor, Resource: request.Resource, Action: request.Action})
}

// Authorize implements the public dygo.Authorizer contract.
func (c Checker) Authorize(ctx context.Context, request dygo.PermissionRequest) error {
	return c.CanResource(ctx, request)
}

// IsDenied reports whether err is a denied permission error.
func IsDenied(err error) bool {
	var permissionErr Error
	return errors.As(err, &permissionErr) && permissionErr.Code == ErrorDenied
}

func normalizeRequest(request Request) (Request, Resource, error) {
	entity := strings.TrimSpace(request.Entity)
	resource := request.Resource
	if resource.Kind == "" && resource.App == "" && resource.Name == "" && entity != "" {
		resource = Resource{Kind: ResourceEntity, Name: entity}
	}
	normalized := Request{
		Actor:    request.Actor,
		Entity:   entity,
		Resource: Resource{Kind: ResourceKind(strings.TrimSpace(string(resource.Kind))), App: strings.TrimSpace(resource.App), Name: strings.TrimSpace(resource.Name)},
		Action:   Action(strings.TrimSpace(string(request.Action))),
	}
	if normalized.Actor.UserID <= 0 {
		return Request{}, Resource{}, permissionError(ErrorInvalidRequest, "user id must be a positive integer", map[string]any{"user-id": request.Actor.UserID}, nil)
	}
	if normalized.Entity != "" && request.Resource != (Resource{}) {
		return Request{}, Resource{}, permissionError(ErrorInvalidRequest, "entity and resource must not both be provided", nil, nil)
	}
	if normalized.Resource.Kind != ResourceEntity && normalized.Resource.Kind != ResourcePage {
		return Request{}, Resource{}, permissionError(ErrorInvalidRequest, "resource kind must be entity or page", map[string]any{"resource-kind": normalized.Resource.Kind}, nil)
	}
	if normalized.Resource.Name == "" {
		return Request{}, Resource{}, permissionError(ErrorInvalidRequest, "resource name is required", nil, nil)
	}
	if normalized.Resource.Kind == ResourcePage && normalized.Resource.App == "" {
		return Request{}, Resource{}, permissionError(ErrorInvalidRequest, "page resource app is required", nil, nil)
	}
	if _, err := ParseAction(string(normalized.Action)); err != nil {
		return Request{}, Resource{}, permissionError(ErrorInvalidRequest, err.Error(), map[string]any{"action": request.Action}, err)
	}
	if normalized.Resource.Kind == ResourceEntity {
		normalized.Entity = normalized.Resource.Name
	}
	return normalized, normalized.Resource, nil
}

func allowedDecision(request Request, resource Resource) Decision {
	return Decision{Allowed: true, Actor: request.Actor, Entity: request.Entity, Resource: resource, Action: request.Action, Reason: ReasonAllowed}
}

func decisionDetails(request Request) map[string]any {
	details := map[string]any{
		"user-id": request.Actor.UserID,
		"action":  request.Action,
	}
	if request.Resource.Kind != "" {
		details["resource-kind"] = request.Resource.Kind
		details["resource-app"] = request.Resource.App
		details["resource-name"] = request.Resource.Name
	} else {
		details["entity"] = request.Entity
	}
	return details
}

func permissionTargetSQL(resource Resource) (string, []any) {
	switch {
	case resource.Kind == ResourcePage:
		return "pa.name = $2 AND pg.key = $3 AND p.page_id IS NOT NULL AND COALESCE(pg.retired, false) = false", []any{resource.App, resource.Name}
	case resource.App != "":
		return "a.name = $2 AND e.key = $3", []any{resource.App, resource.Name}
	default:
		return "e.slug = $2", []any{resource.Name}
	}
}

func permissionActionSQL(action Action, resource Resource, args []any) (string, []any, error) {
	if column, ok := actionColumn(action); ok {
		return fmt.Sprintf("COALESCE(p.%s, false) = true", column), args, nil
	}
	if resource.Kind != ResourceEntity {
		return "", nil, permissionError(ErrorInvalidRequest, "custom actions require an Entity resource", nil, nil)
	}
	args = append(args, string(action))
	return fmt.Sprintf("COALESCE(p.actions, '[]'::jsonb) ? $%d", len(args)), args, nil
}

func permissionError(code string, message string, details map[string]any, err error) Error {
	return Error{
		Code:    code,
		Message: message,
		Details: details,
		Err:     err,
	}
}

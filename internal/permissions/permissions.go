// Package permissions resolves dygo Core permission records.
package permissions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	// ActionRead allows reading Records for an Entity.
	ActionRead Action = "read"
	// ActionCreate allows creating Records for an Entity.
	ActionCreate Action = "create"
	// ActionUpdate allows updating Records for an Entity.
	ActionUpdate Action = "update"
	// ActionDelete allows deleting Records for an Entity.
	ActionDelete Action = "delete"
	// ActionExport allows exporting Records for an Entity.
	ActionExport Action = "export"
	// ActionPrint allows printing Records for an Entity.
	ActionPrint Action = "print"
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
type Action string

// Actor identifies the user asking a permission question.
type Actor struct {
	UserID        int64
	Administrator bool
}

// Request identifies the permission question being asked.
type Request struct {
	Actor    Actor
	Entity   string
	Action   Action
	RecordID int64
}

// Decision is the result of a permission check.
type Decision struct {
	Allowed  bool
	Actor    Actor
	Entity   string
	Action   Action
	RecordID int64
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
	normalized, column, err := normalizeRequest(request)
	if err != nil {
		return Decision{}, err
	}
	if normalized.Actor.Administrator {
		return Decision{
			Allowed:  true,
			Actor:    normalized.Actor,
			Entity:   normalized.Entity,
			Action:   normalized.Action,
			RecordID: normalized.RecordID,
			Reason:   ReasonAllowed,
		}, nil
	}
	if c.queryer == nil {
		return Decision{}, permissionError(ErrorInternal, "permission queryer is required", nil, nil)
	}

	sql := fmt.Sprintf(`
SELECT EXISTS (
	SELECT 1
	FROM "user" u
	JOIN user_role ur ON ur.user_id = u.id
	JOIN "role" r ON r.id = ur.role_id AND COALESCE(r.enabled, false) = true
	JOIN "permission" p ON p.role_id = r.id
	JOIN entity e ON e.id = p.entity_id
	WHERE u.id = $1
		AND COALESCE(u.enabled, false) = true
		AND e.slug = $2
		AND COALESCE(p.retired, false) = false
		AND COALESCE(p.%s, false) = true
	LIMIT 1
)`, column)

	var allowed bool
	if err := c.queryer.QueryRow(ctx, sql, normalized.Actor.UserID, normalized.Entity).Scan(&allowed); err != nil {
		return Decision{}, permissionError(ErrorInternal, "permission check failed", decisionDetails(normalized), err)
	}
	if allowed {
		return Decision{
			Allowed:  true,
			Actor:    normalized.Actor,
			Entity:   normalized.Entity,
			Action:   normalized.Action,
			RecordID: normalized.RecordID,
			Reason:   ReasonAllowed,
		}, nil
	}
	return Decision{
		Allowed:  false,
		Actor:    normalized.Actor,
		Entity:   normalized.Entity,
		Action:   normalized.Action,
		RecordID: normalized.RecordID,
		Reason:   ReasonDenied,
	}, nil
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
		Action:   decision.Action,
		RecordID: decision.RecordID,
	}), nil)
}

// CheckRole evaluates whether a role grants an Entity permission action.
func CheckRole(ctx context.Context, queryer Queryer, role string, entity string, action Action) (RoleDecision, error) {
	role = strings.TrimSpace(role)
	entity = strings.TrimSpace(entity)
	action = Action(strings.TrimSpace(string(action)))
	if role == "" {
		return RoleDecision{}, permissionError(ErrorInvalidRequest, "role is required", map[string]any{"role": role}, nil)
	}
	if entity == "" {
		return RoleDecision{}, permissionError(ErrorInvalidRequest, "entity is required", map[string]any{"entity": entity}, nil)
	}
	column, ok := actionColumn(action)
	if !ok {
		return RoleDecision{}, permissionError(ErrorInvalidRequest, "permission action is not supported", map[string]any{"action": action}, nil)
	}
	if queryer == nil {
		return RoleDecision{}, permissionError(ErrorInternal, "permission queryer is required", nil, nil)
	}

	sql := fmt.Sprintf(`
SELECT EXISTS (
	SELECT 1
	FROM "permission" p
	JOIN "role" r ON r.id = p.role_id AND COALESCE(r.enabled, false) = true
	JOIN entity e ON e.id = p.entity_id
	WHERE r.name = $1
		AND e.slug = $2
		AND COALESCE(p.retired, false) = false
		AND COALESCE(p.%s, false) = true
	LIMIT 1
)`, column)

	var allowed bool
	if err := queryer.QueryRow(ctx, sql, role, entity).Scan(&allowed); err != nil {
		return RoleDecision{}, permissionError(ErrorInternal, "permission check failed", map[string]any{"role": role, "entity": entity, "action": action}, err)
	}
	if allowed {
		return RoleDecision{Allowed: true, Role: role, Entity: entity, Action: action, Reason: ReasonAllowed}, nil
	}
	return RoleDecision{Allowed: false, Role: role, Entity: entity, Action: action, Reason: ReasonDenied}, nil
}

// IsError reports whether err is a permission Error.
func IsError(err error) bool {
	var permissionErr Error
	return errors.As(err, &permissionErr)
}

// IsDenied reports whether err is a denied permission error.
func IsDenied(err error) bool {
	var permissionErr Error
	return errors.As(err, &permissionErr) && permissionErr.Code == ErrorDenied
}

func normalizeRequest(request Request) (Request, string, error) {
	normalized := Request{
		Actor:    request.Actor,
		Entity:   strings.TrimSpace(request.Entity),
		Action:   Action(strings.TrimSpace(string(request.Action))),
		RecordID: request.RecordID,
	}
	if normalized.Actor.UserID <= 0 {
		return Request{}, "", permissionError(ErrorInvalidRequest, "user id must be a positive integer", map[string]any{"user-id": request.Actor.UserID}, nil)
	}
	if normalized.Entity == "" {
		return Request{}, "", permissionError(ErrorInvalidRequest, "entity is required", map[string]any{"entity": request.Entity}, nil)
	}
	if normalized.RecordID < 0 {
		return Request{}, "", permissionError(ErrorInvalidRequest, "record id must be greater than or equal to zero", map[string]any{"record-id": request.RecordID}, nil)
	}
	column, ok := actionColumn(normalized.Action)
	if !ok {
		return Request{}, "", permissionError(ErrorInvalidRequest, "permission action is not supported", map[string]any{"action": request.Action}, nil)
	}
	return normalized, column, nil
}

func decisionDetails(request Request) map[string]any {
	details := map[string]any{
		"user-id": request.Actor.UserID,
		"entity":  request.Entity,
		"action":  request.Action,
	}
	if request.RecordID > 0 {
		details["record-id"] = request.RecordID
	}
	return details
}

func permissionError(code string, message string, details map[string]any, err error) Error {
	return Error{
		Code:    code,
		Message: message,
		Details: details,
		Err:     err,
	}
}

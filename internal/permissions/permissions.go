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

// Request identifies the permission question being asked.
type Request struct {
	UserID int64
	Entity string
	Action Action
}

// Decision is the result of a permission check.
type Decision struct {
	Allowed bool
	UserID  int64
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
	if c.queryer == nil {
		return Decision{}, permissionError(ErrorInternal, "permission queryer is required", nil, nil)
	}
	normalized, column, err := normalizeRequest(request)
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
	JOIN entity e ON e.id = p.entity_id
	WHERE u.id = $1
		AND COALESCE(u.enabled, false) = true
		AND e.name = $2
		AND COALESCE(p.%s, false) = true
	LIMIT 1
)`, column)

	var allowed bool
	if err := c.queryer.QueryRow(ctx, sql, normalized.UserID, normalized.Entity).Scan(&allowed); err != nil {
		return Decision{}, permissionError(ErrorInternal, "permission check failed", decisionDetails(normalized), err)
	}
	if allowed {
		return Decision{
			Allowed: true,
			UserID:  normalized.UserID,
			Entity:  normalized.Entity,
			Action:  normalized.Action,
			Reason:  ReasonAllowed,
		}, nil
	}
	return Decision{
		Allowed: false,
		UserID:  normalized.UserID,
		Entity:  normalized.Entity,
		Action:  normalized.Action,
		Reason:  ReasonDenied,
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
		UserID: decision.UserID,
		Entity: decision.Entity,
		Action: decision.Action,
	}), nil)
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
		UserID: request.UserID,
		Entity: strings.TrimSpace(request.Entity),
		Action: Action(strings.TrimSpace(string(request.Action))),
	}
	if normalized.UserID <= 0 {
		return Request{}, "", permissionError(ErrorInvalidRequest, "user id must be a positive integer", map[string]any{"user-id": request.UserID}, nil)
	}
	if normalized.Entity == "" {
		return Request{}, "", permissionError(ErrorInvalidRequest, "entity is required", map[string]any{"entity": request.Entity}, nil)
	}
	column, ok := actionColumn(normalized.Action)
	if !ok {
		return Request{}, "", permissionError(ErrorInvalidRequest, "permission action is not supported", map[string]any{"action": request.Action}, nil)
	}
	return normalized, column, nil
}

func actionColumn(action Action) (string, bool) {
	switch action {
	case ActionRead:
		return `"read"`, true
	case ActionCreate:
		return `"create"`, true
	case ActionUpdate:
		return `"update"`, true
	case ActionDelete:
		return `"delete"`, true
	case ActionExport:
		return `"export"`, true
	case ActionPrint:
		return `"print"`, true
	default:
		return "", false
	}
}

func decisionDetails(request Request) map[string]any {
	return map[string]any{
		"user-id": request.UserID,
		"entity":  request.Entity,
		"action":  request.Action,
	}
}

func permissionError(code string, message string, details map[string]any, err error) Error {
	return Error{
		Code:    code,
		Message: message,
		Details: details,
		Err:     err,
	}
}

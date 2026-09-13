package dygo

import "context"

// Action is a permission action understood by the framework.
type Action string

const (
	ActionRead   Action = "read"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionExport Action = "export"
	ActionPrint  Action = "print"
)

// Actor identifies the user asking for authorization.
type Actor struct {
	UserID        int64
	Email         string
	Administrator bool
}

// ResourceKind identifies the namespace of a permission target.
type ResourceKind string

const (
	ResourceEntity ResourceKind = "entity"
	ResourcePage   ResourceKind = "page"
)

// Resource identifies an app-scoped Entity or Page.
type Resource struct {
	Kind ResourceKind
	App  string
	Name string
}

// PermissionRequest asks whether an Actor may perform an Action on a Resource.
type PermissionRequest struct {
	Actor    Actor
	Resource Resource
	Action   Action
}

// Authorizer is the stable authorization contract for framework and app code.
type Authorizer interface {
	Authorize(context.Context, PermissionRequest) error
}

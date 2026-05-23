// Package corevalues names Core select values that runtime services use.
package corevalues

const (
	AppStatusInstalled      = "installed"
	AppStatusActive         = "active"
	AppStatusDisabled       = "disabled"
	AppStatusPendingInstall = "pending-install"
	AppStatusPendingUpgrade = "pending-upgrade"
	AppStatusFailed         = "failed"
)

// AppStatuses returns Core app status values in metadata order.
func AppStatuses() []string {
	return []string{
		AppStatusInstalled,
		AppStatusActive,
		AppStatusDisabled,
		AppStatusPendingInstall,
		AppStatusPendingUpgrade,
		AppStatusFailed,
	}
}

const (
	SessionStatusActive  = "active"
	SessionStatusExpired = "expired"
	SessionStatusRevoked = "revoked"
)

// SessionStatuses returns Core session status values in metadata order.
func SessionStatuses() []string {
	return []string{
		SessionStatusActive,
		SessionStatusExpired,
		SessionStatusRevoked,
	}
}

const (
	ActivityKindRecord     = "record"
	ActivityKindComment    = "comment"
	ActivityKindWorkflow   = "workflow"
	ActivityKindJob        = "job"
	ActivityKindEmail      = "email"
	ActivityKindAttachment = "attachment"
	ActivityKindAuth       = "auth"
	ActivityKindSystem     = "system"
)

// ActivityKinds returns Core activity kind values in metadata order.
func ActivityKinds() []string {
	return []string{
		ActivityKindRecord,
		ActivityKindComment,
		ActivityKindWorkflow,
		ActivityKindJob,
		ActivityKindEmail,
		ActivityKindAttachment,
		ActivityKindAuth,
		ActivityKindSystem,
	}
}

const (
	ActivityOperationCreate             = "create"
	ActivityOperationUpdate             = "update"
	ActivityOperationDelete             = "delete"
	ActivityOperationRestore            = "restore"
	ActivityOperationComment            = "comment"
	ActivityOperationWorkflowTransition = "workflow-transition"
	ActivityOperationJobCompleted       = "job-completed"
	ActivityOperationEmailSent          = "email-sent"
	ActivityOperationAttachmentAdded    = "attachment-added"
	ActivityOperationLogin              = "login"
	ActivityOperationLogout             = "logout"
	ActivityOperationSystem             = "system"
)

// ActivityOperations returns Core activity operation values in metadata order.
func ActivityOperations() []string {
	return []string{
		ActivityOperationCreate,
		ActivityOperationUpdate,
		ActivityOperationDelete,
		ActivityOperationRestore,
		ActivityOperationComment,
		ActivityOperationWorkflowTransition,
		ActivityOperationJobCompleted,
		ActivityOperationEmailSent,
		ActivityOperationAttachmentAdded,
		ActivityOperationLogin,
		ActivityOperationLogout,
		ActivityOperationSystem,
	}
}

const (
	ActivityStatusSuccess = "success"
	ActivityStatusFailed  = "failed"
)

// ActivityStatuses returns Core activity status values in metadata order.
func ActivityStatuses() []string {
	return []string{
		ActivityStatusSuccess,
		ActivityStatusFailed,
	}
}

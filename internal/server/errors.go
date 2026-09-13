package server

import (
	"errors"
	"net/http"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/pkg/dygo"
)

// writeActionError maps framework action errors to the shared API envelope.
func writeActionError(w http.ResponseWriter, err error, fallbackMessage string) {
	var actionErr dygo.ActionError
	if errors.As(err, &actionErr) {
		status := http.StatusInternalServerError
		switch actionErr.Code {
		case "invalid_request":
			status = http.StatusBadRequest
		case "validation_error":
			status = http.StatusUnprocessableEntity
		case "not_found":
			status = http.StatusNotFound
		case "constraint_violation", "conflict":
			status = http.StatusConflict
		case "permission_denied":
			status = http.StatusForbidden
		case "internal_error":
			actionErr.Message = fallbackMessage
			actionErr.Details = nil
		}
		writeErrorEnvelope(w, status, actionErr.Code, actionErr.Message, actionErr.Details)
		return
	}
	var permissionErr permissions.Error
	if errors.As(err, &permissionErr) {
		writePermissionError(w, err)
		return
	}
	var recordErr db.RecordError
	if errors.As(err, &recordErr) {
		writeRecordError(w, err)
		return
	}
	if db.IsMetadataNotFound(err) {
		writeErrorEnvelope(w, http.StatusNotFound, "not_found", "Entity not found", nil)
		return
	}
	writeErrorEnvelope(w, http.StatusInternalServerError, "internal_error", fallbackMessage, nil)
}

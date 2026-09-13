package server

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/pkg/dygo"
	"net/http"
)

func (h recordHandler) secretStatus(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	entity := chi.URLParam(r, "entity")
	id, err := recordIDParam(entity, chi.URLParam(r, "id"))
	if err != nil {
		writeRecordError(w, err)
		return
	}
	if !h.authorize(w, r, entity, permissions.ActionRead) {
		return
	}
	store, err := h.storeFor(r, entity, permissions.ActionRead)
	if err != nil {
		writePermissionError(w, err)
		return
	}
	statusStore, ok := store.(interface {
		SecretStatus(context.Context, string, int64) (dygo.SecretStatus, error)
	})
	if !ok {
		writeErrorEnvelope(w, http.StatusServiceUnavailable, "unavailable", "secret status is unavailable", nil)
		return
	}
	status, err := statusStore.SecretStatus(r.Context(), entity, id)
	if err != nil {
		writeRecordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: status})
}

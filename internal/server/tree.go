package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/internal/recordquery"
	"github.com/hapyco/dygo/pkg/dygo"
)

func (h recordHandler) treeRecords(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	entity := chi.URLParam(r, "entity")
	if !h.authorize(w, r, entity, permissions.ActionRead) {
		return
	}
	meta, ok := recordEntityMeta(r.Context())
	if !ok || meta.Tree == nil {
		writeRecordError(w, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: "Entity does not support trees"})
		return
	}
	scoped, err := h.storeFor(r, entity, permissions.ActionRead)
	if err != nil {
		writePermissionError(w, err)
		return
	}
	store, ok := scoped.(db.RecordStore)
	if !ok {
		writeRecordError(w, db.RecordError{Code: db.RecordErrorInternal, Message: "tree store unavailable"})
		return
	}
	values := r.URL.Query()
	operation := chi.URLParam(r, "operation")
	var anchor, exclude int64
	for _, key := range []string{"name", "exclude-subtree"} {
		if len(values[key]) > 1 {
			writeRecordError(w, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: key + " must be provided once"})
			return
		}
		if name := values.Get(key); name != "" {
			id, err := store.ResolveTreeName(r.Context(), meta.App.Name, meta.Key, name)
			if err != nil {
				writeRecordError(w, err)
				return
			}
			if key == "name" {
				anchor = id
			} else {
				exclude = id
			}
		}
		values.Del(key)
	}
	if operation != "roots" && operation != "search" && anchor == 0 {
		writeRecordError(w, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: "name is required"})
		return
	}
	params, err := recordquery.FromValues(values)
	if err != nil {
		writeRecordError(w, recordQueryHTTPError(err))
		return
	}
	result, err := store.TreeRecords(r.Context(), meta.App.Name, meta.Key, operation, anchor, params, exclude)
	if err != nil {
		writeRecordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Data    []dygo.TreeNode `json:"data"`
		Context []dygo.Record   `json:"context"`
		Meta    recordListMeta  `json:"meta"`
	}{result.Nodes, result.Context, recordListMeta{Count: result.Count, Limit: result.Limit, Offset: result.Offset}})
}

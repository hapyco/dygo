package server

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/hapyco/dygo/internal/auth"
	importsvc "github.com/hapyco/dygo/internal/imports"
	"github.com/hapyco/dygo/pkg/dygo"
)

// ImportStore starts durable CSV imports.
type ImportStore interface {
	Start(context.Context, dygo.Actor, importsvc.Target, io.Reader) (importsvc.Info, error)
	Status(context.Context, dygo.Actor, int64) (importsvc.Info, error)
}

type importHandler struct{ imports ImportStore }

func registerImportRoutes(router chi.Router, imports ImportStore) {
	router.Post("/imports", (importHandler{imports: imports}).start)
	router.Get("/imports/{id}", (importHandler{imports: imports}).status)
}

func (h importHandler) status(w http.ResponseWriter, r *http.Request) {
	if h.imports == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{Code: "service_unavailable", Message: "import service is unavailable"}})
		return
	}
	user, ok := CurrentUserFromContext(r.Context())
	if !ok {
		writeAuthError(w, auth.Error{Code: auth.ErrorUnauthenticated, Message: "authentication required"})
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeImportError(w, dygo.ActionError{Code: "invalid_request", Message: "import id must be positive"})
		return
	}
	info, err := h.imports.Status(r.Context(), dygo.Actor{UserID: user.ID, Email: user.Email, Administrator: user.Administrator}, id)
	if err != nil {
		writeImportError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: info})
}

func (h importHandler) start(w http.ResponseWriter, r *http.Request) {
	if h.imports == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{Code: "service_unavailable", Message: "import service is unavailable"}})
		return
	}
	user, ok := CurrentUserFromContext(r.Context())
	if !ok {
		writeAuthError(w, auth.Error{Code: auth.ErrorUnauthenticated, Message: "authentication required"})
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeImportError(w, dygo.ActionError{Code: "invalid_request", Message: "multipart CSV body is required"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeImportError(w, dygo.ActionError{Code: "invalid_request", Message: "multipart CSV file field is required"})
		return
	}
	defer file.Close()
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") && !strings.Contains(strings.ToLower(header.Header.Get("Content-Type")), "csv") {
		writeImportError(w, dygo.ActionError{Code: "invalid_request", Message: "only CSV files are supported"})
		return
	}
	info, err := h.imports.Start(r.Context(), dygo.Actor{UserID: user.ID, Email: user.Email, Administrator: user.Administrator}, importsvc.Target{App: r.FormValue("app"), Entity: r.FormValue("entity")}, file)
	if err != nil {
		writeImportError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, dataEnvelope{Data: info})
}

func writeImportError(w http.ResponseWriter, err error) {
	writeActionError(w, err, "import failed")
}

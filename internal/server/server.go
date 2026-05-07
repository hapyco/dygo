package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/dygo-dev/dygo/internal/db"
	"github.com/go-chi/chi/v5"
)

const shutdownTimeout = 5 * time.Second

// Options configures the dygo HTTP server.
type Options struct {
	Address     string
	DatabaseURL string
	Metadata    MetadataStore
	OnReady     func(string) error
}

// MetadataStore is the runtime metadata behavior used by HTTP handlers.
type MetadataStore interface {
	ListApps(context.Context) ([]db.MetadataApp, error)
	GetApp(context.Context, string) (db.MetadataApp, error)
	ListEntities(context.Context) ([]db.MetadataEntity, error)
	GetEntityMeta(context.Context, string) (db.MetadataEntityMeta, error)
}

// NewRouter creates the dygo HTTP router.
func NewRouter(options ...Options) http.Handler {
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	router := chi.NewRouter()
	router.Get("/health", healthHandler)
	registerMetadataRoutes(router, opts.Metadata)
	return router
}

// Serve starts the dygo HTTP server on address and blocks until it exits.
func Serve(ctx context.Context, options Options) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if options.Address == "" {
		return fmt.Errorf("server address is required")
	}

	if options.Metadata == nil {
		if options.DatabaseURL == "" {
			return fmt.Errorf("database url is required")
		}
		pool, err := db.OpenRuntimePool(ctx, options.DatabaseURL)
		if err != nil {
			return fmt.Errorf("open runtime database: %w", err)
		}
		defer pool.Close()
		options.Metadata = db.NewMetadataReader(pool)
	}

	listener, err := net.Listen("tcp", options.Address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", options.Address, err)
	}
	if options.OnReady != nil {
		if err := options.OnReady(options.Address); err != nil {
			_ = listener.Close()
			return fmt.Errorf("notify server ready: %w", err)
		}
		options.OnReady = nil
	}

	return ServeListener(ctx, listener, options)
}

// ServeListener starts the dygo HTTP server on an existing listener.
func ServeListener(ctx context.Context, listener net.Listener, options ...Options) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if listener == nil {
		return fmt.Errorf("listener is required")
	}
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}

	httpServer := &http.Server{
		Handler: NewRouter(opts),
	}

	done := make(chan error, 1)
	go func() {
		err := httpServer.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			done <- nil
			return
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("serve http: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
			return fmt.Errorf("shutdown http server: %w", err)
		}
		if err := <-done; err != nil {
			return fmt.Errorf("serve http: %w", err)
		}
		return nil
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

type dataEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type metadataHandler struct {
	store MetadataStore
}

func registerMetadataRoutes(router chi.Router, store MetadataStore) {
	handler := metadataHandler{store: store}
	router.Route("/api/v1", func(api chi.Router) {
		api.Get("/apps", handler.listApps)
		api.Get("/apps/{app}", handler.getApp)
		api.Get("/entities", handler.listEntities)
		api.Get("/entities/{entity}/meta", handler.getEntityMeta)
	})
}

func (h metadataHandler) listApps(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	apps, err := h.store.ListApps(r.Context())
	if err != nil {
		writeAPIError(w, err, "", "")
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: apps})
}

func (h metadataHandler) getApp(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	name := chi.URLParam(r, "app")
	app, err := h.store.GetApp(r.Context(), name)
	if err != nil {
		writeAPIError(w, err, "app", name)
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: app})
}

func (h metadataHandler) listEntities(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	entities, err := h.store.ListEntities(r.Context())
	if err != nil {
		writeAPIError(w, err, "", "")
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: entities})
}

func (h metadataHandler) getEntityMeta(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	name := chi.URLParam(r, "entity")
	meta, err := h.store.GetEntityMeta(r.Context(), name)
	if err != nil {
		writeAPIError(w, err, "entity", name)
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: meta})
}

func (h metadataHandler) requireStore(w http.ResponseWriter) bool {
	if h.store != nil {
		return false
	}
	writeJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{
		Code:    "service_unavailable",
		Message: "metadata store is unavailable",
	}})
	return true
}

func writeAPIError(w http.ResponseWriter, err error, detailKey string, detailValue string) {
	if db.IsMetadataNotFound(err) {
		details := map[string]any{}
		if detailKey != "" {
			details[detailKey] = detailValue
		}
		writeJSON(w, http.StatusNotFound, errorEnvelope{Error: apiError{
			Code:    "not_found",
			Message: detailKey + " not found",
			Details: details,
		}})
		return
	}
	writeJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{
		Code:    "internal_error",
		Message: "metadata query failed",
	}})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

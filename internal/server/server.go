package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dygo-dev/dygo/internal/auth"
	"github.com/dygo-dev/dygo/internal/db"
	"github.com/go-chi/chi/v5"
)

const shutdownTimeout = 5 * time.Second
const sessionCookieName = "dygo_session"

// Options configures the dygo HTTP server.
type Options struct {
	Address     string
	DatabaseURL string
	Auth        AuthStore
	Metadata    MetadataStore
	Records     RecordStore
	OnReady     func(string) error
}

// AuthStore is the runtime auth behavior used by HTTP handlers.
type AuthStore interface {
	Login(context.Context, auth.LoginRequest) (auth.LoginResult, error)
	CurrentUser(context.Context, string) (auth.User, error)
	Logout(context.Context, string) error
}

// MetadataStore is the runtime metadata behavior used by HTTP handlers.
type MetadataStore interface {
	ListApps(context.Context) ([]db.MetadataApp, error)
	GetApp(context.Context, string) (db.MetadataApp, error)
	ListEntities(context.Context) ([]db.MetadataEntity, error)
	GetEntityMeta(context.Context, string) (db.MetadataEntityMeta, error)
}

// RecordStore is the runtime Record behavior used by HTTP handlers.
type RecordStore interface {
	ListRecords(context.Context, string, db.RecordListParams) (db.RecordListResult, error)
	GetRecord(context.Context, string, int64) (db.Record, error)
	CreateRecord(context.Context, string, db.RecordInput) (db.Record, error)
	UpdateRecord(context.Context, string, int64, db.RecordInput) (db.Record, error)
	DeleteRecord(context.Context, string, int64) error
}

// NewRouter creates the dygo HTTP router.
func NewRouter(options ...Options) http.Handler {
	var opts Options
	if len(options) > 0 {
		opts = options[0]
	}
	router := chi.NewRouter()
	router.Get("/health", healthHandler)
	router.Route("/api/v1", func(api chi.Router) {
		registerAuthRoutes(api, opts.Auth)
		api.Group(func(protected chi.Router) {
			protected.Use(authMiddleware(opts.Auth))
			registerMetadataRoutes(protected, opts.Metadata)
			registerRecordRoutes(protected, opts.Records)
		})
	})
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
		options.Auth = auth.NewService(pool)
		options.Metadata = db.NewMetadataReader(pool)
		options.Records = db.NewRecordStore(pool)
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

type authContextKey struct{}
type authTokenContextKey struct{}

type authHandler struct {
	store AuthStore
}

func registerAuthRoutes(router chi.Router, store AuthStore) {
	handler := authHandler{store: store}
	router.Post("/auth/login", handler.login)
	router.Group(func(protected chi.Router) {
		protected.Use(authMiddleware(store))
		protected.Post("/auth/logout", handler.logout)
		protected.Get("/auth/me", handler.me)
	})
}

func authMiddleware(store AuthStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if store == nil {
				writeJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{
					Code:    "service_unavailable",
					Message: "auth store is unavailable",
				}})
				return
			}
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || strings.TrimSpace(cookie.Value) == "" {
				writeAuthError(w, auth.Error{Code: auth.ErrorUnauthenticated, Message: "authentication required"})
				return
			}
			user, err := store.CurrentUser(r.Context(), cookie.Value)
			if err != nil {
				writeAuthError(w, err)
				return
			}
			ctx := context.WithValue(r.Context(), authContextKey{}, user)
			ctx = context.WithValue(ctx, authTokenContextKey{}, cookie.Value)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (h authHandler) login(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	input, err := decodeLoginInput(r)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	result, err := h.store.Login(r.Context(), input)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	http.SetCookie(w, sessionCookie(result.Token, result.ExpiresAt, isHTTPS(r)))
	writeJSON(w, http.StatusOK, dataEnvelope{Data: result.User})
}

func (h authHandler) logout(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	token, ok := sessionTokenFromContext(r.Context())
	if !ok {
		writeAuthError(w, auth.Error{Code: auth.ErrorUnauthenticated, Message: "authentication required"})
		return
	}
	if err := h.store.Logout(r.Context(), token); err != nil {
		writeAuthError(w, err)
		return
	}
	http.SetCookie(w, expiredSessionCookie(isHTTPS(r)))
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]any{"logged-out": true}})
}

func (h authHandler) me(w http.ResponseWriter, r *http.Request) {
	user, ok := CurrentUserFromContext(r.Context())
	if !ok {
		writeAuthError(w, auth.Error{Code: auth.ErrorUnauthenticated, Message: "authentication required"})
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: user})
}

func (h authHandler) requireStore(w http.ResponseWriter) bool {
	if h.store != nil {
		return false
	}
	writeJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{
		Code:    "service_unavailable",
		Message: "auth store is unavailable",
	}})
	return true
}

// CurrentUserFromContext returns the authenticated user stored by auth middleware.
func CurrentUserFromContext(ctx context.Context) (auth.User, bool) {
	user, ok := ctx.Value(authContextKey{}).(auth.User)
	return user, ok
}

func sessionTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(authTokenContextKey{}).(string)
	return token, ok
}

func decodeLoginInput(r *http.Request) (auth.LoginRequest, error) {
	var envelope struct {
		Data auth.LoginRequest `json:"data"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		if errors.Is(err, io.EOF) {
			return auth.LoginRequest{}, auth.Error{Code: auth.ErrorInvalidRequest, Message: "request body is required"}
		}
		return auth.LoginRequest{}, auth.Error{Code: auth.ErrorInvalidRequest, Message: "request body must be valid JSON", Err: err}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return auth.LoginRequest{}, auth.Error{Code: auth.ErrorInvalidRequest, Message: "request body must contain one JSON object", Err: err}
	}
	if strings.TrimSpace(envelope.Data.Email) == "" || strings.TrimSpace(envelope.Data.Password) == "" {
		return auth.LoginRequest{}, auth.Error{Code: auth.ErrorInvalidRequest, Message: "email and password are required"}
	}
	return envelope.Data, nil
}

func sessionCookie(token string, expiresAt time.Time, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
}

func expiredSessionCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
}

func isHTTPS(r *http.Request) bool {
	return r.TLS != nil
}

func writeAuthError(w http.ResponseWriter, err error) {
	var authErr auth.Error
	if !errors.As(err, &authErr) {
		writeJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{
			Code:    "internal_error",
			Message: "auth request failed",
		}})
		return
	}
	status := http.StatusInternalServerError
	message := authErr.Message
	details := authErr.Details
	switch authErr.Code {
	case auth.ErrorInvalidRequest:
		status = http.StatusBadRequest
	case auth.ErrorInvalidCredentials, auth.ErrorUnauthenticated:
		status = http.StatusUnauthorized
	case auth.ErrorSchemaNotReady:
		status = http.StatusConflict
	case auth.ErrorAlreadyExists:
		status = http.StatusConflict
	case auth.ErrorInternal:
		status = http.StatusInternalServerError
		message = "auth request failed"
		details = nil
	}
	writeJSON(w, status, errorEnvelope{Error: apiError{
		Code:    authErr.Code,
		Message: message,
		Details: details,
	}})
}

type dataEnvelope struct {
	Data any `json:"data"`
}

type listEnvelope struct {
	Data any `json:"data"`
	Meta any `json:"meta"`
}

type recordListMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Count  int `json:"count"`
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
	router.Get("/apps", handler.listApps)
	router.Get("/apps/{app}", handler.getApp)
	router.Get("/entities", handler.listEntities)
	router.Get("/entities/{entity}/meta", handler.getEntityMeta)
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

type recordHandler struct {
	store RecordStore
}

func registerRecordRoutes(router chi.Router, store RecordStore) {
	handler := recordHandler{store: store}
	router.Route("/records/{entity}", func(records chi.Router) {
		records.Get("/", handler.listRecords)
		records.Post("/", handler.createRecord)
		records.Get("/{id}", handler.getRecord)
		records.Patch("/{id}", handler.updateRecord)
		records.Delete("/{id}", handler.deleteRecord)
	})
}

func (h recordHandler) listRecords(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	params, err := recordListParams(r)
	if err != nil {
		writeRecordError(w, err)
		return
	}
	entity := chi.URLParam(r, "entity")
	result, err := h.store.ListRecords(r.Context(), entity, params)
	if err != nil {
		writeRecordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, listEnvelope{
		Data: result.Records,
		Meta: recordListMeta{Limit: result.Limit, Offset: result.Offset, Count: result.Count},
	})
}

func (h recordHandler) getRecord(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	entity := chi.URLParam(r, "entity")
	id, err := recordIDParam(entity, chi.URLParam(r, "id"))
	if err != nil {
		writeRecordError(w, err)
		return
	}
	record, err := h.store.GetRecord(r.Context(), entity, id)
	if err != nil {
		writeRecordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: record})
}

func (h recordHandler) createRecord(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	entity := chi.URLParam(r, "entity")
	input, err := decodeRecordInput(r)
	if err != nil {
		writeRecordError(w, err)
		return
	}
	record, err := h.store.CreateRecord(r.Context(), entity, input)
	if err != nil {
		writeRecordError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, dataEnvelope{Data: record})
}

func (h recordHandler) updateRecord(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	entity := chi.URLParam(r, "entity")
	id, err := recordIDParam(entity, chi.URLParam(r, "id"))
	if err != nil {
		writeRecordError(w, err)
		return
	}
	input, err := decodeRecordInput(r)
	if err != nil {
		writeRecordError(w, err)
		return
	}
	record, err := h.store.UpdateRecord(r.Context(), entity, id, input)
	if err != nil {
		writeRecordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: record})
}

func (h recordHandler) deleteRecord(w http.ResponseWriter, r *http.Request) {
	if h.requireStore(w) {
		return
	}
	entity := chi.URLParam(r, "entity")
	id, err := recordIDParam(entity, chi.URLParam(r, "id"))
	if err != nil {
		writeRecordError(w, err)
		return
	}
	if err := h.store.DeleteRecord(r.Context(), entity, id); err != nil {
		writeRecordError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dataEnvelope{Data: map[string]any{"deleted": true}})
}

func (h recordHandler) requireStore(w http.ResponseWriter) bool {
	if h.store != nil {
		return false
	}
	writeJSON(w, http.StatusServiceUnavailable, errorEnvelope{Error: apiError{
		Code:    "service_unavailable",
		Message: "record store is unavailable",
	}})
	return true
}

func recordListParams(r *http.Request) (db.RecordListParams, error) {
	params := db.RecordListParams{}
	if value := strings.TrimSpace(r.URL.Query().Get("limit")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return db.RecordListParams{}, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: "limit must be an integer", Details: map[string]any{"limit": value}, Err: err}
		}
		if parsed <= 0 {
			return db.RecordListParams{}, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: "limit must be between 1 and 100", Details: map[string]any{"limit": parsed}}
		}
		params.Limit = parsed
	}
	if value := strings.TrimSpace(r.URL.Query().Get("offset")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return db.RecordListParams{}, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: "offset must be an integer", Details: map[string]any{"offset": value}, Err: err}
		}
		params.Offset = parsed
	}
	return params, nil
}

func recordIDParam(entity string, raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: "record id must be a positive integer", Details: map[string]any{"entity": entity, "id": raw}, Err: err}
	}
	return id, nil
}

func decodeRecordInput(r *http.Request) (db.RecordInput, error) {
	var envelope struct {
		Data db.RecordInput `json:"data"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: "request body is required"}
		}
		return nil, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: "request body must be valid JSON", Err: err}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: "request body must contain one JSON object", Err: err}
	}
	if envelope.Data == nil {
		return nil, db.RecordError{Code: db.RecordErrorInvalidRequest, Message: "request data is required"}
	}
	return envelope.Data, nil
}

func writeRecordError(w http.ResponseWriter, err error) {
	var recordErr db.RecordError
	if !errors.As(err, &recordErr) {
		writeJSON(w, http.StatusInternalServerError, errorEnvelope{Error: apiError{
			Code:    "internal_error",
			Message: "record request failed",
		}})
		return
	}
	status := http.StatusInternalServerError
	message := recordErr.Message
	details := recordErr.Details
	switch recordErr.Code {
	case db.RecordErrorInvalidRequest:
		status = http.StatusBadRequest
	case db.RecordErrorValidation:
		status = http.StatusUnprocessableEntity
	case db.RecordErrorNotFound:
		status = http.StatusNotFound
	case db.RecordErrorConstraintViolation, db.RecordErrorSchemaNotReady:
		status = http.StatusConflict
	case db.RecordErrorInternal:
		status = http.StatusInternalServerError
		message = "record request failed"
		details = nil
	}
	writeJSON(w, status, errorEnvelope{Error: apiError{
		Code:    recordErr.Code,
		Message: message,
		Details: details,
	}})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

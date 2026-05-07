package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dygo-dev/dygo/internal/auth"
	"github.com/dygo-dev/dygo/internal/db"
)

func TestNewRouterHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	NewRouter().ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("health content type = %q, want text/plain; charset=utf-8", got)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(health body) error = %v", err)
	}
	if string(body) != "ok\n" {
		t.Fatalf("health body = %q, want ok newline", string(body))
	}
}

func TestNewRouterNotFound(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/missing", nil)
	recorder := httptest.NewRecorder()

	NewRouter().ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("missing status = %d, want %d", recorder.Result().StatusCode, http.StatusNotFound)
	}
}

func TestServeListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ServeListener(ctx, listener)
	}()

	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + listener.Addr().String() + "/health")
	if err != nil {
		cancel()
		t.Fatalf("GET /health error = %v", err)
	}
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		cancel()
		t.Fatalf("ReadAll(response body) error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("health status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if string(body) != "ok\n" {
		cancel()
		t.Fatalf("health body = %q, want ok newline", string(body))
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeListener() error = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ServeListener() did not stop after context cancellation")
	}
}

func TestMetadataRoutes(t *testing.T) {
	authStore := validFakeAuthStore()
	store := &fakeMetadataStore{
		apps: []db.MetadataApp{{Name: "core", Label: "Core", Version: "0.1.0", Status: "active"}},
		app:  db.MetadataApp{Name: "core", Label: "Core", Version: "0.1.0", Status: "active"},
		entities: []db.MetadataEntity{{
			Name:  "user",
			Label: "User",
			App:   db.MetadataAppRef{Name: "core", Label: "Core"},
		}},
		meta: db.MetadataEntityMeta{
			MetadataEntity: db.MetadataEntity{Name: "user", Label: "User", App: db.MetadataAppRef{Name: "core", Label: "Core"}},
			Fields: []db.MetadataField{{
				Name:     "email",
				Label:    "Email",
				Type:     "email",
				Required: true,
				Unique:   true,
				Index:    true,
				Position: 1,
				Options:  json.RawMessage(`{"values":["a"]}`),
			}},
			Indexes: []db.MetadataIndex{{Name: "by-email", Fields: json.RawMessage(`["email"]`), Position: 1}},
			Constraints: []db.MetadataConstraint{{
				Name:     "user_email_key",
				Type:     "unique",
				Fields:   json.RawMessage(`["email"]`),
				Position: 1,
			}},
		},
	}

	tests := []struct {
		path string
		want string
	}{
		{path: "/api/v1/apps", want: `"name":"core"`},
		{path: "/api/v1/apps/core", want: `"status":"active"`},
		{path: "/api/v1/entities", want: `"name":"user"`},
		{path: "/api/v1/entities/user/meta", want: `"fields":[{"name":"email"`},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			request := authenticatedRequest(http.MethodGet, tt.path, "")
			recorder := httptest.NewRecorder()

			NewRouter(Options{Auth: authStore, Metadata: store}).ServeHTTP(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("%s status = %d, want 200", tt.path, response.StatusCode)
			}
			if got := response.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("%s content type = %q, want application/json", tt.path, got)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("ReadAll(%s) error = %v", tt.path, err)
			}
			if !contains(string(body), `"data":`) || !contains(string(body), tt.want) {
				t.Fatalf("%s body = %s, want data envelope with %q", tt.path, string(body), tt.want)
			}
		})
	}
}

func TestMetadataRouteNotFound(t *testing.T) {
	store := &fakeMetadataStore{appErr: db.MetadataNotFoundError{Kind: "app", Name: "missing"}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/apps/missing", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Metadata: store}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("not found status = %d, want 404", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(not found body) error = %v", err)
	}
	want := `{"error":{"code":"not_found","message":"app not found","details":{"app":"missing"}}}`
	if !contains(string(body), want) {
		t.Fatalf("not found body = %s, want %s", string(body), want)
	}
}

func TestMetadataRouteFailureIsRedacted(t *testing.T) {
	store := &fakeMetadataStore{appsErr: errors.New("postgres://user:secret@localhost:5432/dygo failed")}
	request := authenticatedRequest(http.MethodGet, "/api/v1/apps", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Metadata: store}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("failure status = %d, want 500", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(failure body) error = %v", err)
	}
	for _, leaked := range []string{"postgres://", "secret"} {
		if contains(string(body), leaked) {
			t.Fatalf("failure body leaked %q: %s", leaked, string(body))
		}
	}
	if !contains(string(body), `"code":"internal_error"`) {
		t.Fatalf("failure body = %s, want internal_error", string(body))
	}
}

func TestMetadataRouteWithoutStore(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/apps", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-token"})
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore()}).ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("metadata without store status = %d, want 503", recorder.Result().StatusCode)
	}
}

func TestAuthRoutes(t *testing.T) {
	authStore := validFakeAuthStore()
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		cookie     bool
		status     int
		want       string
		wantCookie bool
	}{
		{
			name:       "login",
			method:     http.MethodPost,
			path:       "/api/v1/auth/login",
			body:       `{"data":{"email":"admin@example.com","password":"secret"}}`,
			status:     http.StatusOK,
			want:       `"administrator":true`,
			wantCookie: true,
		},
		{
			name:   "me",
			method: http.MethodGet,
			path:   "/api/v1/auth/me",
			cookie: true,
			status: http.StatusOK,
			want:   `"email":"admin@example.com"`,
		},
		{
			name:   "logout",
			method: http.MethodPost,
			path:   "/api/v1/auth/logout",
			cookie: true,
			status: http.StatusOK,
			want:   `"logged-out":true`,
		},
		{
			name:   "protected route without cookie",
			method: http.MethodGet,
			path:   "/api/v1/entities",
			status: http.StatusUnauthorized,
			want:   `"code":"unauthenticated"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.cookie {
				request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-token"})
			}
			recorder := httptest.NewRecorder()

			NewRouter(Options{Auth: authStore, Metadata: &fakeMetadataStore{}}).ServeHTTP(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()
			if response.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.status)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("ReadAll(auth body) error = %v", err)
			}
			if !contains(string(body), tt.want) {
				t.Fatalf("body = %s, want %q", string(body), tt.want)
			}
			if tt.wantCookie && len(response.Cookies()) == 0 {
				t.Fatal("login response cookies = none, want session cookie")
			}
			if tt.wantCookie {
				cookie := response.Cookies()[0]
				if cookie.Name != sessionCookieName || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Value != "issued-token" {
					t.Fatalf("login cookie = %+v, want HttpOnly dygo session", cookie)
				}
			}
		})
	}
	if authStore.logoutToken != "valid-token" {
		t.Fatalf("logout token = %q, want valid-token", authStore.logoutToken)
	}
}

func TestAuthRouteErrors(t *testing.T) {
	tests := []struct {
		name   string
		store  *fakeAuthStore
		body   string
		status int
		want   string
	}{
		{
			name:   "bad body",
			store:  validFakeAuthStore(),
			body:   `{"data":`,
			status: http.StatusBadRequest,
			want:   `"code":"invalid_request"`,
		},
		{
			name:   "invalid credentials",
			store:  &fakeAuthStore{loginErr: auth.Error{Code: auth.ErrorInvalidCredentials, Message: "invalid email or password"}},
			body:   `{"data":{"email":"admin@example.com","password":"wrong"}}`,
			status: http.StatusUnauthorized,
			want:   `"code":"invalid_credentials"`,
		},
		{
			name:   "schema not ready",
			store:  &fakeAuthStore{loginErr: auth.Error{Code: auth.ErrorSchemaNotReady, Message: "auth schema is not ready; run dygo migrate", Err: errors.New("postgres://secret")}},
			body:   `{"data":{"email":"admin@example.com","password":"secret"}}`,
			status: http.StatusConflict,
			want:   `"code":"schema_not_ready"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(tt.body))
			recorder := httptest.NewRecorder()

			NewRouter(Options{Auth: tt.store}).ServeHTTP(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()
			if response.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.status)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("ReadAll(auth error body) error = %v", err)
			}
			if !contains(string(body), tt.want) {
				t.Fatalf("body = %s, want %q", string(body), tt.want)
			}
			if contains(string(body), "postgres://") || contains(string(body), "secret") {
				t.Fatalf("body leaked auth detail: %s", string(body))
			}
		})
	}
}

func TestRecordRoutes(t *testing.T) {
	authStore := validFakeAuthStore()
	store := &fakeRecordStore{
		list: db.RecordListResult{
			Records: []db.Record{{"id": int64(1), "email": "a@example.com"}},
			Limit:   50,
			Offset:  0,
			Count:   1,
		},
		record: db.Record{"id": int64(1), "email": "a@example.com"},
	}

	tests := []struct {
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{method: http.MethodGet, path: "/api/v1/records/user", status: http.StatusOK, want: `"meta":{"limit":50,"offset":0,"count":1}`},
		{method: http.MethodGet, path: "/api/v1/records/user/1", status: http.StatusOK, want: `"email":"a@example.com"`},
		{method: http.MethodPost, path: "/api/v1/records/user", body: `{"data":{"email":"a@example.com"}}`, status: http.StatusCreated, want: `"data":`},
		{method: http.MethodPatch, path: "/api/v1/records/user/1", body: `{"data":{"email":"b@example.com"}}`, status: http.StatusOK, want: `"data":`},
		{method: http.MethodDelete, path: "/api/v1/records/user/1", status: http.StatusOK, want: `"deleted":true`},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			request := authenticatedRequest(tt.method, tt.path, tt.body)
			recorder := httptest.NewRecorder()

			NewRouter(Options{Auth: authStore, Records: store}).ServeHTTP(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()
			if response.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.status)
			}
			if got := response.Header.Get("Content-Type"); got != "application/json" {
				t.Fatalf("content type = %q, want application/json", got)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("ReadAll(record body) error = %v", err)
			}
			if !contains(string(body), tt.want) {
				t.Fatalf("body = %s, want %q", string(body), tt.want)
			}
		})
	}
	if string(store.created["email"]) != `"a@example.com"` {
		t.Fatalf("created input = %#v, want email", store.created)
	}
	if string(store.updated["email"]) != `"b@example.com"` {
		t.Fatalf("updated input = %#v, want email", store.updated)
	}
	if store.deletedID != 1 {
		t.Fatalf("deleted id = %d, want 1", store.deletedID)
	}
}

func TestRecordRouteErrors(t *testing.T) {
	tests := []struct {
		name   string
		store  *fakeRecordStore
		method string
		path   string
		body   string
		status int
		want   string
	}{
		{
			name:   "invalid id",
			store:  &fakeRecordStore{},
			method: http.MethodGet,
			path:   "/api/v1/records/user/nope",
			status: http.StatusBadRequest,
			want:   `"code":"invalid_request"`,
		},
		{
			name:   "invalid body",
			store:  &fakeRecordStore{},
			method: http.MethodPost,
			path:   "/api/v1/records/user",
			body:   `{"data":`,
			status: http.StatusBadRequest,
			want:   `"code":"invalid_request"`,
		},
		{
			name:   "validation error",
			store:  &fakeRecordStore{createErr: db.RecordError{Code: db.RecordErrorValidation, Message: "required field is missing", Details: map[string]any{"field": "email"}}},
			method: http.MethodPost,
			path:   "/api/v1/records/user",
			body:   `{"data":{}}`,
			status: http.StatusUnprocessableEntity,
			want:   `"code":"validation_error"`,
		},
		{
			name:   "not found",
			store:  &fakeRecordStore{getErr: db.RecordError{Code: db.RecordErrorNotFound, Message: "record not found", Details: map[string]any{"entity": "user", "id": 1}}},
			method: http.MethodGet,
			path:   "/api/v1/records/user/1",
			status: http.StatusNotFound,
			want:   `"code":"not_found"`,
		},
		{
			name:   "constraint violation",
			store:  &fakeRecordStore{createErr: db.RecordError{Code: db.RecordErrorConstraintViolation, Message: "record violates a database constraint", Details: map[string]any{"constraint": "user_email_key"}}},
			method: http.MethodPost,
			path:   "/api/v1/records/user",
			body:   `{"data":{"email":"a@example.com"}}`,
			status: http.StatusConflict,
			want:   `"code":"constraint_violation"`,
		},
		{
			name:   "internal error redacted",
			store:  &fakeRecordStore{listErr: db.RecordError{Code: db.RecordErrorInternal, Message: "postgres://user:secret@localhost failed"}},
			method: http.MethodGet,
			path:   "/api/v1/records/user",
			status: http.StatusInternalServerError,
			want:   `"message":"record request failed"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-token"})
			recorder := httptest.NewRecorder()

			NewRouter(Options{Auth: validFakeAuthStore(), Records: tt.store}).ServeHTTP(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()
			if response.StatusCode != tt.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.status)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("ReadAll(record error body) error = %v", err)
			}
			if !contains(string(body), tt.want) {
				t.Fatalf("body = %s, want %q", string(body), tt.want)
			}
			if contains(string(body), "postgres://") || contains(string(body), "secret") {
				t.Fatalf("body leaked internal detail: %s", string(body))
			}
		})
	}
}

func TestRecordRouteWithoutStore(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/records/user", nil)
	recorder := httptest.NewRecorder()

	NewRouter().ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("record without store status = %d, want 503", recorder.Result().StatusCode)
	}
}

type fakeMetadataStore struct {
	apps      []db.MetadataApp
	appsErr   error
	app       db.MetadataApp
	appErr    error
	entities  []db.MetadataEntity
	entityErr error
	meta      db.MetadataEntityMeta
	metaErr   error
}

func (s *fakeMetadataStore) ListApps(context.Context) ([]db.MetadataApp, error) {
	return s.apps, s.appsErr
}

func (s *fakeMetadataStore) GetApp(context.Context, string) (db.MetadataApp, error) {
	return s.app, s.appErr
}

func (s *fakeMetadataStore) ListEntities(context.Context) ([]db.MetadataEntity, error) {
	return s.entities, s.entityErr
}

func (s *fakeMetadataStore) GetEntityMeta(context.Context, string) (db.MetadataEntityMeta, error) {
	return s.meta, s.metaErr
}

type fakeRecordStore struct {
	list      db.RecordListResult
	listErr   error
	record    db.Record
	getErr    error
	createErr error
	updateErr error
	deleteErr error
	created   db.RecordInput
	updated   db.RecordInput
	deletedID int64
}

func (s *fakeRecordStore) ListRecords(context.Context, string, db.RecordListParams) (db.RecordListResult, error) {
	return s.list, s.listErr
}

func (s *fakeRecordStore) GetRecord(context.Context, string, int64) (db.Record, error) {
	return s.record, s.getErr
}

func (s *fakeRecordStore) CreateRecord(_ context.Context, _ string, input db.RecordInput) (db.Record, error) {
	s.created = input
	return s.record, s.createErr
}

func (s *fakeRecordStore) UpdateRecord(_ context.Context, _ string, _ int64, input db.RecordInput) (db.Record, error) {
	s.updated = input
	return s.record, s.updateErr
}

func (s *fakeRecordStore) DeleteRecord(_ context.Context, _ string, id int64) error {
	s.deletedID = id
	return s.deleteErr
}

type fakeAuthStore struct {
	user        auth.User
	loginResult auth.LoginResult
	loginErr    error
	currentErr  error
	logoutErr   error
	logoutToken string
}

func validFakeAuthStore() *fakeAuthStore {
	user := auth.User{ID: 7, Email: "admin@example.com", FullName: "Admin User", Enabled: true, Administrator: true}
	return &fakeAuthStore{
		user: user,
		loginResult: auth.LoginResult{
			User:      user,
			Token:     "issued-token",
			ExpiresAt: time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC),
		},
	}
}

func (s *fakeAuthStore) Login(context.Context, auth.LoginRequest) (auth.LoginResult, error) {
	return s.loginResult, s.loginErr
}

func (s *fakeAuthStore) CurrentUser(context.Context, string) (auth.User, error) {
	return s.user, s.currentErr
}

func (s *fakeAuthStore) Logout(_ context.Context, token string) error {
	s.logoutToken = token
	return s.logoutErr
}

func authenticatedRequest(method string, path string, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-token"})
	return request
}

func contains(value string, want string) bool {
	return strings.Contains(value, want)
}

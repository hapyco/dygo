package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dygo-dev/dygo/internal/auth"
	"github.com/dygo-dev/dygo/internal/db"
	"github.com/dygo-dev/dygo/internal/permissions"
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

func TestNewRouterStudioFallback(t *testing.T) {
	studio := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("studio"))
	})
	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	recorder := httptest.NewRecorder()

	NewRouter(Options{Studio: studio}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("studio fallback status = %d, want 200", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(studio body) error = %v", err)
	}
	if string(body) != "studio" {
		t.Fatalf("studio fallback body = %q, want studio", string(body))
	}
}

func TestNewRouterStudioFallbackDoesNotCatchAPI(t *testing.T) {
	studio := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("studio"))
	})
	request := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	recorder := httptest.NewRecorder()

	NewRouter(Options{Studio: studio}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("api fallback status = %d, want 404", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(api fallback body) error = %v", err)
	}
	if contains(string(body), "studio") {
		t.Fatalf("api fallback body = %q, should not proxy Studio", string(body))
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
			MetadataEntity: db.MetadataEntity{Name: "user", RouteSlug: "user", Label: "User", App: db.MetadataAppRef{Name: "core", Label: "Core"}},
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

			NewRouter(Options{Auth: authStore, Metadata: store, Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

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

	NewRouter(Options{Auth: validFakeAuthStore(), Metadata: store, Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

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

	NewRouter(Options{Auth: validFakeAuthStore(), Metadata: store, Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

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

func TestMetadataRoutesFailClosedWithoutPermissionChecker(t *testing.T) {
	store := &fakeMetadataStore{apps: []db.MetadataApp{{Name: "core", Label: "Core", Version: "0.1.0", Status: "active"}}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/apps", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Metadata: store}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("metadata without permission checker status = %d, want 503", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(metadata without permission checker body) error = %v", err)
	}
	if !contains(string(body), `"code":"service_unavailable"`) || !contains(string(body), "permission checker is unavailable") {
		t.Fatalf("metadata without permission checker body = %s, want service_unavailable", string(body))
	}
}

func TestMetadataEntityListFiltersUnreadableEntities(t *testing.T) {
	authStore := validFakeAuthStore()
	authStore.user.Administrator = false
	store := &fakeMetadataStore{entities: []db.MetadataEntity{
		{Name: "user", RouteSlug: "user", Label: "User", App: db.MetadataAppRef{Name: "core", Label: "Core"}},
		{Name: "role", RouteSlug: "role", Label: "Role", App: db.MetadataAppRef{Name: "core", Label: "Core"}},
	}}
	checker := &fakePermissionChecker{denied: map[string]bool{"user": true}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/entities", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: authStore, Metadata: store, Permissions: checker}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("filtered entities status = %d, want 200", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(filtered entities body) error = %v", err)
	}
	if contains(string(body), `"name":"user"`) || !contains(string(body), `"name":"role"`) {
		t.Fatalf("filtered entities body = %s, want role only", string(body))
	}
}

func TestMetadataEntityMetaDenied(t *testing.T) {
	authStore := validFakeAuthStore()
	authStore.user.Administrator = false
	store := &fakeMetadataStore{meta: db.MetadataEntityMeta{MetadataEntity: db.MetadataEntity{Name: "user", RouteSlug: "user", Label: "User", App: db.MetadataAppRef{Name: "core", Label: "Core"}}}}
	checker := &fakePermissionChecker{denied: map[string]bool{"user": true}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/entities/user/meta", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: authStore, Metadata: store, Permissions: checker}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("metadata denied status = %d, want 403", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(metadata denied body) error = %v", err)
	}
	if !contains(string(body), `"code":"forbidden"`) {
		t.Fatalf("metadata denied body = %s, want forbidden", string(body))
	}
}

func TestMetadataRoutesAdministratorReceivesFullMetadata(t *testing.T) {
	store := &fakeMetadataStore{entities: []db.MetadataEntity{
		{Name: "user", RouteSlug: "user", Label: "User", App: db.MetadataAppRef{Name: "core", Label: "Core"}},
		{Name: "role", RouteSlug: "role", Label: "Role", App: db.MetadataAppRef{Name: "core", Label: "Core"}},
	}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/entities", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Metadata: store, Permissions: permissions.NewChecker(nil)}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("administrator metadata status = %d, want 200", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(administrator metadata body) error = %v", err)
	}
	if !contains(string(body), `"name":"user"`) || !contains(string(body), `"name":"role"`) {
		t.Fatalf("administrator metadata body = %s, want all entities", string(body))
	}
}

func TestMetadataAppsFollowReadableEntities(t *testing.T) {
	authStore := validFakeAuthStore()
	authStore.user.Administrator = false
	store := &fakeMetadataStore{
		apps: []db.MetadataApp{
			{Name: "core", Label: "Core", Version: "0.1.0", Status: "active"},
			{Name: "crm", Label: "CRM", Version: "0.1.0", Status: "active"},
		},
		app: db.MetadataApp{Name: "core", Label: "Core", Version: "0.1.0", Status: "active"},
		entities: []db.MetadataEntity{
			{Name: "user", RouteSlug: "user", Label: "User", App: db.MetadataAppRef{Name: "core", Label: "Core"}},
			{Name: "lead", RouteSlug: "lead", Label: "Lead", App: db.MetadataAppRef{Name: "crm", Label: "CRM"}},
		},
	}
	checker := &fakePermissionChecker{denied: map[string]bool{"app": true, "user": true}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/apps", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: authStore, Metadata: store, Permissions: checker}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("filtered apps status = %d, want 200", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(filtered apps body) error = %v", err)
	}
	if contains(string(body), `"name":"core"`) || !contains(string(body), `"name":"crm"`) {
		t.Fatalf("filtered apps body = %s, want crm only", string(body))
	}

	store.app = db.MetadataApp{Name: "core", Label: "Core", Version: "0.1.0", Status: "active"}
	request = authenticatedRequest(http.MethodGet, "/api/v1/apps/core", "")
	recorder = httptest.NewRecorder()

	NewRouter(Options{Auth: authStore, Metadata: store, Permissions: checker}).ServeHTTP(recorder, request)

	response = recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("forbidden app detail status = %d, want 403", response.StatusCode)
	}
}

func TestMetadataAppsReadableWhenCoreAppPermissionAllows(t *testing.T) {
	authStore := validFakeAuthStore()
	authStore.user.Administrator = false
	store := &fakeMetadataStore{apps: []db.MetadataApp{
		{Name: "core", Label: "Core", Version: "0.1.0", Status: "active"},
		{Name: "crm", Label: "CRM", Version: "0.1.0", Status: "active"},
	}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/apps", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: authStore, Metadata: store, Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("core app permission metadata status = %d, want 200", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(core app permission body) error = %v", err)
	}
	if !contains(string(body), `"name":"core"`) || !contains(string(body), `"name":"crm"`) {
		t.Fatalf("core app permission body = %s, want all apps", string(body))
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
			name:       "login with identifier and remember",
			method:     http.MethodPost,
			path:       "/api/v1/auth/login",
			body:       `{"data":{"identifier":"admin@example.com","password":"secret","remember":true}}`,
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
				if strings.Contains(tt.body, `"remember":true`) && cookie.Expires.IsZero() {
					t.Fatalf("remembered login cookie Expires is zero, want persistent cookie")
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
	checker := &fakePermissionChecker{}
	store := &fakeRecordStore{
		list: db.RecordListResult{
			Records: []db.Record{{"id": int64(1), "email": "a@example.com"}},
			Limit:   50,
			Offset:  0,
			Count:   1,
			Total:   1,
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
		{method: http.MethodGet, path: "/api/v1/records/user", status: http.StatusOK, want: `"meta":{"limit":50,"offset":0,"count":1,"total":1}`},
		{method: http.MethodGet, path: "/api/v1/records/user/1", status: http.StatusOK, want: `"email":"a@example.com"`},
		{method: http.MethodGet, path: "/api/v1/records/user/name/a%40example.com", status: http.StatusOK, want: `"email":"a@example.com"`},
		{method: http.MethodPost, path: "/api/v1/records/user", body: `{"data":{"email":"a@example.com"}}`, status: http.StatusCreated, want: `"data":`},
		{method: http.MethodPatch, path: "/api/v1/records/user/1", body: `{"data":{"email":"b@example.com"}}`, status: http.StatusOK, want: `"data":`},
		{method: http.MethodDelete, path: "/api/v1/records/user/1", status: http.StatusOK, want: `"deleted":true`},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			request := authenticatedRequest(tt.method, tt.path, tt.body)
			recorder := httptest.NewRecorder()

			NewRouter(Options{Auth: authStore, Records: store, Permissions: checker}).ServeHTTP(recorder, request)

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
	if string(store.findMatch["name"]) != `"a@example.com"` {
		t.Fatalf("find match = %#v, want name", store.findMatch)
	}
	if store.deletedID != 1 {
		t.Fatalf("deleted id = %d, want 1", store.deletedID)
	}
	if store.createActor != 7 || store.updateActor != 7 || store.deleteActor != 7 {
		t.Fatalf("activity actors create/update/delete = %d/%d/%d, want authenticated user 7", store.createActor, store.updateActor, store.deleteActor)
	}
	if store.createSource != db.ActivitySourceAPI || store.updateSource != db.ActivitySourceAPI || store.deleteSource != db.ActivitySourceAPI {
		t.Fatalf("activity sources create/update/delete = %q/%q/%q, want api", store.createSource, store.updateSource, store.deleteSource)
	}
	wantPermissions := []permissions.Request{
		{Actor: permissions.Actor{UserID: 7, Administrator: true}, Entity: "user", Action: permissions.ActionRead},
		{Actor: permissions.Actor{UserID: 7, Administrator: true}, Entity: "user", Action: permissions.ActionRead, RecordID: 1},
		{Actor: permissions.Actor{UserID: 7, Administrator: true}, Entity: "user", Action: permissions.ActionRead},
		{Actor: permissions.Actor{UserID: 7, Administrator: true}, Entity: "user", Action: permissions.ActionCreate},
		{Actor: permissions.Actor{UserID: 7, Administrator: true}, Entity: "user", Action: permissions.ActionUpdate, RecordID: 1},
		{Actor: permissions.Actor{UserID: 7, Administrator: true}, Entity: "user", Action: permissions.ActionDelete, RecordID: 1},
	}
	if len(checker.requests) != len(wantPermissions) {
		t.Fatalf("permission requests = %+v, want %d requests", checker.requests, len(wantPermissions))
	}
	for i, want := range wantPermissions {
		if checker.requests[i] != want {
			t.Fatalf("permission request %d = %+v, want %+v", i, checker.requests[i], want)
		}
	}
}

func TestRecordListRouteParsesFiltersAndSort(t *testing.T) {
	store := &fakeRecordStore{
		list: db.RecordListResult{Records: []db.Record{}, Limit: 25, Offset: 5},
	}
	request := authenticatedRequest(http.MethodGet, "/api/v1/records/user?status=Open&enabled=true&sort=-created-at,name&limit=25&offset=5", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Records: store, Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("record list status = %d, want 200", response.StatusCode)
	}
	if store.listParams.Limit != 25 || store.listParams.Offset != 5 {
		t.Fatalf("list params pagination = %+v, want limit 25 offset 5", store.listParams)
	}
	wantFilters := []db.RecordFilter{{Field: "enabled", Value: "true"}, {Field: "status", Value: "Open"}}
	if !reflect.DeepEqual(store.listParams.Filters, wantFilters) {
		t.Fatalf("list filters = %#v, want %#v", store.listParams.Filters, wantFilters)
	}
	wantSort := []db.RecordSort{{Field: "created-at", Desc: true}, {Field: "name"}}
	if !reflect.DeepEqual(store.listParams.Sort, wantSort) {
		t.Fatalf("list sort = %#v, want %#v", store.listParams.Sort, wantSort)
	}
}

func TestRecordListRouteRejectsDuplicateFilter(t *testing.T) {
	store := &fakeRecordStore{}
	request := authenticatedRequest(http.MethodGet, "/api/v1/records/user?email=a@example.com&email=b@example.com", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Records: store, Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("duplicate filter status = %d, want 400", response.StatusCode)
	}
	if len(store.calls) != 0 {
		t.Fatalf("record store calls = %v, want none", store.calls)
	}
}

func TestRecordListRouteRejectsInvalidSort(t *testing.T) {
	store := &fakeRecordStore{}
	request := authenticatedRequest(http.MethodGet, "/api/v1/records/user?sort=-", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Records: store, Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid sort status = %d, want 400", response.StatusCode)
	}
	if len(store.calls) != 0 {
		t.Fatalf("record store calls = %v, want none", store.calls)
	}
}

func TestRecordActivityRoute(t *testing.T) {
	authStore := validFakeAuthStore()
	checker := &fakePermissionChecker{}
	created := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	activity := &fakeActivityStore{
		result: db.ActivityListResult{
			Activities: []db.ActivityEntry{{
				ID:        9,
				CreatedAt: created,
				Entity:    "user",
				RecordID:  1,
				Kind:      "record",
				Operation: "update",
				Status:    "success",
				Title:     "Updated User",
				Actor:     &db.ActivityActor{ID: 7, Email: "admin@example.com", FullName: "Admin User"},
				Changes:   []any{map[string]any{"field": "email", "old": "a@example.com", "new": "b@example.com"}},
				Snapshot:  map[string]any{"id": int64(1), "email": "b@example.com"},
				Details:   map[string]any{"source": db.ActivitySourceAPI},
			}},
			Limit:  25,
			Offset: 5,
			Count:  1,
		},
	}
	request := authenticatedRequest(http.MethodGet, "/api/v1/records/user/1/activity?limit=25&offset=5", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: authStore, Activity: activity, Permissions: checker}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("activity status = %d, want 200", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("activity content type = %q, want application/json", got)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(activity body) error = %v", err)
	}
	for _, want := range []string{
		`"operation":"update"`,
		`"actor":{"id":7,"email":"admin@example.com","full-name":"Admin User"}`,
		`"meta":{"limit":25,"offset":5,"count":1}`,
	} {
		if !contains(string(body), want) {
			t.Fatalf("activity body = %s, want %q", string(body), want)
		}
	}
	if activity.entity != "user" || activity.recordID != 1 || activity.params.Limit != 25 || activity.params.Offset != 5 {
		t.Fatalf("activity request = entity %q id %d params %+v, want user/1 limit 25 offset 5", activity.entity, activity.recordID, activity.params)
	}
	wantPermission := permissions.Request{Actor: permissions.Actor{UserID: 7, Administrator: true}, Entity: "user", Action: permissions.ActionRead, RecordID: 1}
	if len(checker.requests) != 1 || checker.requests[0] != wantPermission {
		t.Fatalf("permission requests = %+v, want %+v", checker.requests, wantPermission)
	}
}

func TestRecordActivityRouteForbiddenBeforeStore(t *testing.T) {
	activity := &fakeActivityStore{}
	checker := &fakePermissionChecker{err: permissions.Error{Code: permissions.ErrorDenied, Message: "permission denied", Details: map[string]any{"entity": "user", "action": permissions.ActionRead}}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/records/user/1/activity", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Activity: activity, Permissions: checker}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("activity forbidden status = %d, want 403", response.StatusCode)
	}
	if len(activity.calls) != 0 {
		t.Fatalf("activity store calls = %v, want none", activity.calls)
	}
}

func TestRecordActivityRouteUnauthenticatedBeforePermissionCheck(t *testing.T) {
	activity := &fakeActivityStore{}
	checker := &fakePermissionChecker{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/records/user/1/activity", nil)
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Activity: activity, Permissions: checker}).ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("activity unauthenticated status = %d, want 401", recorder.Result().StatusCode)
	}
	if len(checker.requests) != 0 {
		t.Fatalf("permission requests = %+v, want none", checker.requests)
	}
	if len(activity.calls) != 0 {
		t.Fatalf("activity store calls = %v, want none", activity.calls)
	}
}

func TestRecordActivityRouteErrors(t *testing.T) {
	tests := []struct {
		name   string
		store  *fakeActivityStore
		path   string
		status int
		want   string
	}{
		{
			name:   "invalid id",
			store:  &fakeActivityStore{},
			path:   "/api/v1/records/user/nope/activity",
			status: http.StatusBadRequest,
			want:   `"code":"invalid_request"`,
		},
		{
			name:   "invalid pagination",
			store:  &fakeActivityStore{},
			path:   "/api/v1/records/user/1/activity?limit=nope",
			status: http.StatusBadRequest,
			want:   `"code":"invalid_request"`,
		},
		{
			name:   "unknown entity",
			store:  &fakeActivityStore{err: db.RecordError{Code: db.RecordErrorNotFound, Message: "entity not found", Details: map[string]any{"entity": "missing"}}},
			path:   "/api/v1/records/missing/1/activity",
			status: http.StatusNotFound,
			want:   `"code":"not_found"`,
		},
		{
			name:   "schema not ready",
			store:  &fakeActivityStore{err: db.RecordError{Code: db.RecordErrorSchemaNotReady, Message: "schema is not ready; run dygo migrate", Details: map[string]any{"entity": "activity"}}},
			path:   "/api/v1/records/user/1/activity",
			status: http.StatusConflict,
			want:   `"code":"schema_not_ready"`,
		},
		{
			name:   "internal error redacted",
			store:  &fakeActivityStore{err: db.RecordError{Code: db.RecordErrorInternal, Message: "postgres://user:secret@localhost failed"}},
			path:   "/api/v1/records/user/1/activity",
			status: http.StatusInternalServerError,
			want:   `"message":"record request failed"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := authenticatedRequest(http.MethodGet, tt.path, "")
			recorder := httptest.NewRecorder()

			NewRouter(Options{Auth: validFakeAuthStore(), Activity: tt.store, Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

			response := recorder.Result()
			defer response.Body.Close()
			if response.StatusCode != tt.status {
				t.Fatalf("activity status = %d, want %d", response.StatusCode, tt.status)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("ReadAll(activity error body) error = %v", err)
			}
			if !contains(string(body), tt.want) {
				t.Fatalf("activity body = %s, want %q", string(body), tt.want)
			}
			if contains(string(body), "postgres://") || contains(string(body), "secret") {
				t.Fatalf("activity body leaked internal detail: %s", string(body))
			}
		})
	}
}

func TestRecordActivityRouteWithoutStore(t *testing.T) {
	request := authenticatedRequest(http.MethodGet, "/api/v1/records/user/1/activity", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("activity without store status = %d, want 503", recorder.Result().StatusCode)
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

			NewRouter(Options{Auth: validFakeAuthStore(), Records: tt.store, Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

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
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-token"})
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Permissions: &fakePermissionChecker{}}).ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("record without store status = %d, want 503", recorder.Result().StatusCode)
	}
}

func TestRecordRoutesFailClosedWithoutPermissionChecker(t *testing.T) {
	store := &fakeRecordStore{}
	request := authenticatedRequest(http.MethodGet, "/api/v1/records/user", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Records: store}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("record without permission checker status = %d, want 503", response.StatusCode)
	}
	if len(store.calls) != 0 {
		t.Fatalf("record store calls = %v, want none", store.calls)
	}
}

func TestRecordRoutesDenyForbiddenBeforeStore(t *testing.T) {
	store := &fakeRecordStore{}
	checker := &fakePermissionChecker{err: permissions.Error{Code: permissions.ErrorDenied, Message: "permission denied", Details: map[string]any{"entity": "user", "action": permissions.ActionRead}}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/records/user", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Records: store, Permissions: checker}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("permission denied status = %d, want 403", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(permission denied body) error = %v", err)
	}
	if !contains(string(body), `"code":"forbidden"`) {
		t.Fatalf("permission denied body = %s, want forbidden", string(body))
	}
	if len(store.calls) != 0 {
		t.Fatalf("record store calls = %v, want none", store.calls)
	}
}

func TestRecordRoutesAdministratorUsesPermissionEngineBypass(t *testing.T) {
	store := &fakeRecordStore{record: db.Record{"id": int64(1), "email": "a@example.com"}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/records/user/1", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Records: store, Permissions: permissions.NewChecker(nil)}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("administrator record status = %d, want 200", response.StatusCode)
	}
	if len(store.calls) != 1 || store.calls[0] != "get" {
		t.Fatalf("record store calls = %v, want get", store.calls)
	}
}

func TestRecordRoutesUnauthenticatedBeforePermissionCheck(t *testing.T) {
	store := &fakeRecordStore{}
	checker := &fakePermissionChecker{}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/records/user", nil)
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Records: store, Permissions: checker}).ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated record status = %d, want 401", recorder.Result().StatusCode)
	}
	if len(checker.requests) != 0 {
		t.Fatalf("permission requests = %+v, want none", checker.requests)
	}
	if len(store.calls) != 0 {
		t.Fatalf("record store calls = %v, want none", store.calls)
	}
}

func TestRecordRoutePermissionFailureIsRedacted(t *testing.T) {
	checker := &fakePermissionChecker{err: permissions.Error{Code: permissions.ErrorInternal, Message: "postgres://user:secret@localhost permission failed", Err: errors.New("SELECT postgres://secret")}}
	request := authenticatedRequest(http.MethodGet, "/api/v1/records/user", "")
	recorder := httptest.NewRecorder()

	NewRouter(Options{Auth: validFakeAuthStore(), Records: &fakeRecordStore{}, Permissions: checker}).ServeHTTP(recorder, request)

	response := recorder.Result()
	defer response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("permission failure status = %d, want 500", response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll(permission failure body) error = %v", err)
	}
	if !contains(string(body), `"code":"internal_error"`) || !contains(string(body), `"message":"permission check failed"`) {
		t.Fatalf("permission failure body = %s, want internal_error", string(body))
	}
	if contains(string(body), "postgres://") || contains(string(body), "secret") || contains(string(body), "SELECT") {
		t.Fatalf("permission failure body leaked internal detail: %s", string(body))
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
	list         db.RecordListResult
	listErr      error
	record       db.Record
	getErr       error
	findErr      error
	createErr    error
	updateErr    error
	deleteErr    error
	findMatch    db.RecordInput
	created      db.RecordInput
	updated      db.RecordInput
	deletedID    int64
	createActor  int64
	updateActor  int64
	deleteActor  int64
	createSource string
	updateSource string
	deleteSource string
	listParams   db.RecordListParams
	calls        []string
}

func (s *fakeRecordStore) ListRecords(_ context.Context, _ string, params db.RecordListParams) (db.RecordListResult, error) {
	s.calls = append(s.calls, "list")
	s.listParams = params
	return s.list, s.listErr
}

func (s *fakeRecordStore) GetRecord(context.Context, string, int64) (db.Record, error) {
	s.calls = append(s.calls, "get")
	return s.record, s.getErr
}

func (s *fakeRecordStore) FindRecord(_ context.Context, _ string, match db.RecordInput) (db.Record, error) {
	s.calls = append(s.calls, "find")
	s.findMatch = match
	return s.record, s.findErr
}

func (s *fakeRecordStore) CreateRecord(ctx context.Context, _ string, input db.RecordInput) (db.Record, error) {
	s.calls = append(s.calls, "create")
	s.created = input
	s.createActor, _ = db.ActivityActorFromContext(ctx)
	s.createSource, _ = db.ActivitySourceFromContext(ctx)
	return s.record, s.createErr
}

func (s *fakeRecordStore) UpdateRecord(ctx context.Context, _ string, _ int64, input db.RecordInput) (db.Record, error) {
	s.calls = append(s.calls, "update")
	s.updated = input
	s.updateActor, _ = db.ActivityActorFromContext(ctx)
	s.updateSource, _ = db.ActivitySourceFromContext(ctx)
	return s.record, s.updateErr
}

func (s *fakeRecordStore) DeleteRecord(ctx context.Context, _ string, id int64) error {
	s.calls = append(s.calls, "delete")
	s.deletedID = id
	s.deleteActor, _ = db.ActivityActorFromContext(ctx)
	s.deleteSource, _ = db.ActivitySourceFromContext(ctx)
	return s.deleteErr
}

type fakeActivityStore struct {
	result   db.ActivityListResult
	err      error
	entity   string
	recordID int64
	params   db.RecordListParams
	calls    []string
}

func (s *fakeActivityStore) ListRecordActivity(_ context.Context, entity string, recordID int64, params db.RecordListParams) (db.ActivityListResult, error) {
	s.calls = append(s.calls, "list")
	s.entity = entity
	s.recordID = recordID
	s.params = params
	return s.result, s.err
}

type fakePermissionChecker struct {
	err      error
	denied   map[string]bool
	requests []permissions.Request
}

func (c *fakePermissionChecker) Can(_ context.Context, request permissions.Request) error {
	c.requests = append(c.requests, request)
	if c.denied[request.Entity] {
		return permissions.Error{Code: permissions.ErrorDenied, Message: "permission denied", Details: map[string]any{"entity": request.Entity, "action": request.Action}}
	}
	return c.err
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

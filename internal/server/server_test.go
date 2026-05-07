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
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()

			NewRouter(Options{Metadata: store}).ServeHTTP(recorder, request)

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
	request := httptest.NewRequest(http.MethodGet, "/api/v1/apps/missing", nil)
	recorder := httptest.NewRecorder()

	NewRouter(Options{Metadata: store}).ServeHTTP(recorder, request)

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
	request := httptest.NewRequest(http.MethodGet, "/api/v1/apps", nil)
	recorder := httptest.NewRecorder()

	NewRouter(Options{Metadata: store}).ServeHTTP(recorder, request)

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
	recorder := httptest.NewRecorder()

	NewRouter().ServeHTTP(recorder, request)

	if recorder.Result().StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("metadata without store status = %d, want 503", recorder.Result().StatusCode)
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

func contains(value string, want string) bool {
	return strings.Contains(value, want)
}

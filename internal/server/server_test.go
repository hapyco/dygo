package server

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

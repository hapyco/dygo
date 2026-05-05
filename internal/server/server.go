package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

const shutdownTimeout = 5 * time.Second

// NewRouter creates the dygo HTTP router.
func NewRouter() http.Handler {
	router := chi.NewRouter()
	router.Get("/health", healthHandler)
	return router
}

// Serve starts the dygo HTTP server on address and blocks until it exits.
func Serve(ctx context.Context, address string) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}

	return ServeListener(ctx, listener)
}

// ServeListener starts the dygo HTTP server on an existing listener.
func ServeListener(ctx context.Context, listener net.Listener) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if listener == nil {
		return fmt.Errorf("listener is required")
	}

	httpServer := &http.Server{
		Handler: NewRouter(),
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

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestProgressKeepsFastAndCancelledOperationsQuiet(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelled {
				cancel()
			}
			var stderr bytes.Buffer
			stop := renderProgress(ctx, &stderr, "Working", true, time.Hour)
			stop()
			stop() // Completion and deferred cleanup may both stop the same phase.
			if stderr.Len() != 0 {
				t.Fatalf("fast/cancelled stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestProgressPlainOutputIsBounded(t *testing.T) {
	stderr := newProgressBuffer()
	stop := renderProgress(context.Background(), stderr, "Downloading", false, 0)
	defer stop()
	stderr.wait(t)
	stop()
	if got := stderr.String(); got != "Downloading...\n" {
		t.Fatalf("stderr = %q, want one plain status line", got)
	}
}

func TestProgressClearsTerminalBeforeReturning(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stderr := newProgressBuffer()
			stop := renderProgress(ctx, stderr, "Working", true, 0)
			defer stop()
			stderr.wait(t)
			if cancelled {
				cancel()
			}
			stop()
			_, _ = fmt.Fprint(stderr, "Continue? ")
			if got := stderr.String(); !bytes.HasSuffix([]byte(got), []byte("\r\x1b[2KContinue? ")) {
				t.Fatalf("stderr = %q, want spinner cleared before prompt", got)
			}
		})
	}
}

func TestProgressPreservesOperationError(t *testing.T) {
	stderr := newProgressBuffer()
	failure := errors.New("download failed")
	result, err := withProgress(context.Background(), stderr, "Downloading", func() (int, error) {
		stderr.wait(t)
		return 42, failure
	})
	if result != 42 || !errors.Is(err, failure) {
		t.Fatalf("result = %d, error = %v, want original operation result/error", result, err)
	}
	if got := stderr.String(); got != "Downloading...\n" {
		t.Fatalf("stderr = %q, want status without false success or animation", got)
	}
}

// progressBuffer lets an operation wait until diagnostic output was actually
// rendered, without timing sleeps or concurrent reads of an ordinary Buffer.
type progressBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	writes chan struct{}
}

func newProgressBuffer() *progressBuffer {
	return &progressBuffer{writes: make(chan struct{}, 16)}
}

func (b *progressBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	n, err := b.buffer.Write(p)
	b.mu.Unlock()
	select {
	case b.writes <- struct{}{}:
	default:
	}
	return n, err
}

func (b *progressBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func (b *progressBuffer) wait(t *testing.T) {
	t.Helper()
	select {
	case <-b.writes:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for progress")
	}
}

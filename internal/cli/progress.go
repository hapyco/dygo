package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

const progressDelay = 300 * time.Millisecond

// startProgress reports only operations that outlast the delay. Call stop before
// writing results or prompts; it waits for the renderer to finish using stderr.
func startProgress(ctx context.Context, stderr io.Writer, label string) func() {
	file, ok := stderr.(*os.File)
	animated := ok && term.IsTerminal(int(file.Fd())) && os.Getenv("CI") == "" && os.Getenv("TERM") != "dumb"
	return renderProgress(ctx, stderr, label, animated, progressDelay)
}

func renderProgress(ctx context.Context, stderr io.Writer, label string, animated bool, delay time.Duration) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		// Avoid drawing if completion or cancellation coincided with the delay.
		select {
		case <-stop:
			return
		case <-ctx.Done():
			return
		default:
		}
		if !animated {
			_, _ = fmt.Fprintf(stderr, "%s...\n", label)
			return
		}
		defer fmt.Fprint(stderr, "\r\x1b[2K")
		frames := `|/-\`
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for frame := 0; ; frame++ {
			if _, err := fmt.Fprintf(stderr, "\r\x1b[2K%c %s...", frames[frame%len(frames)], label); err != nil {
				return
			}
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() { close(stop) })
		<-done
	}
}

// withProgress preserves the operation's result/error, including cancellation.
// Progress is diagnostic only; it never writes to the command's stdout.
func withProgress[T any](ctx context.Context, stderr io.Writer, label string, run func() (T, error)) (T, error) {
	stop := startProgress(ctx, stderr, label)
	defer stop()
	return run()
}

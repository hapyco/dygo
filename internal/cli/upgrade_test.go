package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/upgrade"
)

func TestUpgradeCommandRunsUpgrade(t *testing.T) {
	oldRunUpgrade := runUpgrade
	var got upgrade.Options
	runUpgrade = func(_ context.Context, options upgrade.Options) (upgrade.Result, error) {
		got = options
		return upgrade.Result{
			Warnings: []string{"PATH points elsewhere"},
			Lines:    []string{"upgrade check target: v1.2.3", "project: current /app from v1.2.3 to v1.2.3"},
		}, nil
	}
	defer func() {
		runUpgrade = oldRunUpgrade
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"upgrade", "--check", "--to", "v1.2.3"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(upgrade) error = %v, want nil", err)
	}
	if !got.Check || got.TargetVersion != "v1.2.3" {
		t.Fatalf("upgrade options = %+v, want parsed flags", got)
	}
	if !strings.Contains(stdout.String(), "upgrade check target: v1.2.3") || !strings.Contains(stdout.String(), "project: current /app") {
		t.Fatalf("stdout = %q, want upgrade result lines", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: PATH points elsewhere") {
		t.Fatalf("stderr = %q, want warning", stderr.String())
	}
}

func TestUpgradeProgressStopsForConfirmationAndPreservesOutput(t *testing.T) {
	oldRunUpgrade := runUpgrade
	defer func() { runUpgrade = oldRunUpgrade }()
	stderr := newProgressBuffer()
	runUpgrade = func(ctx context.Context, options upgrade.Options) (upgrade.Result, error) {
		stderr.wait(t)
		confirmed, err := options.Confirm(ctx, "Apply upgrade?")
		if err != nil || !confirmed {
			t.Fatalf("confirmation = %v, error = %v", confirmed, err)
		}
		return upgrade.Result{Lines: []string{"project upgraded"}}, nil
	}
	var stdout bytes.Buffer
	stdin := progressPromptReader{t: t, stderr: stderr}
	if err := Run(context.Background(), []string{"upgrade"}, stdin, &stdout, stderr); err != nil {
		t.Fatal(err)
	}
	if got := stdout.String(); got != "project upgraded\n" {
		t.Fatalf("stdout = %q, want only command result", got)
	}
	if got := stderr.String(); got != "Upgrading project and resolving Go dependencies...\nApply upgrade? [y/N]: " {
		t.Fatalf("stderr = %q, want separate progress and prompt", got)
	}
}

type progressPromptReader struct {
	t      *testing.T
	stderr *progressBuffer
}

func (r progressPromptReader) Read(p []byte) (int, error) {
	if got := r.stderr.String(); !strings.HasSuffix(got, "Apply upgrade? [y/N]: ") {
		r.t.Fatalf("stderr before reading confirmation = %q, want clean prompt", got)
	}
	return copy(p, "yes\n"), io.EOF
}

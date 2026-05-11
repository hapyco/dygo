package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/upgrade"
)

func TestUpgradeCommandRunsUpgrade(t *testing.T) {
	oldRunUpgrade := runUpgrade
	var got upgrade.Options
	runUpgrade = func(_ context.Context, options upgrade.Options) (upgrade.Result, error) {
		got = options
		return upgrade.Result{
			Warnings: []string{"PATH points elsewhere"},
			Lines:    []string{"upgrade check target: v1.2.3", "cli: available v1.2.3"},
		}, nil
	}
	defer func() {
		runUpgrade = oldRunUpgrade
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"upgrade", "--check", "--to", "v1.2.3", "--cli-only", "--install-dir", "/tmp/dygo-bin"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(upgrade) error = %v, want nil", err)
	}
	if !got.Check || !got.CLIOnly || got.ProjectOnly || got.TargetVersion != "v1.2.3" || got.InstallDir != "/tmp/dygo-bin" {
		t.Fatalf("upgrade options = %+v, want parsed flags", got)
	}
	if !strings.Contains(stdout.String(), "upgrade check target: v1.2.3") || !strings.Contains(stdout.String(), "cli: available v1.2.3") {
		t.Fatalf("stdout = %q, want upgrade result lines", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: PATH points elsewhere") {
		t.Fatalf("stderr = %q, want warning", stderr.String())
	}
}

func TestUpgradeCommandRejectsExclusiveModes(t *testing.T) {
	oldRunUpgrade := runUpgrade
	runUpgrade = func(_ context.Context, _ upgrade.Options) (upgrade.Result, error) {
		t.Fatal("runUpgrade called, want flag validation before execution")
		return upgrade.Result{}, nil
	}
	defer func() {
		runUpgrade = oldRunUpgrade
	}()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"upgrade", "--cli-only", "--project-only"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(upgrade invalid flags) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "--cli-only and --project-only cannot be used together") {
		t.Fatalf("Run(upgrade invalid flags) error = %q, want exclusive flag error", err.Error())
	}
}

package cli

import (
	"bytes"
	"context"
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

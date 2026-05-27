package upgrade

import (
	"context"
	"strings"
	"testing"
)

func TestRunCheckUsesInstalledVersion(t *testing.T) {
	root := newUpgradeTestProject(t)

	result, err := Run(context.Background(), Options{
		CurrentVersion: "v1.2.3",
		Check:          true,
		WorkingDir:     root,
	})
	if err != nil {
		t.Fatalf("Run(check) error = %v, want nil", err)
	}
	if result.TargetVersion != "v1.2.3" || result.Project == nil {
		t.Fatalf("Run(check) result = %+v, want installed-version project check", result)
	}
	if result.Project.CurrentVersion != "v0.0.0" || !result.Project.WouldUpdate {
		t.Fatalf("Project result = %+v, want available update", result.Project)
	}
}

func TestRunDryRunInsideProjectPlansProject(t *testing.T) {
	root := newUpgradeTestProject(t)

	result, err := Run(context.Background(), Options{
		CurrentVersion: "v1.0.0",
		TargetVersion:  "v1.2.3",
		DryRun:         true,
		WorkingDir:     root,
	})
	if err != nil {
		t.Fatalf("Run(dry-run project) error = %v, want nil", err)
	}
	if result.Project == nil {
		t.Fatalf("Run(dry-run project) result = %+v, want project plan", result)
	}
	if result.Project.CurrentVersion != "v0.0.0" || result.Project.TargetVersion != "v1.2.3" {
		t.Fatalf("Project result = %+v, want current and target versions", result.Project)
	}
}

func TestRunUpgradeNoOpsWhenProjectVersionMatchesTarget(t *testing.T) {
	root := newUpgradeTestProject(t)
	calledRunner := false
	calledConfirm := false

	result, err := Run(context.Background(), Options{
		CurrentVersion: "v0.0.0",
		WorkingDir:     root,
		CommandRunner: func(context.Context, string, string, ...string) ([]byte, error) {
			calledRunner = true
			return nil, nil
		},
		Confirm: func(context.Context, string) (bool, error) {
			calledConfirm = true
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("Run(upgrade no-op) error = %v, want nil", err)
	}
	if result.Project == nil || result.Project.WouldUpdate || result.Project.Updated {
		t.Fatalf("Run(upgrade no-op) result = %+v, want current project", result)
	}
	if calledRunner || calledConfirm {
		t.Fatalf("Run(upgrade no-op) called runner=%v confirm=%v, want neither", calledRunner, calledConfirm)
	}
	if got := strings.Join(result.Lines, "\n"); !strings.Contains(got, "project: current") {
		t.Fatalf("Run(upgrade no-op) lines = %#v, want current output", result.Lines)
	}
}

func TestRunOutsideProjectFails(t *testing.T) {
	_, err := Run(context.Background(), Options{
		CurrentVersion: "v1.2.3",
		WorkingDir:     t.TempDir(),
	})
	if err == nil {
		t.Fatal("Run(outside project) error = nil, want error")
	}
	if got := err.Error(); got != "dygo upgrade requires a generated dygo project" {
		t.Fatalf("Run(outside project) error = %q, want project requirement", got)
	}
}

func TestRunDevBinaryRequiresExplicitTarget(t *testing.T) {
	root := newUpgradeTestProject(t)

	_, err := Run(context.Background(), Options{
		CurrentVersion: "dev",
		WorkingDir:     root,
	})
	if err == nil {
		t.Fatal("Run(dev upgrade) error = nil, want explicit target error")
	}
	if got := err.Error(); got != "dygo upgrade requires --to when running an unreleased dev binary" {
		t.Fatalf("Run(dev upgrade) error = %q, want explicit target", got)
	}
}

func TestRunUpgradePromptsBeforeApply(t *testing.T) {
	root := newUpgradeTestProject(t)
	calledRunner := false

	_, err := Run(context.Background(), Options{
		CurrentVersion: "v1.2.3",
		WorkingDir:     root,
		CommandRunner: func(context.Context, string, string, ...string) ([]byte, error) {
			calledRunner = true
			return nil, nil
		},
		Confirm: func(context.Context, string) (bool, error) {
			return false, nil
		},
	})
	if err == nil {
		t.Fatal("Run(upgrade cancelled) error = nil, want cancellation")
	}
	if got := err.Error(); got != "project upgrade cancelled" {
		t.Fatalf("Run(upgrade cancelled) error = %q, want cancellation", got)
	}
	if calledRunner {
		t.Fatal("CommandRunner called after declined prompt")
	}
}

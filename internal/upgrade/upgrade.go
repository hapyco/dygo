// Package upgrade upgrades generated dygo projects to a target dygo release.
package upgrade

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hapyco/dygo/internal/project"
)

const (
	ModulePath = "github.com/hapyco/dygo"
)

// CommandRunner runs an external command in a directory.
type CommandRunner func(context.Context, string, string, ...string) ([]byte, error)

// Confirmer asks for confirmation before a risky but non-destructive operation.
type Confirmer func(context.Context, string) (bool, error)

// Options configures an upgrade run.
type Options struct {
	CurrentVersion string
	TargetVersion  string
	WorkingDir     string

	Check  bool
	DryRun bool
	Yes    bool

	CommandRunner CommandRunner
	Confirm       Confirmer

	// SkipTidy is for tests; the public CLI always runs tidy for project upgrades.
	SkipTidy bool
}

// Result describes the planned or completed upgrade.
type Result struct {
	CurrentVersion string
	TargetVersion  string
	ProjectContext ProjectContext

	Project *ProjectResult

	Warnings []string
	Lines    []string
}

// ProjectContext describes where upgrade was invoked.
type ProjectContext struct {
	Root      string
	Marker    string
	Generated bool
	Framework bool
}

// ProjectResult describes project upgrade work.
type ProjectResult struct {
	Root           string
	CurrentVersion string
	TargetVersion  string

	WouldUpdate   bool
	Updated       bool
	RunnerUpdated bool
	StudioUpdated bool
	StudioSource  string
	NoGit         bool
}

// Run resolves and executes an upgrade.
func Run(ctx context.Context, options Options) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("context is required")
	}

	workingDir := strings.TrimSpace(options.WorkingDir)
	if workingDir == "" {
		workingDir = "."
	}
	workingDir, err := filepath.Abs(workingDir)
	if err != nil {
		return Result{}, fmt.Errorf("resolve working directory: %w", err)
	}

	currentVersion := strings.TrimSpace(options.CurrentVersion)
	if currentVersion == "" {
		currentVersion = "dev"
	}
	targetVersion, err := upgradeTargetVersion(options.TargetVersion, currentVersion)
	if err != nil {
		return Result{}, err
	}

	projectContext := discoverProjectContext(workingDir)
	if !projectContext.Generated {
		return Result{}, fmt.Errorf("dygo upgrade requires a generated dygo project")
	}

	result := Result{
		CurrentVersion: currentVersion,
		TargetVersion:  targetVersion,
		ProjectContext: projectContext,
	}

	if options.Check {
		projectResult, err := CheckProject(projectContext.Root, targetVersion)
		if err != nil {
			return Result{}, err
		}
		result.Project = &projectResult
		result.Lines = resultLines(result, options)
		return result, nil
	}

	projectResult, err := PlanProject(projectContext.Root, targetVersion)
	if err != nil {
		return Result{}, err
	}
	result.Project = &projectResult
	if !projectResult.WouldUpdate {
		result.Lines = resultLines(result, options)
		return result, nil
	}
	if !options.DryRun {
		if !options.Yes {
			if options.Confirm == nil {
				return Result{}, fmt.Errorf("project upgrade requires confirmation; rerun with --yes to apply without prompting")
			}
			ok, err := options.Confirm(ctx, "Apply project upgrade?")
			if err != nil {
				return Result{}, err
			}
			if !ok {
				return Result{}, fmt.Errorf("project upgrade cancelled")
			}
		}
		updated, err := UpgradeProject(ctx, ProjectOptions{
			Root:          projectContext.Root,
			TargetVersion: targetVersion,
			Yes:           true,
			CommandRunner: options.CommandRunner,
			Confirm:       options.Confirm,
			SkipTidy:      options.SkipTidy,
		})
		if err != nil {
			return Result{}, err
		}
		result.Project = &updated
	}

	result.Lines = resultLines(result, options)
	return result, nil
}

func upgradeTargetVersion(targetVersion string, currentVersion string) (string, error) {
	targetVersion = strings.TrimSpace(targetVersion)
	if targetVersion == "" {
		if currentVersion == "dev" {
			return "", fmt.Errorf("dygo upgrade requires --to when running an unreleased dev binary")
		}
		return currentVersion, nil
	}
	return normalizeVersion(targetVersion), nil
}

func discoverProjectContext(workingDir string) ProjectContext {
	root, err := project.DiscoverRoot(workingDir)
	if err != nil {
		return ProjectContext{}
	}
	context := ProjectContext{Root: root.Path, Marker: root.Marker}
	switch root.Marker {
	case project.MarkerFile:
		context.Generated = true
	case "framework-repo":
		context.Framework = true
	}
	return context
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func resultLines(result Result, options Options) []string {
	mode := "upgrade"
	if options.Check {
		mode = "upgrade check"
	} else if options.DryRun {
		mode = "upgrade dry run"
	}
	lines := []string{
		fmt.Sprintf("%s target: %s", mode, result.TargetVersion),
		fmt.Sprintf("installed dygo: %s", result.CurrentVersion),
	}
	if result.Project != nil {
		action := "would update"
		if result.Project.Updated {
			action = "updated"
		} else if !result.Project.WouldUpdate {
			action = "current"
		}
		lines = append(lines, fmt.Sprintf("project: %s %s from %s to %s", action, result.Project.Root, result.Project.CurrentVersion, result.Project.TargetVersion))
		if result.Project.RunnerUpdated {
			lines = append(lines, "project runner: updated")
		}
		if result.Project.StudioUpdated {
			lines = append(lines, fmt.Sprintf("project Studio cache: updated from %s", result.Project.StudioSource))
		}
	}
	return lines
}

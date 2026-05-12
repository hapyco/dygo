// Package upgrade upgrades dygo CLI binaries and generated dygo projects.
package upgrade

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dygo-dev/dygo/internal/project"
)

const (
	ModulePath        = "github.com/dygo-dev/dygo"
	DefaultAPIBaseURL = "https://api.github.com/repos/dygo-dev/dygo"
	DefaultInstallDir = "~/.dygo/bin"
)

// CommandRunner runs an external command in a directory.
type CommandRunner func(context.Context, string, string, ...string) ([]byte, error)

// Confirmer asks for confirmation before a risky but non-destructive operation.
type Confirmer func(context.Context, string) (bool, error)

// Options configures an upgrade run.
type Options struct {
	CurrentVersion string
	TargetVersion  string
	InstallDir     string
	WorkingDir     string
	ExecutablePath string

	Check       bool
	DryRun      bool
	Yes         bool
	CLIOnly     bool
	ProjectOnly bool

	GOOS       string
	GOARCH     string
	APIBaseURL string
	HTTPClient *http.Client

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

	CLI     *CLIResult
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

// CLIResult describes CLI upgrade work.
type CLIResult struct {
	CurrentVersion string
	TargetVersion  string
	InstallDir     string
	BinaryPath     string

	WouldInstall bool
	Installed    bool
	Checked      bool
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
	if options.CLIOnly && options.ProjectOnly {
		return Result{}, fmt.Errorf("--cli-only and --project-only cannot be used together")
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
	installDir := strings.TrimSpace(options.InstallDir)
	if installDir == "" {
		installDir = DefaultInstallDir
	}
	installDir, err = expandPath(installDir)
	if err != nil {
		return Result{}, err
	}

	projectContext := discoverProjectContext(workingDir)
	upgradeCLI, upgradeProject, err := upgradeModes(options, projectContext)
	if err != nil {
		return Result{}, err
	}

	client := NewGitHubClient(ClientOptions{
		BaseURL:    firstNonEmpty(options.APIBaseURL, DefaultAPIBaseURL),
		HTTPClient: options.HTTPClient,
	})
	release, err := resolveRelease(ctx, client, options.TargetVersion)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		CurrentVersion: currentVersion,
		TargetVersion:  release.TagName,
		ProjectContext: projectContext,
	}

	goos := firstNonEmpty(options.GOOS, runtime.GOOS)
	goarch := firstNonEmpty(options.GOARCH, runtime.GOARCH)
	if upgradeCLI {
		assetName, err := releaseAssetName(release.TagName, goos, goarch)
		if err != nil {
			return Result{}, err
		}
		if _, ok := release.Asset(assetName); !ok {
			return Result{}, fmt.Errorf("release %s does not contain asset %s", release.TagName, assetName)
		}
		if _, ok := release.Asset("checksums.txt"); !ok {
			return Result{}, fmt.Errorf("release %s does not contain checksums.txt", release.TagName)
		}
		cli := &CLIResult{
			CurrentVersion: currentVersion,
			TargetVersion:  release.TagName,
			InstallDir:     installDir,
			BinaryPath:     filepath.Join(installDir, executableName(goos)),
			WouldInstall:   true,
			Checked:        options.Check,
		}
		result.CLI = cli
		if warning := binaryPathWarning(options.ExecutablePath, installDir); warning != "" {
			result.Warnings = append(result.Warnings, warning)
		}
		if !options.Check && !options.DryRun {
			if err := InstallCLI(ctx, InstallOptions{
				Release:    release,
				InstallDir: installDir,
				GOOS:       goos,
				GOARCH:     goarch,
				HTTPClient: options.HTTPClient,
			}); err != nil {
				return Result{}, err
			}
			cli.Installed = true
		}
	}

	if upgradeProject {
		projectResult, err := PlanProject(projectContext.Root, release.TagName)
		if err != nil {
			return Result{}, err
		}
		result.Project = &projectResult
		if !options.Check && !options.DryRun {
			updated, err := UpgradeProject(ctx, ProjectOptions{
				Root:          projectContext.Root,
				TargetVersion: release.TagName,
				Yes:           options.Yes,
				CommandRunner: options.CommandRunner,
				Confirm:       options.Confirm,
				SkipTidy:      options.SkipTidy,
			})
			if err != nil {
				return Result{}, err
			}
			result.Project = &updated
		}
	}

	result.Lines = resultLines(result, options)
	return result, nil
}

func resolveRelease(ctx context.Context, client GitHubClient, targetVersion string) (Release, error) {
	targetVersion = strings.TrimSpace(targetVersion)
	if targetVersion == "" {
		return client.LatestRelease(ctx)
	}
	return client.ReleaseByTag(ctx, normalizeVersion(targetVersion))
}

func upgradeModes(options Options, context ProjectContext) (bool, bool, error) {
	if options.CLIOnly {
		return true, false, nil
	}
	if options.ProjectOnly {
		if !context.Generated {
			return false, false, fmt.Errorf("--project-only requires a generated dygo project")
		}
		return false, true, nil
	}
	if context.Generated {
		return true, true, nil
	}
	return true, false, nil
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

func expandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, path[2:])
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve install directory: %w", err)
	}
	return absolute, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func executableName(goos string) string {
	if goos == "windows" {
		return "dygo.exe"
	}
	return "dygo"
}

func binaryPathWarning(executablePath string, installDir string) string {
	executablePath = strings.TrimSpace(executablePath)
	if executablePath == "" {
		path, err := os.Executable()
		if err != nil {
			return ""
		}
		executablePath = path
	}
	executablePath, err := filepath.Abs(executablePath)
	if err != nil {
		return ""
	}
	installDir, err = filepath.Abs(installDir)
	if err != nil {
		return ""
	}
	relative, err := filepath.Rel(installDir, executablePath)
	if err == nil && relative != "." && !strings.HasPrefix(relative, "..") {
		return ""
	}
	if err == nil && relative == "." {
		return ""
	}
	return fmt.Sprintf("current dygo binary is %s; upgraded binary will be installed under %s, so PATH may still point elsewhere", executablePath, installDir)
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
		fmt.Sprintf("current cli: %s", result.CurrentVersion),
	}
	if result.CLI != nil {
		action := "would install"
		if result.CLI.Installed {
			action = "installed"
		} else if result.CLI.Checked {
			action = "available"
		}
		lines = append(lines, fmt.Sprintf("cli: %s %s to %s", action, result.CLI.TargetVersion, result.CLI.BinaryPath))
	}
	if result.Project != nil {
		action := "would update"
		if result.Project.Updated {
			action = "updated"
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

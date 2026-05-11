package upgrade

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dygo-dev/dygo/internal/hookgen"
)

// ProjectOptions configures a project upgrade.
type ProjectOptions struct {
	Root          string
	TargetVersion string
	Yes           bool
	CommandRunner CommandRunner
	Confirm       Confirmer
	SkipTidy      bool
}

// PlanProject describes project upgrade work without writing files.
func PlanProject(root string, targetVersion string) (ProjectResult, error) {
	root = filepath.Clean(root)
	current, err := ReadProjectVersion(root)
	if err != nil {
		return ProjectResult{}, err
	}
	if _, err := hookgen.RenderRunner(root); err != nil {
		return ProjectResult{}, fmt.Errorf("render project runner: %w", err)
	}
	return ProjectResult{
		Root:           root,
		CurrentVersion: current,
		TargetVersion:  targetVersion,
		WouldUpdate:    true,
	}, nil
}

// UpgradeProject updates the current project dependency and dygo-managed files.
func UpgradeProject(ctx context.Context, options ProjectOptions) (ProjectResult, error) {
	if ctx == nil {
		return ProjectResult{}, fmt.Errorf("context is required")
	}
	root := filepath.Clean(options.Root)
	result, err := PlanProject(root, options.TargetVersion)
	if err != nil {
		return ProjectResult{}, err
	}

	runner := options.CommandRunner
	if runner == nil {
		runner = defaultCommandRunner
	}
	git, err := gitState(ctx, root, runner)
	if err != nil {
		return ProjectResult{}, err
	}
	result.NoGit = !git.InsideWorkTree
	if git.InsideWorkTree && git.Dirty {
		return ProjectResult{}, fmt.Errorf("project worktree is dirty; commit or stash changes before running dygo upgrade")
	}
	if !git.InsideWorkTree && !options.Yes {
		if options.Confirm == nil {
			return ProjectResult{}, fmt.Errorf("project is not inside a git worktree; rerun with --yes to upgrade without git safety checks")
		}
		ok, err := options.Confirm(ctx, "Project is not inside a git worktree. Continue with project upgrade?")
		if err != nil {
			return ProjectResult{}, err
		}
		if !ok {
			return ProjectResult{}, fmt.Errorf("project upgrade cancelled")
		}
	}

	if _, err := runner(ctx, root, "go", "mod", "edit", "-dropreplace="+ModulePath); err != nil {
		return ProjectResult{}, fmt.Errorf("drop dygo replace directive: %w", err)
	}
	if _, err := runner(ctx, root, "go", "mod", "edit", "-require="+ModulePath+"@"+options.TargetVersion); err != nil {
		return ProjectResult{}, fmt.Errorf("update dygo module requirement: %w", err)
	}
	update, written, err := hookgen.UpdateRunner(root)
	if err != nil {
		return ProjectResult{}, fmt.Errorf("update project runner: %w", err)
	}
	_ = update
	if !options.SkipTidy {
		if _, err := runner(ctx, root, "go", "mod", "tidy"); err != nil {
			return ProjectResult{}, fmt.Errorf("run go mod tidy: %w", err)
		}
	}
	result.Updated = true
	result.RunnerUpdated = written
	return result, nil
}

// ReadProjectVersion reads the dygo module version from go.mod.
func ReadProjectVersion(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	inRequireBlock := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if inRequireBlock {
			if line == ")" {
				inRequireBlock = false
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == ModulePath {
				return fields[1], nil
			}
			continue
		}
		if line == "require (" {
			inRequireBlock = true
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[0] == "require" && fields[1] == ModulePath {
			return fields[2], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan go.mod: %w", err)
	}
	return "", fmt.Errorf("go.mod does not require %s", ModulePath)
}

type gitStatus struct {
	InsideWorkTree bool
	Dirty          bool
}

func gitState(ctx context.Context, root string, runner CommandRunner) (gitStatus, error) {
	if _, err := runner(ctx, root, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return gitStatus{}, nil
	}
	output, err := runner(ctx, root, "git", "status", "--porcelain", "--", ".")
	if err != nil {
		return gitStatus{}, fmt.Errorf("check git status: %w", err)
	}
	return gitStatus{InsideWorkTree: true, Dirty: strings.TrimSpace(string(output)) != ""}, nil
}

func defaultCommandRunner(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return output, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
		return output, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return output, nil
}

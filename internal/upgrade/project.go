package upgrade

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hapyco/dygo/internal/frameworkapp"
	"github.com/hapyco/dygo/internal/hookgen"
	"github.com/hapyco/dygo/internal/studio"
)

// ProjectOptions configures a project upgrade.
type ProjectOptions struct {
	Root          string
	TargetVersion string
	Yes           bool
	CommandRunner CommandRunner
	Confirm       Confirmer
	SkipTidy      bool

	// StudioAssets is for tests and custom upgrade callers. Normal upgrades use
	// bundled release assets from the running dygo binary.
	StudioAssets fs.FS
	// CoreAssets is for tests and custom upgrade callers. Normal upgrades use
	// the Core App bundled into the running dygo binary.
	CoreAssets fs.FS
}

var updateProjectRunner = hookgen.UpdateRunner

// PlanProject describes project upgrade work without writing files.
func PlanProject(root string, targetVersion string) (ProjectResult, error) {
	root = filepath.Clean(root)
	current, err := ReadProjectVersion(root)
	if err != nil {
		return ProjectResult{}, err
	}
	wouldUpdate := current != targetVersion
	if !wouldUpdate {
		return ProjectResult{
			Root:           root,
			CurrentVersion: current,
			TargetVersion:  targetVersion,
			WouldUpdate:    false,
		}, nil
	}
	if _, err := hookgen.RenderRunner(root); err != nil {
		return ProjectResult{}, fmt.Errorf("render project runner: %w", err)
	}
	return ProjectResult{
		Root:                      root,
		CurrentVersion:            current,
		TargetVersion:             targetVersion,
		WouldUpdate:               wouldUpdate,
		MetadataMigrationRequired: wouldUpdate,
	}, nil
}

// CheckProject compares the current project dependency with a target dygo release.
func CheckProject(root string, targetVersion string) (ProjectResult, error) {
	root = filepath.Clean(root)
	current, err := ReadProjectVersion(root)
	if err != nil {
		return ProjectResult{}, err
	}
	return ProjectResult{
		Root:                      root,
		CurrentVersion:            current,
		TargetVersion:             targetVersion,
		WouldUpdate:               current != targetVersion,
		MetadataMigrationRequired: current != targetVersion,
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
	if !result.WouldUpdate {
		return result, nil
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

	goModPath := filepath.Join(root, "go.mod")
	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		return ProjectResult{}, fmt.Errorf("read go.mod: %w", err)
	}
	goSumPath := filepath.Join(root, "go.sum")
	goSum, goSumErr := os.ReadFile(goSumPath)
	goSumExists := goSumErr == nil
	if goSumErr != nil && !os.IsNotExist(goSumErr) {
		return ProjectResult{}, fmt.Errorf("read go.sum: %w", goSumErr)
	}
	restoreModuleFiles := func() error {
		if err := os.WriteFile(goModPath, goMod, 0o644); err != nil {
			return fmt.Errorf("restore go.mod after failed upgrade: %w", err)
		}
		if goSumExists {
			if err := os.WriteFile(goSumPath, goSum, 0o644); err != nil {
				return fmt.Errorf("restore go.sum after failed upgrade: %w", err)
			}
		} else if err := os.Remove(goSumPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove go.sum after failed upgrade: %w", err)
		}
		return nil
	}
	runnerPath := filepath.Join(root, "cmd", "dygo", "main.go")
	runnerSource, runnerReadErr := os.ReadFile(runnerPath)
	runnerExists := runnerReadErr == nil
	if runnerReadErr != nil && !os.IsNotExist(runnerReadErr) {
		return ProjectResult{}, fmt.Errorf("read generated runner: %w", runnerReadErr)
	}
	restoreRunner := func() error {
		if runnerExists {
			if err := os.WriteFile(runnerPath, runnerSource, 0o644); err != nil {
				return fmt.Errorf("restore generated runner after failed upgrade: %w", err)
			}
			return nil
		}
		if err := os.Remove(runnerPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove generated runner after failed upgrade: %w", err)
		}
		return nil
	}
	gitignorePath := filepath.Join(root, ".gitignore")
	gitignore, gitignoreErr := os.ReadFile(gitignorePath)
	gitignoreExists := gitignoreErr == nil
	if gitignoreErr != nil && !os.IsNotExist(gitignoreErr) {
		return ProjectResult{}, fmt.Errorf("read .gitignore: %w", gitignoreErr)
	}
	restoreGitignore := func() error {
		if gitignoreExists {
			if err := os.WriteFile(gitignorePath, gitignore, 0o644); err != nil {
				return fmt.Errorf("restore .gitignore after failed upgrade: %w", err)
			}
			return nil
		}
		if err := os.Remove(gitignorePath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove .gitignore after failed upgrade: %w", err)
		}
		return nil
	}
	studioStage, err := stageUpgradePath(filepath.Join(root, ".dygo", "apps", "studio"), ".dygo-upgrade-studio")
	if err != nil {
		return ProjectResult{}, err
	}
	coreStage, err := stageUpgradePath(filepath.Join(root, filepath.FromSlash(frameworkapp.CoreProjectPath)), ".dygo-upgrade-core")
	if err != nil {
		_ = studioStage.restore()
		return ProjectResult{}, err
	}
	failed := func(cause error) (ProjectResult, error) {
		restoreErr := errors.Join(restoreModuleFiles(), restoreRunner(), restoreGitignore(), coreStage.restore(), studioStage.restore())
		if restoreErr != nil {
			return ProjectResult{}, fmt.Errorf("%w; restore failed: %v", cause, restoreErr)
		}
		return ProjectResult{}, cause
	}

	if _, err := runner(ctx, root, "go", "mod", "edit", "-dropreplace="+ModulePath); err != nil {
		return failed(fmt.Errorf("drop dygo replace directive: %w", err))
	}
	if _, err := runner(ctx, root, "go", "mod", "edit", "-require="+ModulePath+"@"+options.TargetVersion); err != nil {
		return failed(fmt.Errorf("update dygo module requirement: %w", err))
	}
	studioSource, err := installStudioCache(root, options.StudioAssets)
	if err != nil {
		return failed(err)
	}
	coreSource, err := installCoreCache(root, options.CoreAssets)
	if err != nil {
		return failed(err)
	}
	if err := ensureStudioMetadataTracked(root); err != nil {
		return failed(err)
	}
	_, written, err := updateProjectRunner(root)
	if err != nil {
		return failed(fmt.Errorf("update project runner: %w", err))
	}
	if !options.SkipTidy {
		if _, err := runner(ctx, root, "go", "mod", "tidy"); err != nil {
			return failed(fmt.Errorf("run go mod tidy: %w", err))
		}
	}
	coreStage.commit()
	studioStage.commit()
	result.Updated = true
	result.RunnerUpdated = written
	result.CoreUpdated = true
	result.CoreSource = coreSource
	result.StudioSource = studioSource
	result.MetadataMigrationRequired = true
	return result, nil
}

func ensureStudioMetadataTracked(root string) error {
	path := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read .gitignore: %w", err)
	}
	content := string(data)
	required := []string{
		"!.dygo/apps/studio/",
		"!.dygo/apps/studio/**",
		".dygo/apps/studio/ui/",
		".dygo/apps/studio/ui/**",
	}
	missing := make([]string, 0, len(required))
	for _, line := range required {
		found := false
		for _, existing := range strings.Split(content, "\n") {
			if strings.TrimSpace(existing) == line {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, line)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	content = strings.TrimRight(content, "\n")
	if content != "" {
		content += "\n\n"
	}
	content += "# dygo Studio metadata (keep metadata, ignore built UI)\n" + strings.Join(missing, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("update .gitignore for Studio metadata: %w", err)
	}
	return nil
}

type stagedUpgradePath struct {
	path    string
	backup  string
	existed bool
}

func stageUpgradePath(path string, prefix string) (stagedUpgradePath, error) {
	if _, err := os.Lstat(path); err != nil {
		if os.IsNotExist(err) {
			return stagedUpgradePath{path: path}, nil
		}
		return stagedUpgradePath{}, fmt.Errorf("stat managed upgrade path %s: %w", path, err)
	}
	parent := filepath.Dir(path)
	backup, err := os.MkdirTemp(filepath.Dir(parent), prefix+"-")
	if err != nil {
		return stagedUpgradePath{}, fmt.Errorf("create managed upgrade backup: %w", err)
	}
	if err := os.Remove(backup); err != nil {
		return stagedUpgradePath{}, fmt.Errorf("prepare managed upgrade backup: %w", err)
	}
	if err := os.Rename(path, backup); err != nil {
		return stagedUpgradePath{}, fmt.Errorf("stage managed upgrade path %s: %w", path, err)
	}
	return stagedUpgradePath{path: path, backup: backup, existed: true}, nil
}

func (s *stagedUpgradePath) restore() error {
	if s.path == "" {
		return nil
	}
	if s.backup == "" {
		if s.existed {
			return nil
		}
		if err := os.RemoveAll(s.path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove created managed upgrade path %s: %w", s.path, err)
		}
		return nil
	}
	if _, err := os.Lstat(s.path); err == nil {
		if err := os.RemoveAll(s.path); err != nil {
			return fmt.Errorf("remove replacement %s: %w", s.path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat replacement %s: %w", s.path, err)
	}
	if err := os.Rename(s.backup, s.path); err != nil {
		return fmt.Errorf("restore managed upgrade path %s: %w", s.path, err)
	}
	s.backup = ""
	return nil
}

func (s *stagedUpgradePath) commit() {
	if s.backup != "" {
		_ = os.RemoveAll(s.backup)
		s.backup = ""
	}
}

func installCoreCache(root string, configured fs.FS) (string, error) {
	sources := make([]frameworkapp.Source, 0, 2)
	if configured != nil {
		sources = append(sources, frameworkapp.Source{Name: "configured Core App", FS: configured})
	}
	embedded, err := frameworkapp.EmbeddedCoreSource()
	if err != nil {
		return "", err
	}
	sources = append(sources, embedded)
	name, err := frameworkapp.InstallCore(root, sources...)
	if err != nil {
		return "", fmt.Errorf("install Core App: %w", err)
	}
	return name, nil
}

func installStudioCache(root string, configured fs.FS) (string, error) {
	appSource, err := studio.EmbeddedAppSource()
	if err != nil {
		return "", err
	}
	assetSources := make([]studio.Source, 0, 2)
	if configured != nil {
		assetSources = append(assetSources, studio.Source{Name: "configured Studio assets", FS: configured})
	}
	source, ok, err := studio.EmbeddedSource()
	if err != nil {
		return "", err
	}
	if ok {
		assetSources = append(assetSources, source)
	}
	name, err := studio.InstallApp(root, []studio.AppSource{appSource}, assetSources)
	if err != nil {
		return "", fmt.Errorf("install Studio App: %w", err)
	}
	return name, nil
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

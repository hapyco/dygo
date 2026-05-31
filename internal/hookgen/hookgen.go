// Package hookgen generates Entity hook scaffolds and project runner wiring.
package hookgen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/runnergen"
	"github.com/hapyco/dygo/internal/shape"
)

// Result describes files touched by hook scaffold generation.
type Result struct {
	AppName string
	Entity  string

	HookFile   string
	RunnerFile string

	HookFileStatus   string
	RunnerFileStatus string

	HookFileCreated   bool
	RunnerFileWritten bool
}

// HookFile describes one discovered Entity hook file.
type HookFile = runnergen.HookFile

// RunnerUpdate describes a generated project runner update.
type RunnerUpdate = runnergen.RunnerUpdate

// GenerateOptions controls Entity hook scaffold generation.
type GenerateOptions struct {
	Root       string
	AppName    string
	EntityName string
	DryRun     bool
	Force      bool
}

// Generate creates an Entity hook scaffold and updates generated runner wiring.
func Generate(root string, appName string, entityName string) (Result, error) {
	return GenerateWithOptions(GenerateOptions{Root: root, AppName: appName, EntityName: entityName})
}

// GenerateWithOptions creates or previews an Entity hook scaffold and runner wiring.
func GenerateWithOptions(options GenerateOptions) (Result, error) {
	root := filepath.Clean(options.Root)
	appName := strings.TrimSpace(options.AppName)
	entityName := strings.TrimSpace(options.EntityName)
	if appName == "" {
		return Result{}, fmt.Errorf("app name is required")
	}
	if entityName == "" {
		return Result{}, fmt.Errorf("entity name is required")
	}
	if err := runnergen.RequireGeneratedProjectRoot(root); err != nil {
		return Result{}, err
	}

	modulePath, err := runnergen.ReadModulePath(root)
	if err != nil {
		return Result{}, err
	}
	metadata, err := project.LoadMetadata(root)
	if err != nil {
		return Result{}, err
	}

	app, ok := findApp(metadata.Apps, appName)
	if !ok {
		return Result{}, fmt.Errorf("app %q not found", appName)
	}
	if !runnergen.IsProjectOwnedApp(root, app.Dir) {
		return Result{}, fmt.Errorf("app %q is not a project-owned app under apps/", appName)
	}
	entity, ok := findEntity(metadata.Entities, appName, entityName)
	if !ok {
		return Result{}, fmt.Errorf("entity %q not found in app %q", entityName, appName)
	}
	if entity.IsCollection() {
		return Result{}, fmt.Errorf("entity %q in app %q is a collection; generate hooks for the parent Entity that owns collection row usage", entityName, appName)
	}

	entityDir := filepath.Dir(entity.Path)
	hookFile := filepath.Join(entityDir, shape.EntityHooksFile)
	runnerFile := filepath.Join(root, "cmd", "dygo", "main.go")

	if err := preflightPath(entityDir, wantDirectory); err != nil {
		return Result{}, err
	}
	if err := preflightPath(hookFile, wantRegularFile); err != nil {
		return Result{}, err
	}
	if exists, hasRegister, err := runnergen.InspectFunctionFile(hookFile, "Register"); err != nil {
		return Result{}, err
	} else if exists && !hasRegister {
		return Result{}, fmt.Errorf("%s exists but does not expose Register(registry dygo.RecordHookRegistry) error", hookFile)
	}
	if err := runnergen.PreflightGeneratedFile(runnerFile, runnergen.HookManualSnippet(root, modulePath, appName, entityName, entityDir)); err != nil {
		return Result{}, err
	}

	hookSource, err := renderEntityHookSource(appName, entityName)
	if err != nil {
		return Result{}, err
	}
	runnerUpdate, err := runnergen.Render(root, runnergen.RenderOptions{HookTarget: runnergen.HookTarget{AppName: appName, EntityName: entityName}})
	if err != nil {
		return Result{}, err
	}

	hookStatus, err := hookFileStatus(hookFile, options.DryRun)
	if err != nil {
		return Result{}, err
	}
	runnerStatus, err := runnergen.GeneratedFileStatus(runnerFile, runnerUpdate.Source, options.DryRun)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		AppName:          appName,
		Entity:           entityName,
		HookFile:         hookFile,
		RunnerFile:       runnerFile,
		HookFileStatus:   hookStatus,
		RunnerFileStatus: runnerStatus,
		HookFileCreated:  hookStatus == "created",
		RunnerFileWritten: runnerStatus == "created" ||
			runnerStatus == "updated",
	}
	if options.DryRun {
		return result, nil
	}
	if err := os.MkdirAll(entityDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create Entity directory %s: %w", entityDir, err)
	}
	if hookStatus == "created" || hookStatus == "updated" {
		if err := os.WriteFile(hookFile, hookSource, 0o644); err != nil {
			return Result{}, fmt.Errorf("write hook file %s: %w", hookFile, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(runnerFile), 0o755); err != nil {
		return Result{}, fmt.Errorf("create runner directory %s: %w", filepath.Dir(runnerFile), err)
	}
	written, err := runnergen.WriteFileIfChanged(runnerFile, runnerUpdate.Source)
	if err != nil {
		return Result{}, err
	}
	result.RunnerFileWritten = written
	if result.RunnerFileStatus == "" {
		result.RunnerFileStatus = writeStatus(written)
	}
	return result, nil
}

// RenderRunner renders the project runner for all app hooks that expose Register.
func RenderRunner(root string) (RunnerUpdate, error) {
	return runnergen.Render(root, runnergen.RenderOptions{})
}

// UpdateRunner writes the generated project runner when its content changes.
func UpdateRunner(root string) (RunnerUpdate, bool, error) {
	return runnergen.Update(root)
}

// Discover returns Entity hook files found in canonical Entity bundles.
func Discover(root string) ([]HookFile, error) {
	// TODO: include compiled hook registrations after project runners expose a
	// cheap introspection mode that does not start the server.
	return runnergen.DiscoverHooks(root)
}

// Validate reports static hook convention and runner wiring problems.
func Validate(root string) ([]string, error) {
	root = filepath.Clean(root)
	hookFiles, err := Discover(root)
	if err != nil {
		return nil, err
	}
	var problems []string
	for _, hook := range hookFiles {
		if !hook.HasRegister {
			problems = append(problems, fmt.Sprintf("%s: hook file must expose Register(registry dygo.RecordHookRegistry) error", filepath.ToSlash(hook.Path)))
		}
	}
	jobFiles, err := runnergen.DiscoverJobs(root)
	if err != nil {
		return nil, err
	}
	for _, job := range jobFiles {
		if !job.HasRun {
			problems = append(problems, fmt.Sprintf("%s: job runner file must expose Run(ctx context.Context, job dygo.JobExecution) error", filepath.ToSlash(job.Path)))
		}
	}
	update, err := RenderRunner(root)
	if err != nil {
		problems = append(problems, err.Error())
		return problems, nil
	}
	current, err := os.ReadFile(update.RunnerFile)
	if err != nil {
		if os.IsNotExist(err) {
			problems = append(problems, fmt.Sprintf("%s is missing; run dygo hook sync", filepath.ToSlash(update.RunnerFile)))
			return problems, nil
		}
		return nil, fmt.Errorf("read runner file %s: %w", update.RunnerFile, err)
	}
	if !bytes.Equal(current, update.Source) {
		problems = append(problems, fmt.Sprintf("%s is out of date; run dygo hook sync", filepath.ToSlash(update.RunnerFile)))
	}
	return problems, nil
}

func findApp(apps []manifest.LoadedApp, appName string) (manifest.LoadedApp, bool) {
	for _, app := range apps {
		if app.Manifest.Name == appName {
			return app, true
		}
	}
	return manifest.LoadedApp{}, false
}

func findEntity(entities []catalog.LoadedEntity, appName string, entityName string) (catalog.LoadedEntity, bool) {
	for _, entity := range entities {
		if entity.AppName == appName && entity.Entity.Name == entityName {
			return entity, true
		}
	}
	return catalog.LoadedEntity{}, false
}

type pathExpectation int

const (
	wantDirectory pathExpectation = iota
	wantRegularFile
)

func preflightPath(path string, want pathExpectation) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	switch want {
	case wantDirectory:
		if !info.IsDir() {
			return fmt.Errorf("%s must be a directory", path)
		}
	case wantRegularFile:
		if info.IsDir() || !info.Mode().IsRegular() {
			return fmt.Errorf("%s must be a regular file", path)
		}
	}
	return nil
}

func hookFileStatus(path string, dryRun bool) (string, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if dryRun {
				return "would create", nil
			}
			return "created", nil
		}
		return "", fmt.Errorf("stat hook file %s: %w", path, err)
	}
	return "existing", nil
}

func renderEntityHookSource(appName string, entityName string) ([]byte, error) {
	name := runnergen.ExportedIdentifier(entityName)
	source := fmt.Sprintf(`package hooks

import (
	"context"

	"github.com/hapyco/dygo/pkg/dygo"
)

func Register(registry dygo.RecordHookRegistry) error {
	if err := registry.RegisterEntity(%[2]q, %[3]q, dygo.RecordBeforeCreate, %[4]q, beforeCreate%[1]s); err != nil {
		return err
	}
	if err := registry.RegisterEntity(%[2]q, %[3]q, dygo.RecordAfterCreate, %[5]q, afterCreate%[1]s); err != nil {
		return err
	}
	if err := registry.RegisterEntity(%[2]q, %[3]q, dygo.RecordBeforeUpdate, %[6]q, beforeUpdate%[1]s); err != nil {
		return err
	}
	if err := registry.RegisterEntity(%[2]q, %[3]q, dygo.RecordAfterUpdate, %[7]q, afterUpdate%[1]s); err != nil {
		return err
	}
	return nil
}

func beforeCreate%[1]s(ctx context.Context, hook dygo.RecordHook) error {
	return nil
}

func afterCreate%[1]s(ctx context.Context, hook dygo.RecordHook) error {
	return nil
}

func beforeUpdate%[1]s(ctx context.Context, hook dygo.RecordHook) error {
	return nil
}

func afterUpdate%[1]s(ctx context.Context, hook dygo.RecordHook) error {
	return nil
}
`, name, appName, entityName, entityName+"-before-create", entityName+"-after-create", entityName+"-before-update", entityName+"-after-update")
	return runnergen.FormatGoSource([]byte(source))
}

func writeStatus(written bool) string {
	if written {
		return "updated"
	}
	return "unchanged"
}

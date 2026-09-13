// Package projectgen generates dygo project scaffolds.
package projectgen

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/hapyco/dygo/internal/frameworkapp"
	scaffold "github.com/hapyco/dygo/internal/generate"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/reserved"
	"github.com/hapyco/dygo/internal/runnergen"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/studio"
)

const (
	dygoModulePath  = "github.com/hapyco/dygo"
	defaultGo       = "1.26.2"
	developmentPort = 6790
)

// Options configures project generation.
type Options struct {
	Name          string
	ModulePath    string
	WorkingDir    string
	DygoVersion   string
	FrameworkRoot string
	SkipTidy      bool
	DatabaseURL   string

	// StudioAssets is for tests and custom project generators. Normal builds use
	// framework build output or bundled release assets.
	StudioAssets fs.FS
	// CoreAssets is for tests and custom project generators. Normal builds use
	// framework source metadata or the Core App bundled into the release binary.
	CoreAssets fs.FS
}

// Result describes a generated dygo project.
type Result struct {
	Name         string
	Label        string
	ModulePath   string
	Path         string
	DatabaseURL  string
	TidyRun      bool
	CoreSource   string
	StudioSource string
}

type dygoDependency struct {
	Version string
	Replace string
}

// Generate creates a new dygo project.
func Generate(ctx context.Context, options Options) (Result, error) {
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
	name, err := NormalizeName(options.Name)
	if err != nil {
		return Result{}, err
	}
	if reserved.IsApp(name) {
		return Result{}, fmt.Errorf("app name %q is reserved for framework-managed apps", name)
	}
	modulePath := strings.TrimSpace(options.ModulePath)
	if modulePath == "" {
		modulePath = name
	}
	if err := validateModulePath(modulePath); err != nil {
		return Result{}, err
	}
	dep, err := resolveDygoDependency(options, workingDir)
	if err != nil {
		return Result{}, err
	}

	target := filepath.Join(workingDir, name)
	if _, err := os.Stat(target); err == nil {
		return Result{}, fmt.Errorf("target project path %s already exists", target)
	} else if !os.IsNotExist(err) {
		return Result{}, fmt.Errorf("stat target project path %s: %w", target, err)
	}

	label := LabelForName(name)
	databaseURL := strings.TrimSpace(options.DatabaseURL)
	if databaseURL == "" {
		databaseURL = defaultDatabaseURL(name)
	}

	if err := writeProjectFiles(target, name, label, modulePath, dep); err != nil {
		return Result{}, err
	}
	store := secrets.NewStore(target)
	if _, err := store.Init(); err != nil {
		return Result{}, fmt.Errorf("initialize encrypted secrets: %w", err)
	}
	if err := store.Set(secrets.EnvironmentDevelopment, "DATABASE_URL", databaseURL); err != nil {
		return Result{}, fmt.Errorf("write development database secret: %w", err)
	}
	coreSource, err := installCoreCache(target, options, dep)
	if err != nil {
		return Result{}, err
	}
	studioSource, err := installStudioCache(target, options, dep)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Name:         name,
		Label:        label,
		ModulePath:   modulePath,
		Path:         target,
		DatabaseURL:  databaseURL,
		CoreSource:   coreSource,
		StudioSource: studioSource,
	}
	if !options.SkipTidy {
		if err := runGoModTidy(ctx, target); err != nil {
			return Result{}, err
		}
		result.TidyRun = true
	}
	return result, nil
}

func installCoreCache(root string, options Options, dep dygoDependency) (string, error) {
	sources := make([]frameworkapp.Source, 0, 3)
	if options.CoreAssets != nil {
		sources = append(sources, frameworkapp.Source{Name: "configured Core App", FS: options.CoreAssets})
	}
	if dep.Replace != "" {
		source, ok, err := frameworkapp.SourceFromDir("framework Core App", frameworkapp.FrameworkCorePath(dep.Replace))
		if err != nil {
			return "", fmt.Errorf("resolve framework Core App: %w", err)
		}
		if ok {
			sources = append(sources, source)
		}
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

// NormalizeName converts a project display name into a dygo kebab-case name.
func NormalizeName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("project name is required")
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case isASCIIAlpha(r) || isASCIIDigit(r):
			builder.WriteRune(lowerASCII(r))
			lastDash = false
		default:
			if builder.Len() > 0 && !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	name := strings.Trim(builder.String(), "-")
	if name == "" {
		return "", fmt.Errorf("project name %q must contain letters or numbers", value)
	}
	first, _ := firstRune(name)
	if !isASCIIAlpha(first) {
		return "", fmt.Errorf("project name %q must start with a letter after normalization", value)
	}
	return name, nil
}

func isASCIIAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func lowerASCII(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

// LabelForName converts a kebab-case project name into a display label.
func LabelForName(name string) string {
	parts := strings.Split(name, "-")
	for index, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[index] = string(runes)
	}
	return strings.Join(parts, " ")
}

func firstRune(value string) (rune, bool) {
	for _, r := range value {
		return r, true
	}
	return 0, false
}

func validateModulePath(modulePath string) error {
	if strings.TrimSpace(modulePath) == "" {
		return fmt.Errorf("module path is required")
	}
	if strings.ContainsAny(modulePath, " \t\r\n") {
		return fmt.Errorf("module path %q must not contain whitespace", modulePath)
	}
	return nil
}

func resolveDygoDependency(options Options, workingDir string) (dygoDependency, error) {
	version := strings.TrimSpace(options.DygoVersion)
	if version == "" {
		version = "dev"
	}
	if version != "dev" {
		return dygoDependency{Version: version}, nil
	}
	frameworkRoot := strings.TrimSpace(options.FrameworkRoot)
	if frameworkRoot == "" {
		root, err := project.DiscoverRoot(workingDir)
		if err == nil && root.Marker == "framework-repo" {
			frameworkRoot = root.Path
		}
	}
	if frameworkRoot == "" {
		return dygoDependency{}, fmt.Errorf("cannot resolve dygo module version for dev build; run dygo new from the framework checkout or use a release build")
	}
	frameworkRoot, err := filepath.Abs(frameworkRoot)
	if err != nil {
		return dygoDependency{}, fmt.Errorf("resolve framework root: %w", err)
	}
	return dygoDependency{Version: "v0.0.0", Replace: frameworkRoot}, nil
}

func installStudioCache(root string, options Options, dep dygoDependency) (string, error) {
	appSources := make([]studio.AppSource, 0, 2)
	if dep.Replace != "" {
		source, ok, err := studio.AppSourceFromDir("framework Studio App", studio.FrameworkAppPath(dep.Replace))
		if err != nil {
			return "", fmt.Errorf("resolve framework Studio App: %w", err)
		}
		if ok {
			appSources = append(appSources, source)
		}
	}
	embeddedApp, err := studio.EmbeddedAppSource()
	if err != nil {
		return "", err
	}
	appSources = append(appSources, embeddedApp)

	assetSources := make([]studio.Source, 0, 3)
	if options.StudioAssets != nil {
		assetSources = append(assetSources, studio.Source{Name: "configured Studio assets", FS: options.StudioAssets})
	}
	if dep.Replace != "" {
		source, ok, err := studio.SourceFromDir("framework Studio build", studio.FrameworkDistPath(dep.Replace))
		if err != nil {
			return "", fmt.Errorf("resolve framework Studio build: %w", err)
		}
		if ok {
			assetSources = append(assetSources, source)
		}
	}
	source, ok, err := studio.EmbeddedSource()
	if err != nil {
		return "", err
	}
	if ok {
		assetSources = append(assetSources, source)
	}
	name, err := studio.InstallApp(root, appSources, assetSources)
	if err != nil {
		return "", fmt.Errorf("install Studio App: %w", err)
	}
	return name, nil
}

func writeProjectFiles(root string, name string, label string, modulePath string, dep dygoDependency) error {
	dirs := []string{
		shape.ConfigSecretsDir,
		shape.DatabaseDir,
		shape.DocsDir,
		shape.LocalStudioAppDir,
		shape.LocalFilesDir,
		shape.LocalLogsDir,
		shape.LocalTempDir,
		shape.LocalSecretsDir,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	runner, err := runnerSource()
	if err != nil {
		return err
	}
	files := map[string]string{
		".gitignore":           gitignoreSource(),
		"README.md":            readmeSource(label),
		project.MarkerFile:     configSource(name),
		"go.mod":               goModSource(modulePath, dep),
		"cmd/dygo/main.go":     runner,
		shape.ConfigQueuesFile: queuesSource(),
		shape.SchemaSnapshot:   schemaSource(),
		"docs/index.md":        docsIndexSource(label),
	}
	for path, source := range files {
		if err := writeProjectFile(root, path, []byte(source), 0o644); err != nil {
			return err
		}
	}
	if _, err := scaffold.App(scaffold.Options{Root: root}, name); err != nil {
		return fmt.Errorf("generate default app skeleton: %w", err)
	}
	return nil
}

func writeProjectFile(root string, path string, data []byte, mode os.FileMode) error {
	target := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(target, data, mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func goModSource(modulePath string, dep dygoDependency) string {
	var builder strings.Builder
	builder.WriteString("module ")
	builder.WriteString(modulePath)
	builder.WriteString("\n\ngo ")
	builder.WriteString(defaultGo)
	builder.WriteString("\n\nrequire ")
	builder.WriteString(dygoModulePath)
	builder.WriteByte(' ')
	builder.WriteString(dep.Version)
	builder.WriteByte('\n')
	if dep.Replace != "" {
		builder.WriteString("\nreplace ")
		builder.WriteString(dygoModulePath)
		builder.WriteString(" => ")
		builder.WriteString(filepath.ToSlash(dep.Replace))
		builder.WriteByte('\n')
	}
	return builder.String()
}

func runnerSource() (string, error) {
	formatted, err := runnergen.RenderSource(nil, nil)
	if err != nil {
		return "", fmt.Errorf("format generated runner: %w", err)
	}
	return string(formatted), nil
}

func configSource(name string) string {
	return fmt.Sprintf(`name: %s
server:
  host: 127.0.0.1
  port: %d
database:
  driver: postgres
  url:
    secret: DATABASE_URL
`, name, developmentPort)
}

func gitignoreSource() string {
	return `# dygo local secrets
.dygo/secrets/master.key

# dygo runtime/cache data
.dygo/*
!.dygo/apps/
.dygo/apps/*
!.dygo/apps/core/
!.dygo/apps/core/**
!.dygo/apps/studio/
!.dygo/apps/studio/**
.dygo/apps/studio/ui/
.dygo/apps/studio/ui/**

# Go artifacts
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
coverage.*
*.coverprofile

# Environment/editor files
.env
.DS_Store
`
}

func readmeSource(label string) string {
	return fmt.Sprintf(`# %s

Generated by dygo.

## Development

%s

The generated development database secret is encrypted in `+"`config/secrets/development.yml.age`"+`.
Do not commit `+"`.dygo/secrets/master.key`"+`.
Commit `+"`.dygo/apps/core`"+` and `+"`.dygo/apps/studio`"+` metadata so every checkout has the required framework metadata and Studio configuration. The generated Studio UI build remains ignored.
`, label, "```sh\n"+
		"dygo secret edit\n"+
		"dygo db prepare\n"+
		"dygo setup\n"+
		"dygo dev\n"+
		"```")
}

func docsIndexSource(label string) string {
	return "# " + label + " Docs\n"
}

func schemaSource() string {
	return "-- Generated by dygo. Run `dygo db migrate` to update this snapshot.\n"
}

func queuesSource() string {
	return `queues:
  - name: default
    concurrency: 4
`
}

func defaultDatabaseURL(name string) string {
	return "postgres://localhost/" + strings.ReplaceAll(name, "-", "_") + "_development?sslmode=disable"
}

func runGoModTidy(ctx context.Context, root string) error {
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run go mod tidy in %s: %w: %s", root, err, strings.TrimSpace(string(output)))
	}
	return nil
}

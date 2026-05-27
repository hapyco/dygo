// Package manifest loads and validates dygo app manifests.
package manifest

import (
	"bytes"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hapyco/dygo/internal/yamlmeta"
	"gopkg.in/yaml.v3"
)

// Filename is the only recognized dygo app manifest filename.
const Filename = "app.yml"

var (
	kebabNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	versionPattern   = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
)

// Manifest describes one dygo app.
type Manifest struct {
	Name         string   `yaml:"name"`
	Label        string   `yaml:"label"`
	Version      string   `yaml:"version"`
	Description  string   `yaml:"description,omitempty"`
	Dependencies []string `yaml:"dependencies,omitempty"`
	Paths        Paths    `yaml:"paths,omitempty"`
}

// Paths contains app-relative directories for app-owned metadata and behavior.
type Paths struct {
	Entities string `yaml:"entities,omitempty"`
	Patches  string `yaml:"patches,omitempty"`
	Docs     string `yaml:"docs,omitempty"`
	Assets   string `yaml:"assets,omitempty"`
}

// LoadedApp is an app manifest loaded from a concrete app directory.
type LoadedApp struct {
	Dir          string
	ManifestPath string
	Manifest     Manifest
}

// ValidationError reports one or more manifest validation problems.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "app manifest validation failed: " + strings.Join(e.Problems, "; ")
}

// DefaultPaths returns dygo's standard app directory names.
func DefaultPaths() Paths {
	return Paths{
		Entities: "entities",
		Patches:  "patches",
		Docs:     "docs",
		Assets:   "assets",
	}
}

// WithDefaults returns paths with omitted values filled from DefaultPaths.
func (p Paths) WithDefaults() Paths {
	defaults := DefaultPaths()
	if p.Entities == "" {
		p.Entities = defaults.Entities
	}
	if p.Patches == "" {
		p.Patches = defaults.Patches
	}
	if p.Docs == "" {
		p.Docs = defaults.Docs
	}
	if p.Assets == "" {
		p.Assets = defaults.Assets
	}
	return p
}

// LoadFile reads, decodes, and validates one app manifest file.
func LoadFile(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read app manifest %s: %w", path, err)
	}

	if err := rejectDuplicateKeys(data); err != nil {
		return Manifest{}, fmt.Errorf("validate app manifest %s: %w", path, err)
	}

	var app Manifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&app); err != nil {
		return Manifest{}, fmt.Errorf("decode app manifest %s: %w", path, err)
	}

	app.Paths = app.Paths.WithDefaults()
	if err := app.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("validate app manifest %s: %w", path, err)
	}

	return app, nil
}

// LoadAppDir loads app.yml from an app directory.
func LoadAppDir(dir string) (LoadedApp, error) {
	manifestPath := filepath.Join(dir, Filename)
	app, err := LoadFile(manifestPath)
	if err != nil {
		return LoadedApp{}, err
	}
	return LoadedApp{
		Dir:          dir,
		ManifestPath: manifestPath,
		Manifest:     app,
	}, nil
}

// Discover loads app manifests from immediate child directories of root.
func Discover(root string) ([]LoadedApp, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read app root %s: %w", root, err)
	}

	var apps []LoadedApp
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appDir := filepath.Join(root, entry.Name())
		manifestPath := filepath.Join(appDir, Filename)
		if _, err := os.Stat(manifestPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat app manifest %s: %w", manifestPath, err)
		}

		app, err := LoadAppDir(appDir)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}

	sort.SliceStable(apps, func(i, j int) bool {
		return apps[i].Manifest.Name < apps[j].Manifest.Name
	})

	return apps, nil
}

// Validate validates one app manifest.
func (m Manifest) Validate() error {
	var problems []string

	if strings.TrimSpace(m.Name) == "" {
		problems = append(problems, "name is required")
	} else if !isKebabName(m.Name) {
		problems = append(problems, fmt.Sprintf("name %q must be kebab-case", m.Name))
	}

	if strings.TrimSpace(m.Label) == "" {
		problems = append(problems, "label is required")
	}

	if strings.TrimSpace(m.Version) == "" {
		problems = append(problems, "version is required")
	} else if !versionPattern.MatchString(m.Version) {
		problems = append(problems, fmt.Sprintf("version %q must look like semantic version major.minor.patch", m.Version))
	}

	seenDependencies := map[string]struct{}{}
	for _, dependency := range m.Dependencies {
		if !isKebabName(dependency) {
			problems = append(problems, fmt.Sprintf("dependency %q must be kebab-case", dependency))
			continue
		}
		if dependency == m.Name {
			problems = append(problems, fmt.Sprintf("app %q cannot depend on itself", m.Name))
		}
		if _, ok := seenDependencies[dependency]; ok {
			problems = append(problems, fmt.Sprintf("duplicate dependency %q", dependency))
		}
		seenDependencies[dependency] = struct{}{}
	}

	validatePaths(m.Paths.WithDefaults(), &problems)

	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

// ValidateSet validates app names and dependencies across loaded apps.
func ValidateSet(apps []LoadedApp) error {
	var problems []string
	seenApps := map[string]string{}

	for _, app := range apps {
		if err := app.Manifest.Validate(); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", app.ManifestPath, err))
		}

		name := app.Manifest.Name
		if name == "" {
			continue
		}
		if previousPath, ok := seenApps[name]; ok {
			problems = append(problems, fmt.Sprintf("duplicate app %q in %s and %s", name, previousPath, app.ManifestPath))
			continue
		}
		seenApps[name] = app.ManifestPath
	}

	for _, app := range apps {
		for _, dependency := range app.Manifest.Dependencies {
			if _, ok := seenApps[dependency]; !ok {
				problems = append(problems, fmt.Sprintf("app %q depends on unknown app %q", app.Manifest.Name, dependency))
			}
		}
	}

	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func isKebabName(value string) bool {
	return kebabNamePattern.MatchString(value)
}

func validatePaths(paths Paths, problems *[]string) {
	validatePath("entities", paths.Entities, problems)
	validatePath("patches", paths.Patches, problems)
	validatePath("docs", paths.Docs, problems)
	validatePath("assets", paths.Assets, problems)
}

func validatePath(name string, value string, problems *[]string) {
	if value == "" {
		*problems = append(*problems, fmt.Sprintf("path %s is required", name))
		return
	}
	if strings.TrimSpace(value) != value {
		*problems = append(*problems, fmt.Sprintf("path %s %q must not have leading or trailing whitespace", name, value))
		return
	}
	if strings.Contains(value, `\`) {
		*problems = append(*problems, fmt.Sprintf("path %s %q must use forward slashes", name, value))
		return
	}
	if pathpkg.IsAbs(value) {
		*problems = append(*problems, fmt.Sprintf("path %s %q must be relative", name, value))
		return
	}
	clean := pathpkg.Clean(value)
	if clean != value {
		*problems = append(*problems, fmt.Sprintf("path %s %q must be clean", name, value))
		return
	}
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		*problems = append(*problems, fmt.Sprintf("path %s %q must stay inside the app directory", name, value))
	}
}

func rejectDuplicateKeys(data []byte) error {
	root, err := yamlmeta.Parse(data, "parse yaml")
	if err != nil {
		return err
	}
	return yamlmeta.RejectDuplicateKeys(&root, func(duplicate yamlmeta.DuplicateKey) error {
		return fmt.Errorf("duplicate key %q at %s", duplicate.Key, strings.TrimSuffix(duplicate.Location, "."+duplicate.Key))
	})
}

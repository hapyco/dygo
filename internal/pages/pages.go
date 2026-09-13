// Package pages loads app-owned dygo Page metadata.
package pages

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/reserved"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/yamlmeta"
	"github.com/hapyco/dygo/pkg/dygo"
	"gopkg.in/yaml.v3"
)

// LoadedPage is one Page loaded from an owning app.
type LoadedPage struct {
	AppName string
	AppDir  string
	Path    string
	Page    dygo.Page
}

type pageMetadata struct {
	Label       string         `yaml:"label"`
	Description string         `yaml:"description,omitempty"`
	Icon        string         `yaml:"icon,omitempty"`
	Route       pageRoute      `yaml:"route"`
	Renderer    string         `yaml:"renderer"`
	Options     map[string]any `yaml:"options,omitempty"`
}

type pageRoute struct {
	Path string `yaml:"path"`
}

// Key returns the stable app-scoped Page identity.
func (p LoadedPage) Key() string {
	return p.AppName + "\x00" + p.Page.Key
}

// RoutePath returns the normalized public URL path for this Page.
func (p LoadedPage) RoutePath() string {
	path := strings.TrimSpace(p.Page.Path)
	if path == "" {
		return "/" + p.Page.Key
	}
	if path == "/" {
		return path
	}
	return "/" + strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
}

// Catalog loads Page metadata from discovered apps.
type Catalog struct {
	apps []manifest.LoadedApp
}

// New returns a Page Catalog for the given loaded apps.
func New(apps []manifest.LoadedApp) Catalog {
	copied := make([]manifest.LoadedApp, len(apps))
	copy(copied, apps)
	return Catalog{apps: copied}
}

// Discover loads Page metadata files from app-owned pages directories.
func (c Catalog) Discover() ([]LoadedPage, error) {
	var loaded []LoadedPage
	for _, app := range c.apps {
		pages, err := c.discoverApp(app)
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, pages...)
	}
	sort.SliceStable(loaded, func(i, j int) bool {
		if loaded[i].AppName != loaded[j].AppName {
			return loaded[i].AppName < loaded[j].AppName
		}
		if loaded[i].Page.Key != loaded[j].Page.Key {
			return loaded[i].Page.Key < loaded[j].Page.Key
		}
		return loaded[i].Path < loaded[j].Path
	})
	return loaded, nil
}

// Validate discovers Pages and validates app-level Page catalog rules.
func (c Catalog) Validate() ([]LoadedPage, error) {
	loaded, err := c.Discover()
	if err != nil {
		return nil, err
	}
	if err := validateCatalog(loaded); err != nil {
		return nil, err
	}
	return loaded, nil
}

func (c Catalog) discoverApp(app manifest.LoadedApp) ([]LoadedPage, error) {
	pagesDir := filepath.Join(app.Dir, shape.AppPagesDir)
	entries, err := os.ReadDir(pagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read pages for app %q from %s: %w", app.Manifest.Name, pagesDir, err)
	}

	var loaded []LoadedPage
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pageDir := filepath.Join(pagesDir, entry.Name())
		expected := shape.PageMetadataFileName(entry.Name())
		var metadataPath string
		children, err := os.ReadDir(pageDir)
		if err != nil {
			return nil, fmt.Errorf("read page bundle for app %q from %s: %w", app.Manifest.Name, pageDir, err)
		}
		for _, child := range children {
			if child.IsDir() || filepath.Ext(child.Name()) != ".yml" {
				continue
			}
			info, err := child.Info()
			if err != nil {
				return nil, fmt.Errorf("stat page for app %q from %s: %w", app.Manifest.Name, filepath.Join(pageDir, child.Name()), err)
			}
			if !info.Mode().IsRegular() {
				continue
			}
			path := filepath.Join(pageDir, child.Name())
			if child.Name() != expected {
				return nil, fmt.Errorf("%s is not a valid Page bundle file; Page metadata must be %s", path, filepath.Join(pageDir, expected))
			}
			metadataPath = path
		}
		if metadataPath == "" {
			continue
		}
		page, err := LoadFile(metadataPath)
		if err != nil {
			return nil, fmt.Errorf("load page for app %q from %s: %w", app.Manifest.Name, metadataPath, err)
		}
		page.App = app.Manifest.Name
		page.Name = app.Manifest.Name + "." + page.Key
		page.Source = "file"
		loaded = append(loaded, LoadedPage{AppName: app.Manifest.Name, AppDir: app.Dir, Path: metadataPath, Page: page})
	}
	return loaded, nil
}

// LoadFile reads and validates one Page metadata file.
func LoadFile(path string) (dygo.Page, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return dygo.Page{}, fmt.Errorf("read page metadata %s: %w", path, err)
	}
	page, err := Decode(data)
	if err != nil {
		return dygo.Page{}, fmt.Errorf("load page metadata %s: %w", path, err)
	}
	name, err := pageNameFromPath(path)
	if err != nil {
		return dygo.Page{}, fmt.Errorf("load page metadata %s: %w", path, err)
	}
	page.Key = name
	if err := ValidatePage(page); err != nil {
		return dygo.Page{}, fmt.Errorf("load page metadata %s: %w", path, err)
	}
	return page, nil
}

// Decode decodes and validates one Page metadata document.
func Decode(data []byte) (dygo.Page, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return dygo.Page{}, err
	}
	var metadata pageMetadata
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&metadata); err != nil && err != io.EOF {
		return dygo.Page{}, fmt.Errorf("decode page metadata: %w", err)
	}
	page := dygo.Page{
		Label:       metadata.Label,
		Description: metadata.Description,
		Icon:        metadata.Icon,
		Path:        metadata.Route.Path,
		Renderer:    metadata.Renderer,
		Options:     metadata.Options,
	}
	if err := ValidatePage(page); err != nil {
		return dygo.Page{}, err
	}
	return page, nil
}

func rejectDuplicateKeys(data []byte) error {
	root, err := yamlmeta.Parse(data, "parse page metadata")
	if err != nil {
		return err
	}
	return yamlmeta.RejectDuplicateKeys(&root, func(duplicate yamlmeta.DuplicateKey) error {
		return fmt.Errorf("duplicate page metadata key %q at line %d, previously defined at line %d", duplicate.Location, duplicate.Line, duplicate.PreviousLine)
	})
}

// ValidatePage checks one Page definition without app catalog context.
func ValidatePage(p dygo.Page) error {
	var problems []string
	if strings.TrimSpace(p.Key) != "" && !isMetadataName(p.Key) {
		problems = append(problems, fmt.Sprintf("page %q must be kebab-case", p.Key))
	}
	if strings.TrimSpace(p.Label) == "" {
		problems = append(problems, "label is required")
	}
	if strings.TrimSpace(p.Path) == "" {
		problems = append(problems, "path is required")
	} else if p.Path != "/" && (p.Path[0] != '/' || strings.Count(p.Path, "/") != 1 || strings.ContainsAny(p.Path, "?#")) {
		problems = append(problems, fmt.Sprintf("path %q must be / or an absolute kebab-case root path", p.Path))
	} else if p.Path != "/" && !isMetadataName(strings.TrimPrefix(p.Path, "/")) {
		problems = append(problems, fmt.Sprintf("path %q must be / or an absolute kebab-case root path", p.Path))
	}
	if strings.TrimSpace(p.Renderer) == "" {
		problems = append(problems, "renderer is required")
	} else if p.Renderer != "entity-index" {
		problems = append(problems, fmt.Sprintf("renderer %q is not supported", p.Renderer))
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

// ValidationError reports one or more Page metadata validation problems.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "page metadata validation failed: " + strings.Join(e.Problems, "; ")
}

func validateCatalog(loaded []LoadedPage) error {
	var problems []string
	seen := map[string]LoadedPage{}
	seenPaths := map[string]LoadedPage{}
	for _, page := range loaded {
		if previous, ok := seen[page.Key()]; ok {
			problems = append(problems, fmt.Sprintf("app %q page %q duplicates Page identity from %s", page.AppName, page.Page.Key, previous.Path))
		}
		seen[page.Key()] = page
		path := page.RoutePath()
		if path != "/" && reserved.IsSlug(strings.TrimPrefix(path, "/")) {
			problems = append(problems, fmt.Sprintf("app %q page %q uses reserved root route path %q", page.AppName, page.Page.Key, path))
		}
		if previous, ok := seenPaths[path]; ok {
			problems = append(problems, fmt.Sprintf("app %q page %q path %q conflicts with app %q page %q at %s", page.AppName, page.Page.Key, path, previous.AppName, previous.Page.Key, previous.Path))
		} else {
			seenPaths[path] = page
		}
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

func pageNameFromPath(path string) (string, error) {
	base := filepath.Base(path)
	dir := filepath.Base(filepath.Dir(path))
	if base != shape.PageMetadataFileName(dir) {
		return "", fmt.Errorf("metadata filename must be %s", shape.PageMetadataFileName(dir))
	}
	if err := shape.ValidateMetadataName("page", dir); err != nil {
		return "", err
	}
	return dir, nil
}

func isMetadataName(value string) bool {
	return shape.ValidateMetadataName("metadata name", value) == nil
}

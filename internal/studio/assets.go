// Package studio serves and installs first-party Studio UI assets.
package studio

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/hapyco/dygo/internal/fsutil"
)

const (
	projectAppDir   = ".dygo/apps/studio"
	projectCacheDir = projectAppDir + "/ui/dist"
)

//go:embed bundled
var bundled embed.FS

//go:embed all:bundled-app
var bundledApp embed.FS

var embeddedSource = defaultEmbeddedSource

// Source describes one possible Studio asset source.
type Source struct {
	Name string
	FS   fs.FS
}

// AppSource describes one possible Studio App metadata source.
type AppSource struct {
	Name string
	FS   fs.FS
}

// ProjectCachePath returns the generated-project Studio asset cache path.
func ProjectCachePath(root string) string {
	return filepath.Join(root, filepath.FromSlash(projectCacheDir))
}

// FrameworkDistPath returns the Studio build output path inside a framework checkout.
func FrameworkDistPath(root string) string {
	return filepath.Join(root, "apps", "studio", "ui", "dist")
}

// FrameworkAppPath returns the Studio App path inside a framework checkout.
func FrameworkAppPath(root string) string {
	return filepath.Join(root, "apps", "studio")
}

// AppSourceFromDir returns a source when dir contains the Studio App manifest.
func AppSourceFromDir(name string, dir string) (AppSource, bool, error) {
	if strings.TrimSpace(dir) == "" {
		return AppSource{}, false, nil
	}
	source := AppSource{Name: name, FS: os.DirFS(dir)}
	ok, err := hasManifest(source.FS)
	if err != nil {
		return AppSource{}, false, fmt.Errorf("check %s: %w", name, err)
	}
	return source, ok, nil
}

// EmbeddedAppSource returns the Studio App metadata bundled into this dygo binary.
func EmbeddedAppSource() (AppSource, error) {
	fsys, err := fs.Sub(bundledApp, "bundled-app")
	if err != nil {
		return AppSource{}, fmt.Errorf("open bundled Studio App: %w", err)
	}
	if ok, err := hasManifest(fsys); err != nil {
		return AppSource{}, fmt.Errorf("check bundled Studio App: %w", err)
	} else if !ok {
		return AppSource{}, fmt.Errorf("bundled Studio App is missing app.yml")
	}
	return AppSource{Name: "bundled Studio App", FS: fsys}, nil
}

// SourceFromDir returns a source when dir contains a built Studio index.html.
func SourceFromDir(name string, dir string) (Source, bool, error) {
	if strings.TrimSpace(dir) == "" {
		return Source{}, false, nil
	}
	source := Source{Name: name, FS: os.DirFS(dir)}
	ok, err := HasIndex(source.FS)
	if err != nil {
		return Source{}, false, fmt.Errorf("check %s: %w", name, err)
	}
	if !ok {
		return Source{}, false, nil
	}
	return source, true, nil
}

// EmbeddedSource returns the Studio assets bundled into this dygo binary.
func EmbeddedSource() (Source, bool, error) {
	return embeddedSource()
}

// SetEmbeddedSourceForTest replaces the bundled asset source for tests.
func SetEmbeddedSourceForTest(fn func() (Source, bool, error)) func() {
	previous := embeddedSource
	embeddedSource = fn
	return func() {
		embeddedSource = previous
	}
}

func defaultEmbeddedSource() (Source, bool, error) {
	fsys, err := fs.Sub(bundled, "bundled")
	if err != nil {
		return Source{}, false, fmt.Errorf("open bundled Studio assets: %w", err)
	}
	source := Source{Name: "bundled Studio assets", FS: fsys}
	ok, err := HasIndex(source.FS)
	if err != nil {
		return Source{}, false, fmt.Errorf("check bundled Studio assets: %w", err)
	}
	if !ok {
		return Source{}, false, nil
	}
	return source, true, nil
}

// HasIndex reports whether fsys contains a built Studio entrypoint.
func HasIndex(fsys fs.FS) (bool, error) {
	return fsutil.HasFile(fsys, "index.html")
}

func hasManifest(fsys fs.FS) (bool, error) {
	return fsutil.HasFile(fsys, "app.yml")
}

// NewStaticHandler serves built Studio assets with SPA route fallback.
func NewStaticHandler(fsys fs.FS) (http.Handler, error) {
	ok, err := HasIndex(fsys)
	if err != nil {
		return nil, fmt.Errorf("check Studio assets: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("Studio assets are missing index.html")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := cleanAssetPath(r.URL.Path)
		if assetExists(fsys, name) {
			http.ServeFileFS(w, r, fsys, name)
			return
		}
		if name == "assets" || strings.HasPrefix(name, "assets/") || path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFileFS(w, r, fsys, "index.html")
	}), nil
}

// HandlerForProject returns a Studio handler for cached project assets or bundled release assets.
func HandlerForProject(root string) (http.Handler, string, error) {
	cache, ok, err := SourceFromDir("project Studio cache", ProjectCachePath(root))
	if err != nil {
		return nil, "", err
	}
	if ok {
		handler, err := NewStaticHandler(cache.FS)
		if err != nil {
			return nil, "", err
		}
		return handler, cache.Name, nil
	}

	embedded, ok, err := EmbeddedSource()
	if err != nil {
		return nil, "", err
	}
	if ok {
		handler, err := NewStaticHandler(embedded.FS)
		if err != nil {
			return nil, "", err
		}
		return handler, embedded.Name, nil
	}

	return nil, "", fmt.Errorf("Studio UI assets are unavailable; expected a built Studio cache at %s or bundled Studio assets in this dygo binary. Run dygo upgrade to refresh generated-project assets, or use dygo dev to proxy a Studio dev server", ProjectCachePath(root))
}

// InstallApp installs Studio metadata and UI assets as one framework-managed App.
func InstallApp(root string, appSources []AppSource, assetSources []Source) (string, error) {
	appSource, err := firstAppSource(appSources)
	if err != nil {
		return "", err
	}
	assetSource, err := firstAssetSource(assetSources)
	if err != nil {
		return "", err
	}
	target := filepath.Join(root, filepath.FromSlash(projectAppDir))
	if err := replaceAppDir(appSource.FS, assetSource.FS, target); err != nil {
		return "", fmt.Errorf("install %s with %s: %w", appSource.Name, assetSource.Name, err)
	}
	return assetSource.Name, nil
}

func firstAppSource(sources []AppSource) (AppSource, error) {
	for _, source := range sources {
		if source.FS == nil {
			continue
		}
		ok, err := hasManifest(source.FS)
		if err != nil {
			return AppSource{}, fmt.Errorf("check %s: %w", source.Name, err)
		}
		if ok {
			return source, nil
		}
	}
	return AppSource{}, fmt.Errorf("Studio App metadata is unavailable")
}

func firstAssetSource(sources []Source) (Source, error) {
	for _, source := range sources {
		if source.FS == nil {
			continue
		}
		ok, err := HasIndex(source.FS)
		if err != nil {
			return Source{}, fmt.Errorf("check %s: %w", source.Name, err)
		}
		if ok {
			return source, nil
		}
	}
	return Source{}, fmt.Errorf("Studio UI assets are unavailable")
}

func replaceAppDir(appSource fs.FS, assetSource fs.FS, target string) error {
	return fsutil.ReplaceDir(target, ".studio-app-*", "Studio App", func(temp string) error {
		if err := copyAppMetadata(appSource, temp); err != nil {
			return err
		}
		if err := fsutil.CopyFS(assetSource, filepath.Join(temp, "ui", "dist"), "Studio asset"); err != nil {
			return err
		}
		if ok, err := hasManifest(os.DirFS(temp)); err != nil {
			return fmt.Errorf("verify temporary Studio App cache: %w", err)
		} else if !ok {
			return fmt.Errorf("temporary Studio App cache is missing app.yml")
		}
		if ok, err := HasIndex(os.DirFS(filepath.Join(temp, "ui", "dist"))); err != nil {
			return fmt.Errorf("verify temporary Studio UI cache: %w", err)
		} else if !ok {
			return fmt.Errorf("temporary Studio UI cache is missing index.html")
		}
		return nil
	})
}

func copyAppMetadata(source fs.FS, target string) error {
	return fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if name == "." {
			return nil
		}
		if !studioMetadataPath(name) {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		destination := filepath.Join(target, filepath.FromSlash(name))
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return fmt.Errorf("read Studio App metadata %s: %w", name, err)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return fmt.Errorf("create Studio App metadata directory %s: %w", name, err)
		}
		if err := os.WriteFile(destination, data, 0o644); err != nil {
			return fmt.Errorf("write Studio App metadata %s: %w", name, err)
		}
		return nil
	})
}

func studioMetadataPath(name string) bool {
	name = filepath.ToSlash(name)
	return name == "app.yml" ||
		name == "entities" ||
		strings.HasPrefix(name, "entities/") ||
		name == "access" ||
		strings.HasPrefix(name, "access/") ||
		name == "pages" ||
		strings.HasPrefix(name, "pages/")
}

func cleanAssetPath(value string) string {
	cleaned := path.Clean("/" + value)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." || cleaned == "" {
		return "index.html"
	}
	return cleaned
}

func assetExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}

// Package studio serves and installs first-party Studio UI assets.
package studio

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const projectCacheDir = ".dygo/apps/studio/ui/dist"

//go:embed bundled
var bundled embed.FS

var embeddedSource = defaultEmbeddedSource

// Source describes one possible Studio asset source.
type Source struct {
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
	if fsys == nil {
		return false, nil
	}
	info, err := fs.Stat(fsys, "index.html")
	if err == nil {
		return !info.IsDir(), nil
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
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

// InstallCache copies the first available source into the generated-project Studio cache.
func InstallCache(root string, sources ...Source) (bool, string, error) {
	for _, source := range sources {
		if source.FS == nil {
			continue
		}
		ok, err := HasIndex(source.FS)
		if err != nil {
			return false, "", fmt.Errorf("check %s: %w", source.Name, err)
		}
		if !ok {
			continue
		}
		if err := replaceDir(source.FS, ProjectCachePath(root)); err != nil {
			return false, "", fmt.Errorf("install %s: %w", source.Name, err)
		}
		return true, source.Name, nil
	}
	return false, "", nil
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

func replaceDir(source fs.FS, target string) error {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create Studio cache parent: %w", err)
	}
	temp, err := os.MkdirTemp(parent, ".studio-dist-*")
	if err != nil {
		return fmt.Errorf("create temporary Studio cache: %w", err)
	}
	defer func() {
		if temp != "" {
			_ = os.RemoveAll(temp)
		}
	}()

	if err := copyFS(source, temp); err != nil {
		return err
	}
	if ok, err := HasIndex(os.DirFS(temp)); err != nil {
		return fmt.Errorf("verify temporary Studio cache: %w", err)
	} else if !ok {
		return fmt.Errorf("temporary Studio cache is missing index.html")
	}

	backup := target + ".previous"
	_ = os.RemoveAll(backup)
	hadExisting := false
	if _, err := os.Stat(target); err == nil {
		hadExisting = true
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("move existing Studio cache aside: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check existing Studio cache: %w", err)
	}

	if err := os.Rename(temp, target); err != nil {
		if hadExisting {
			_ = os.Rename(backup, target)
		}
		return fmt.Errorf("replace Studio cache: %w", err)
	}
	temp = ""
	if hadExisting {
		_ = os.RemoveAll(backup)
	}
	return nil
}

func copyFS(source fs.FS, target string) error {
	return fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		targetPath := filepath.Join(target, filepath.FromSlash(name))
		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create Studio asset directory %s: %w", name, err)
			}
			return nil
		}
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return fmt.Errorf("read Studio asset %s: %w", name, err)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create Studio asset parent %s: %w", name, err)
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return fmt.Errorf("write Studio asset %s: %w", name, err)
		}
		return nil
	})
}

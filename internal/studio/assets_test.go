package studio

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestNewStaticHandlerServesAssetsAndSPAFallback(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":        {Data: []byte("<html>studio</html>")},
		"assets/index.js":   {Data: []byte("console.log('studio')")},
		"assets/index.css":  {Data: []byte("body{}")},
		"nested/readme.txt": {Data: []byte("asset")},
	}
	handler, err := NewStaticHandler(fsys)
	if err != nil {
		t.Fatalf("NewStaticHandler() error = %v, want nil", err)
	}

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{path: "/", wantStatus: http.StatusOK, wantBody: "studio"},
		{path: "/login", wantStatus: http.StatusOK, wantBody: "studio"},
		{path: "/lead/1", wantStatus: http.StatusOK, wantBody: "studio"},
		{path: "/assets/index.js", wantStatus: http.StatusOK, wantBody: "console.log"},
		{path: "/nested/readme.txt", wantStatus: http.StatusOK, wantBody: "asset"},
		{path: "/assets", wantStatus: http.StatusNotFound},
		{path: "/assets/missing.js", wantStatus: http.StatusNotFound},
		{path: "/missing.txt", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, tt.path, nil))
			response := recorder.Result()
			defer response.Body.Close()
			if response.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.wantStatus)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("ReadAll(body) error = %v", err)
			}
			if tt.wantBody != "" && !strings.Contains(string(body), tt.wantBody) {
				t.Fatalf("body = %q, want substring %q", string(body), tt.wantBody)
			}
		})
	}
}

func TestEmbeddedAppMatchesFrameworkSource(t *testing.T) {
	embedded, err := EmbeddedAppSource()
	if err != nil {
		t.Fatalf("EmbeddedAppSource() error = %v, want nil", err)
	}
	want := studioMetadataContents(t, os.DirFS(FrameworkAppPath(filepath.Join("..", ".."))))
	got := studioMetadataContents(t, embedded.FS)
	for name, wantContent := range want {
		if gotContent, ok := got[name]; !ok || gotContent != wantContent {
			t.Fatalf("bundled Studio App asset %q is stale; got %q, want %q; run go generate ./internal/studio", name, gotContent, wantContent)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Fatalf("bundled Studio App has retired asset %q; run go generate ./internal/studio", name)
		}
	}
}

func TestHandlerForProjectUsesProjectCache(t *testing.T) {
	root := t.TempDir()
	writeStudioAsset(t, root, "index.html", "<html>cached studio</html>")

	handler, source, err := HandlerForProject(root)
	if err != nil {
		t.Fatalf("HandlerForProject() error = %v, want nil", err)
	}
	if source != "project Studio cache" {
		t.Fatalf("source = %q, want project Studio cache", source)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/login", nil))
	body, err := io.ReadAll(recorder.Result().Body)
	if err != nil {
		t.Fatalf("ReadAll(body) error = %v", err)
	}
	if !strings.Contains(string(body), "cached studio") {
		t.Fatalf("body = %q, want cached Studio", string(body))
	}
}

func TestHandlerForProjectFailsWhenAssetsUnavailable(t *testing.T) {
	restore := SetEmbeddedSourceForTest(func() (Source, bool, error) {
		return Source{}, false, nil
	})
	defer restore()

	root := t.TempDir()

	_, _, err := HandlerForProject(root)
	if err == nil {
		t.Fatal("HandlerForProject() error = nil, want missing assets error")
	}
	for _, want := range []string{"Studio UI assets are unavailable", ProjectCachePath(root), "dygo dev"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestInstallAppReplacesManagedBundle(t *testing.T) {
	root := t.TempDir()
	stalePath := filepath.Join(root, filepath.FromSlash(projectAppDir), "stale.txt")
	if err := os.MkdirAll(filepath.Dir(stalePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(stale bundle) error = %v", err)
	}
	if err := os.WriteFile(stalePath, []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile(stale bundle) error = %v", err)
	}

	name, err := InstallApp(root, []AppSource{{
		Name: "test Studio App",
		FS: fstest.MapFS{
			"app.yml": {Data: []byte("name: studio\n")},
			"entities/preference/preference.entity.yml": {Data: []byte("label: Preference\n")},
			"access/home.page.access.yml":               {Data: []byte("policy: []\n")},
			"pages/home/home.page.yml":                  {Data: []byte("renderer: entity-index\n")},
			"ui/src/should-not-be-copied.txt":           {Data: []byte("source")},
		},
	}}, []Source{{
		Name: "test Studio assets",
		FS: fstest.MapFS{
			"index.html":    {Data: []byte("<html>installed</html>")},
			"assets/app.js": {Data: []byte("console.log('installed')")},
		},
	}})
	if err != nil {
		t.Fatalf("InstallApp() error = %v, want nil", err)
	}
	if name != "test Studio assets" {
		t.Fatalf("InstallApp() source = %q, want test Studio assets", name)
	}

	appRoot := filepath.Join(root, filepath.FromSlash(projectAppDir))
	for _, path := range []string{
		"app.yml",
		"entities/preference/preference.entity.yml",
		"access/home.page.access.yml",
		"pages/home/home.page.yml",
		"ui/dist/index.html",
		"ui/dist/assets/app.js",
	} {
		if _, err := os.Stat(filepath.Join(appRoot, filepath.FromSlash(path))); err != nil {
			t.Fatalf("Stat(%s) error = %v, want installed file", path, err)
		}
	}
	for _, path := range []string{"stale.txt", "ui/src/should-not-be-copied.txt"} {
		if _, err := os.Stat(filepath.Join(appRoot, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("Stat(%s) error = %v, want file absent", path, err)
		}
	}
}

func writeStudioAsset(t *testing.T, root string, name string, body string) {
	t.Helper()
	path := filepath.Join(ProjectCachePath(root), filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func studioMetadataContents(t *testing.T, source fs.FS) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !studioMetadataPath(name) {
			return nil
		}
		data, err := fs.ReadFile(source, name)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(name)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("WalkDir() error = %v", err)
	}
	return files
}

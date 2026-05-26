package studio

import (
	"io"
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

func TestInstallCacheCopiesFirstAvailableSource(t *testing.T) {
	root := t.TempDir()
	source := Source{
		Name: "test Studio assets",
		FS: fstest.MapFS{
			"index.html":      {Data: []byte("<html>installed</html>")},
			"assets/app.js":   {Data: []byte("console.log('installed')")},
			"assets/app.css":  {Data: []byte("body{}")},
			"placeholder.txt": {Data: []byte("ok")},
		},
	}

	installed, name, err := InstallCache(root, Source{Name: "empty", FS: fstest.MapFS{}}, source)
	if err != nil {
		t.Fatalf("InstallCache() error = %v, want nil", err)
	}
	if !installed || name != "test Studio assets" {
		t.Fatalf("InstallCache() = %v, %q, want test source", installed, name)
	}
	if got := readStudioCacheFile(t, root, "index.html"); !strings.Contains(got, "installed") {
		t.Fatalf("cached index.html = %q, want installed content", got)
	}
	if got := readStudioCacheFile(t, root, "assets/app.js"); !strings.Contains(got, "installed") {
		t.Fatalf("cached app.js = %q, want installed content", got)
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

func readStudioCacheFile(t *testing.T, root string, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(ProjectCachePath(root), filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", name, err)
	}
	return string(data)
}

package upgrade

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunCheckCLIOnlyUsesLatestRelease(t *testing.T) {
	server := releaseServer(t)
	defer server.Close()

	result, err := Run(context.Background(), Options{
		CurrentVersion: "v1.0.0",
		Check:          true,
		CLIOnly:        true,
		InstallDir:     t.TempDir(),
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
		GOOS:           "darwin",
		GOARCH:         "arm64",
	})
	if err != nil {
		t.Fatalf("Run(check cli-only) error = %v, want nil", err)
	}
	if result.TargetVersion != "v1.2.3" || result.CLI == nil || result.Project != nil {
		t.Fatalf("Run(check cli-only) result = %+v, want CLI-only latest plan", result)
	}
	if result.CLI.Installed {
		t.Fatalf("CLI.Installed = true, want check-only")
	}
}

func TestRunDryRunInsideProjectPlansCLIAndProject(t *testing.T) {
	server := releaseServer(t)
	defer server.Close()
	root := newUpgradeTestProject(t)

	result, err := Run(context.Background(), Options{
		CurrentVersion: "v1.0.0",
		DryRun:         true,
		WorkingDir:     root,
		InstallDir:     t.TempDir(),
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
		GOOS:           "linux",
		GOARCH:         "amd64",
	})
	if err != nil {
		t.Fatalf("Run(dry-run project) error = %v, want nil", err)
	}
	if result.CLI == nil || result.Project == nil {
		t.Fatalf("Run(dry-run project) result = %+v, want CLI and project plans", result)
	}
	if result.Project.CurrentVersion != "v0.0.0" || result.Project.TargetVersion != "v1.2.3" {
		t.Fatalf("Project result = %+v, want current and target versions", result.Project)
	}
}

func TestRunProjectOnlyOutsideProjectFails(t *testing.T) {
	server := releaseServer(t)
	defer server.Close()

	_, err := Run(context.Background(), Options{
		ProjectOnly: true,
		WorkingDir:  t.TempDir(),
		APIBaseURL:  server.URL,
		HTTPClient:  server.Client(),
	})
	if err == nil {
		t.Fatal("Run(project-only outside project) error = nil, want error")
	}
}

func TestRunCheckFailsWhenReleaseAssetIsMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tag_name":"v1.2.3","assets":[{"name":"checksums.txt","browser_download_url":"https://example.test/checksums.txt"}]}`)
	}))
	defer server.Close()

	_, err := Run(context.Background(), Options{
		Check:      true,
		CLIOnly:    true,
		InstallDir: t.TempDir(),
		APIBaseURL: server.URL,
		HTTPClient: server.Client(),
		GOOS:       "linux",
		GOARCH:     "amd64",
	})
	if err == nil {
		t.Fatal("Run(check missing asset) error = nil, want missing asset error")
	}
	if got := err.Error(); got != "release v1.2.3 does not contain asset dygo_v1.2.3_linux_amd64.tar.gz" {
		t.Fatalf("Run(check missing asset) error = %q, want missing asset error", got)
	}
}

func releaseServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/latest", "/releases/tags/v1.2.3":
			fmt.Fprint(w, `{"tag_name":"v1.2.3","assets":[{"name":"dygo_v1.2.3_darwin_arm64.tar.gz","browser_download_url":"https://example.test/darwin"},{"name":"dygo_v1.2.3_linux_amd64.tar.gz","browser_download_url":"https://example.test/linux"},{"name":"checksums.txt","browser_download_url":"https://example.test/checksums.txt"}]}`)
		default:
			t.Fatalf("unexpected release path %s", r.URL.Path)
		}
	}))
}

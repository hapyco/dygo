package upgrade

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubClientResolvesLatestRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases/latest" {
			t.Fatalf("request path = %s, want /releases/latest", r.URL.Path)
		}
		fmt.Fprint(w, `{"tag_name":"v1.2.3","assets":[{"name":"checksums.txt","browser_download_url":"https://example.test/checksums.txt"}]}`)
	}))
	defer server.Close()

	client := NewGitHubClient(ClientOptions{BaseURL: server.URL, HTTPClient: server.Client()})
	release, err := client.LatestRelease(context.Background())
	if err != nil {
		t.Fatalf("LatestRelease() error = %v, want nil", err)
	}
	if release.TagName != "v1.2.3" {
		t.Fatalf("TagName = %q, want v1.2.3", release.TagName)
	}
	if _, ok := release.Asset("checksums.txt"); !ok {
		t.Fatal("Asset(checksums.txt) missing, want present")
	}
}

func TestReleaseAssetName(t *testing.T) {
	got, err := releaseAssetName("v1.2.3", "darwin", "arm64")
	if err != nil {
		t.Fatalf("releaseAssetName() error = %v, want nil", err)
	}
	if got != "dygo_v1.2.3_darwin_arm64.tar.gz" {
		t.Fatalf("releaseAssetName() = %q, want darwin asset", got)
	}
	got, err = releaseAssetName("v1.2.3", "windows", "amd64")
	if err != nil {
		t.Fatalf("releaseAssetName(windows) error = %v, want nil", err)
	}
	if got != "dygo_v1.2.3_windows_amd64.zip" {
		t.Fatalf("releaseAssetName(windows) = %q, want windows asset", got)
	}
	if _, err := releaseAssetName("v1.2.3", "plan9", "amd64"); err == nil {
		t.Fatal("releaseAssetName(plan9) error = nil, want unsupported platform error")
	}
}

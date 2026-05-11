package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ClientOptions configures the GitHub release client.
type ClientOptions struct {
	BaseURL    string
	HTTPClient *http.Client
}

// GitHubClient resolves dygo releases.
type GitHubClient interface {
	LatestRelease(context.Context) (Release, error)
	ReleaseByTag(context.Context, string) (Release, error)
}

type githubClient struct {
	baseURL string
	client  *http.Client
}

// Release describes one GitHub release.
type Release struct {
	TagName string
	Assets  []Asset
}

// Asset describes one GitHub release asset.
type Asset struct {
	Name        string
	DownloadURL string
}

// NewGitHubClient returns a GitHub release client.
func NewGitHubClient(options ClientOptions) GitHubClient {
	client := options.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return githubClient{
		baseURL: strings.TrimRight(firstNonEmpty(options.BaseURL, DefaultAPIBaseURL), "/"),
		client:  client,
	}
}

func (c githubClient) LatestRelease(ctx context.Context) (Release, error) {
	return c.getRelease(ctx, c.baseURL+"/releases/latest")
}

func (c githubClient) ReleaseByTag(ctx context.Context, tag string) (Release, error) {
	return c.getRelease(ctx, c.baseURL+"/releases/tags/"+url.PathEscape(tag))
}

func (c githubClient) getRelease(ctx context.Context, endpoint string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Release{}, fmt.Errorf("create release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.client.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("fetch dygo release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Release{}, fmt.Errorf("fetch dygo release: GitHub returned %s", resp.Status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name        string `json:"name"`
			DownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Release{}, fmt.Errorf("decode dygo release: %w", err)
	}
	if strings.TrimSpace(payload.TagName) == "" {
		return Release{}, fmt.Errorf("dygo release is missing tag_name")
	}
	release := Release{TagName: payload.TagName}
	for _, asset := range payload.Assets {
		release.Assets = append(release.Assets, Asset{Name: asset.Name, DownloadURL: asset.DownloadURL})
	}
	return release, nil
}

func (r Release) Asset(name string) (Asset, bool) {
	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

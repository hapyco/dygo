// Package project contains helpers for locating dygo project state.
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hapyco/dygo/internal/shape"
)

const (
	// MarkerFile is the canonical root marker for generated dygo projects.
	MarkerFile = shape.ProjectConfigFile

	frameworkModule = "github.com/hapyco/dygo"
)

// Root describes a discovered dygo project or framework repository root.
type Root struct {
	Path   string
	Marker string
}

// DiscoverRoot walks upward from start until it finds a dygo project root.
func DiscoverRoot(start string) (Root, error) {
	if strings.TrimSpace(start) == "" {
		start = "."
	}

	current, err := filepath.Abs(start)
	if err != nil {
		return Root{}, fmt.Errorf("resolve start directory: %w", err)
	}
	info, err := os.Stat(current)
	if err != nil {
		return Root{}, fmt.Errorf("stat start directory %s: %w", current, err)
	}
	if !info.IsDir() {
		current = filepath.Dir(current)
	}

	for {
		root, ok, err := inspectRoot(current)
		if err != nil {
			return Root{}, err
		}
		if ok {
			return root, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			return Root{}, fmt.Errorf("no dygo project root found from %s", start)
		}
		current = parent
	}
}

func inspectRoot(dir string) (Root, bool, error) {
	markerPath := filepath.Join(dir, MarkerFile)
	markerInfo, err := os.Stat(markerPath)
	if err == nil {
		if markerInfo.IsDir() {
			return Root{}, false, fmt.Errorf("dygo root marker %s must be a file", markerPath)
		}
		return Root{Path: dir, Marker: MarkerFile}, true, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return Root{}, false, fmt.Errorf("stat dygo root marker %s: %w", markerPath, err)
	}

	ok, err := isFrameworkRoot(dir)
	if err != nil {
		return Root{}, false, err
	}
	if ok {
		return Root{Path: dir, Marker: "framework-repo"}, true, nil
	}

	return Root{}, false, nil
}

func isFrameworkRoot(dir string) (bool, error) {
	goModPath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read go.mod %s: %w", goModPath, err)
	}
	if !strings.Contains(string(data), "module "+frameworkModule) {
		return false, nil
	}

	for _, path := range []string{shape.AppsDir, filepath.Join("internal", "cli")} {
		info, err := os.Stat(filepath.Join(dir, path))
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, fmt.Errorf("stat framework root path %s: %w", filepath.Join(dir, path), err)
		}
		if !info.IsDir() {
			return false, nil
		}
	}

	return true, nil
}

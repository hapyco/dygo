package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dygo-dev/dygo/internal/project"
)

func workingRootPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("detect working directory: %w", err)
	}
	root, err := project.DiscoverRoot(wd)
	if err != nil {
		return "", err
	}
	return root.Path, nil
}

func relToWorkingRoot(path string) string {
	root, err := workingRootPath()
	if err != nil {
		return filepath.Clean(path)
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Clean(path)
	}
	return relative
}

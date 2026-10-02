package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/app/registry"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/reserved"
	"github.com/hapyco/dygo/internal/runnergen"
	"github.com/spf13/cobra"
)

type appRepositoryCheckout struct {
	Root   string
	App    manifest.LoadedApp
	Commit string
}

func newAppInstallCommand(stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "install <repository-url>",
		Short: "Install an App from a Git repository",
		Long: "Clone and validate an App repository, install its source under apps/, and update generated Hook and Job runner wiring. " +
			"Run dygo db migrate separately to apply database changes.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			if err := runnergen.RequireGeneratedProjectRoot(root); err != nil {
				return fmt.Errorf("app install requires a generated dygo project: %w", err)
			}

			checkout, err := withProgress(cmd.Context(), stderr, "Cloning and validating App repository", func() (appRepositoryCheckout, error) {
				return cloneAppRepository(cmd.Context(), args[0])
			})
			if err != nil {
				return err
			}
			defer os.RemoveAll(checkout.Root)

			existingApps, err := project.LoadApps(root)
			if err != nil {
				return err
			}
			for _, app := range existingApps {
				if app.Manifest.Name == checkout.App.Manifest.Name {
					return fmt.Errorf("app %q is already present at %s", app.Manifest.Name, relToHooksRoot(root, app.Dir))
				}
			}
			if reserved.IsApp(checkout.App.Manifest.Name) {
				return fmt.Errorf("validate cloned App: app name %q is reserved for framework-managed apps", checkout.App.Manifest.Name)
			}

			target := filepath.Join(root, "apps", checkout.App.Manifest.Name)
			plannedApp := checkout.App
			plannedApp.Dir = target
			plannedApp.ManifestPath = filepath.Join(target, manifest.Filename)
			apps := append(append([]manifest.LoadedApp(nil), existingApps...), plannedApp)
			// Share the manifest-set validation used by `dygo app validate` before
			// adding source to the project.
			if err := manifest.ValidateSet(apps); err != nil {
				return fmt.Errorf("validate cloned App with project: %w", err)
			}
			order, err := appInstallOrder(apps, checkout.App.Manifest.Name)
			if err != nil {
				return err
			}

			runnerFile := filepath.Join(root, "cmd", "dygo", "main.go")
			if err := runnergen.PreflightGeneratedFile(runnerFile, runnergen.UpgradeManualSnippet()); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(stdout, "app install plan\nproject: %s\napp: %s %s\nsource commit: %s\ndestination: %s (create)\n%d apps are valid\nprepare order: %s\nrunner: %s (regenerate for all project Apps)\n", root, checkout.App.Manifest.Name, checkout.App.Manifest.Version, checkout.Commit, relToHooksRoot(root, target), len(apps), strings.Join(order, " -> "), relToHooksRoot(root, runnerFile)); err != nil {
				return fmt.Errorf("write app install plan: %w", err)
			}
			if dryRun {
				_, err := fmt.Fprintln(stdout, "dry-run: no project files or database changes")
				return err
			}
			if !yes {
				confirmed, err := streamConfirmer(stdin, stderr)(cmd.Context(), "Install App source and update the project runner?")
				if err != nil {
					return err
				}
				if !confirmed {
					_, err := fmt.Fprintln(stdout, "App installation cancelled.")
					return err
				}
			}

			stop := startProgress(cmd.Context(), stderr, "Installing App and preparing project runner")
			defer stop()
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			if err := installCheckout(root, checkout); err != nil {
				return err
			}
			keepTarget := false
			defer func() {
				if !keepTarget {
					_ = os.RemoveAll(target)
				}
			}()

			// Run the same project-level validation used by `dygo app validate`
			// against the installed paths before updating generated code.
			if _, err := project.LoadApps(root); err != nil {
				return err
			}
			before, err := os.ReadFile(runnerFile)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("read project runner: %w", err)
			}
			runnerMissing := os.IsNotExist(err)
			update, err := runnergen.Render(root, runnergen.RenderOptions{})
			if err != nil {
				return fmt.Errorf("prepare project runner: %w", err)
			}
			current, err := os.ReadFile(runnerFile)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("read project runner: %w", err)
			}
			if !bytes.Equal(before, current) || runnerMissing != os.IsNotExist(err) {
				return fmt.Errorf("project runner changed during App installation; rerun dygo app install")
			}
			if err := os.MkdirAll(filepath.Dir(runnerFile), 0o755); err != nil {
				return fmt.Errorf("create project runner directory: %w", err)
			}
			if _, err := runnergen.WriteFileIfChanged(update.RunnerFile, update.Source); err != nil {
				if restoreErr := restoreRunner(runnerFile, before, runnerMissing); restoreErr != nil {
					return fmt.Errorf("write project runner: %v; restore runner: %w", err, restoreErr)
				}
				return err
			}
			keepTarget = true
			stop()
			_, err = fmt.Fprintf(stdout, "App installed: %s %s\nApps prepared: %s\nNext: build the project runner, then run dygo db migrate.\n", checkout.App.Manifest.Name, checkout.App.Manifest.Version, strings.Join(order, ", "))
			return err
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Clone and validate without writing project files")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the local installation confirmation")
	return cmd
}

func cloneAppRepository(ctx context.Context, source string) (appRepositoryCheckout, error) {
	if strings.TrimSpace(source) == "" {
		return appRepositoryCheckout{}, fmt.Errorf("App repository URL is required")
	}
	root, err := os.MkdirTemp("", "dygo-app-repository-*")
	if err != nil {
		return appRepositoryCheckout{}, fmt.Errorf("create App repository checkout: %w", err)
	}
	checkout := filepath.Join(root, "source")
	if err := exec.CommandContext(ctx, "git", "clone", "--quiet", "--depth", "1", "--", source, checkout).Run(); err != nil {
		_ = os.RemoveAll(root)
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return appRepositoryCheckout{}, fmt.Errorf("clone App repository: %w", err)
	}
	app, err := manifest.LoadAppDir(checkout)
	if err != nil {
		_ = os.RemoveAll(root)
		return appRepositoryCheckout{}, fmt.Errorf("validate cloned App: %w", err)
	}
	commitOutput, err := exec.CommandContext(ctx, "git", "-C", checkout, "rev-parse", "HEAD").Output()
	if err != nil {
		_ = os.RemoveAll(root)
		return appRepositoryCheckout{}, fmt.Errorf("read cloned App revision: %w", err)
	}
	return appRepositoryCheckout{Root: root, App: app, Commit: strings.TrimSpace(string(commitOutput))}, nil
}

func installCheckout(root string, checkout appRepositoryCheckout) error {
	target := filepath.Join(root, "apps", checkout.App.Manifest.Name)
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("App destination already exists: %s", relToHooksRoot(root, target))
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check App destination: %w", err)
	}
	stagingRoot := filepath.Join(root, ".dygo", "tmp")
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return fmt.Errorf("create App installation staging directory: %w", err)
	}
	staging, err := os.MkdirTemp(stagingRoot, "app-install-*")
	if err != nil {
		return fmt.Errorf("create App installation staging directory: %w", err)
	}
	defer os.RemoveAll(staging)
	if err := copyAppRepository(checkout.App.Dir, staging); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create project Apps directory: %w", err)
	}
	if err := os.Rename(staging, target); err != nil {
		return fmt.Errorf("install App source: %w", err)
	}
	return nil
}

func copyAppRepository(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.Name() == ".git" && entry.IsDir() {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("install App source: symbolic links are not supported: %s", filepath.ToSlash(relative))
		}
		destination := filepath.Join(target, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if err := os.MkdirAll(destination, info.Mode().Perm()); err != nil {
				return fmt.Errorf("create App directory %s: %w", filepath.ToSlash(relative), err)
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("install App source: unsupported file type: %s", filepath.ToSlash(relative))
		}
		if err := copyAppFile(path, destination, info.Mode().Perm()); err != nil {
			return fmt.Errorf("copy App file %s: %w", filepath.ToSlash(relative), err)
		}
		return nil
	})
}

func copyAppFile(source, destination string, mode fs.FileMode) (copyErr error) {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	defer func() {
		if err := output.Close(); copyErr == nil {
			copyErr = err
		}
		if copyErr != nil {
			_ = os.Remove(destination)
		}
	}()
	_, copyErr = io.Copy(output, input)
	return copyErr
}

func restoreRunner(path string, before []byte, missing bool) error {
	if missing {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, before, 0o644)
}

func appInstallOrder(apps []manifest.LoadedApp, target string) ([]string, error) {
	ordered, err := registry.DependencyOrder(apps)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{target: true}
	found := false
	for index := len(ordered) - 1; index >= 0; index-- {
		app := ordered[index]
		if app.Manifest.Name == target {
			found = true
		}
		if wanted[app.Manifest.Name] {
			for _, dependency := range app.Manifest.Dependencies {
				wanted[dependency] = true
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("app %q was not found in the validated project", target)
	}
	var names []string
	for _, app := range ordered {
		if wanted[app.Manifest.Name] {
			names = append(names, app.Manifest.Name)
		}
	}
	return names, nil
}

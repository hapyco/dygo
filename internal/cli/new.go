package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/dygo-dev/dygo/internal/projectgen"
	"github.com/spf13/cobra"
)

var readBuildInfo = debug.ReadBuildInfo

func newProjectCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	modulePath := ""
	skipTidy := false

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new dygo project",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("detect working directory: %w", err)
			}
			result, err := projectgen.Generate(ctx, projectgen.Options{
				Name:        args[0],
				ModulePath:  modulePath,
				WorkingDir:  wd,
				DygoVersion: dygoVersionForNew(),
				SkipTidy:    skipTidy,
			})
			if err != nil {
				return fmt.Errorf("create project: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "created dygo project: %s\n", result.Name); err != nil {
				return fmt.Errorf("write new project output: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "path: %s\n", relToNewWorkingDir(wd, result.Path)); err != nil {
				return fmt.Errorf("write new project output: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "module: %s\n", result.ModulePath); err != nil {
				return fmt.Errorf("write new project output: %w", err)
			}
			if _, err := fmt.Fprintln(stdout, "secrets: initialized"); err != nil {
				return fmt.Errorf("write new project output: %w", err)
			}
			if result.TidyRun {
				if _, err := fmt.Fprintln(stdout, "dependencies: tidy complete"); err != nil {
					return fmt.Errorf("write new project output: %w", err)
				}
			} else if _, err := fmt.Fprintln(stdout, "dependencies: tidy skipped"); err != nil {
				return fmt.Errorf("write new project output: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "\nnext:\n  cd %s\n  dygo db prepare\n  dygo serve\n", result.Name); err != nil {
				return fmt.Errorf("write new project output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&modulePath, "module", modulePath, "Go module path for the generated project")
	cmd.Flags().BoolVar(&skipTidy, "skip-tidy", skipTidy, "Skip go mod tidy after generating the project")

	return cmd
}

func relToNewWorkingDir(workingDir string, path string) string {
	relative, err := filepath.Rel(workingDir, path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.ToSlash(relative)
}

func dygoVersionForNew() string {
	if cliVersion := strings.TrimSpace(version); cliVersion != "" && cliVersion != "dev" {
		return cliVersion
	}
	buildInfo, ok := readBuildInfo()
	if ok {
		buildVersion := strings.TrimSpace(buildInfo.Main.Version)
		if buildVersion != "" && buildVersion != "(devel)" {
			return buildVersion
		}
	}
	return "dev"
}

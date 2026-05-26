package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/hapyco/dygo/internal/hookgen"
	"github.com/spf13/cobra"
)

func newHooksCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Manage dygo hooks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newHooksGenerateCommand(stdout))

	return cmd
}

func newHooksGenerateCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "generate <app> <entity>",
		Short: "Generate Entity hook scaffold and runner wiring",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			result, err := hookgen.Generate(root, args[0], args[1])
			if err != nil {
				return fmt.Errorf("generate hooks: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "generated hooks for %s/%s\n", result.AppName, result.Entity); err != nil {
				return fmt.Errorf("write hooks output: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "hook: %s (%s)\n", relToHooksRoot(root, result.HookFile), createdStatus(result.HookFileCreated)); err != nil {
				return fmt.Errorf("write hooks output: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "register: %s (%s)\n", relToHooksRoot(root, result.RegisterFile), writtenStatus(result.RegisterFileWritten)); err != nil {
				return fmt.Errorf("write hooks output: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "runner: %s (%s)\n", relToHooksRoot(root, result.RunnerFile), writtenStatus(result.RunnerFileWritten)); err != nil {
				return fmt.Errorf("write hooks output: %w", err)
			}
			return nil
		},
	}
}

func relToHooksRoot(root string, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.ToSlash(relative)
}

func createdStatus(created bool) string {
	if created {
		return "created"
	}
	return "existing"
}

func writtenStatus(written bool) string {
	if written {
		return "updated"
	}
	return "unchanged"
}

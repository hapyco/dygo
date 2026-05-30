package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hapyco/dygo/internal/hookgen"
	"github.com/spf13/cobra"
)

func newHookCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Inspect and maintain dygo hooks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newHookListCommand(stdout))
	cmd.AddCommand(newHookValidateCommand(stdout))
	cmd.AddCommand(newHookSyncCommand(stdout))

	return cmd
}

func newHookListCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List discovered Entity hook files",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			hooks, err := hookgen.Discover(root)
			if err != nil {
				return fmt.Errorf("list hooks: %w", err)
			}
			if len(hooks) == 0 {
				if _, err := fmt.Fprintln(stdout, "No hooks found."); err != nil {
					return fmt.Errorf("write hook output: %w", err)
				}
				return nil
			}
			for _, hook := range hooks {
				if _, err := fmt.Fprintf(stdout, "%s/%s %s register:%s runner:%s\n", hook.AppName, hook.EntityName, relToHooksRoot(root, hook.Path), yesNo(hook.HasRegister), wiredStatus(hook.RunnerWired)); err != nil {
					return fmt.Errorf("write hook output: %w", err)
				}
			}
			return nil
		},
	}
}

func newHookValidateCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate Entity hook files and runner wiring",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			problems, err := hookgen.Validate(root)
			if err != nil {
				return fmt.Errorf("validate hooks: %w", err)
			}
			if len(problems) == 0 {
				if _, err := fmt.Fprintln(stdout, "hooks are valid"); err != nil {
					return fmt.Errorf("write hook output: %w", err)
				}
				return nil
			}
			for _, problem := range problems {
				if _, err := fmt.Fprintln(stdout, problem); err != nil {
					return fmt.Errorf("write hook output: %w", err)
				}
			}
			return fmt.Errorf("hook validation failed with %d problem(s)", len(problems))
		},
	}
}

func newHookSyncCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Update generated runner app-code wiring",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			if dryRun {
				update, err := hookgen.RenderRunner(root)
				if err != nil {
					return fmt.Errorf("plan hook sync: %w", err)
				}
				status, err := runnerChangeStatus(update.RunnerFile, update.Source)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintf(stdout, "runner: %s (%s)\n", relToHooksRoot(root, update.RunnerFile), status); err != nil {
					return fmt.Errorf("write hook output: %w", err)
				}
				return nil
			}
			update, written, err := hookgen.UpdateRunner(root)
			if err != nil {
				return fmt.Errorf("sync hooks: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "runner: %s (%s)\n", relToHooksRoot(root, update.RunnerFile), writtenStatus(written)); err != nil {
				return fmt.Errorf("write hook output: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print runner wiring changes without writing")
	return cmd
}

func runnerChangeStatus(path string, source []byte) (string, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "would create", nil
		}
		return "", fmt.Errorf("read runner file %s: %w", path, err)
	}
	if bytes.Equal(existing, source) {
		return "unchanged", nil
	}
	return "would update", nil
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

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func wiredStatus(value bool) string {
	if value {
		return "wired"
	}
	return "missing"
}

package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dygo-dev/dygo/internal/upgrade"
	"github.com/spf13/cobra"
)

var runUpgrade = upgrade.Run

func newUpgradeCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	options := upgrade.Options{InstallDir: upgrade.DefaultInstallDir}

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade dygo CLI and the current project",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if options.CLIOnly && options.ProjectOnly {
				return fmt.Errorf("--cli-only and --project-only cannot be used together")
			}
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("detect working directory: %w", err)
			}
			executable, _ := os.Executable()
			options.CurrentVersion = currentVersion()
			options.WorkingDir = wd
			options.ExecutablePath = executable
			options.Confirm = streamConfirmer(stdin, stderr)

			result, err := runUpgrade(ctx, options)
			if err != nil {
				return fmt.Errorf("upgrade dygo: %w", err)
			}
			for _, warning := range result.Warnings {
				if _, err := fmt.Fprintf(stderr, "warning: %s\n", warning); err != nil {
					return fmt.Errorf("write upgrade warning: %w", err)
				}
			}
			for _, line := range result.Lines {
				if _, err := fmt.Fprintln(stdout, line); err != nil {
					return fmt.Errorf("write upgrade output: %w", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&options.Check, "check", false, "Check available upgrades without writing files")
	cmd.Flags().StringVar(&options.TargetVersion, "to", "", "Upgrade to a specific dygo version")
	cmd.Flags().BoolVar(&options.CLIOnly, "cli-only", false, "Only upgrade the dygo CLI binary")
	cmd.Flags().BoolVar(&options.ProjectOnly, "project-only", false, "Only upgrade the current dygo project")
	cmd.Flags().BoolVar(&options.DryRun, "dry-run", false, "Show planned upgrade work without writing files")
	cmd.Flags().BoolVar(&options.Yes, "yes", false, "Skip interactive upgrade confirmations")
	cmd.Flags().StringVar(&options.InstallDir, "install-dir", options.InstallDir, "Directory for the managed dygo binary")

	return cmd
}

func streamConfirmer(stdin io.Reader, stderr io.Writer) upgrade.Confirmer {
	return func(_ context.Context, message string) (bool, error) {
		if _, err := fmt.Fprintf(stderr, "%s [y/N]: ", message); err != nil {
			return false, fmt.Errorf("write confirmation prompt: %w", err)
		}
		scanner := bufio.NewScanner(stdin)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return false, fmt.Errorf("read confirmation: %w", err)
			}
			return false, nil
		}
		answer := strings.ToLower(strings.TrimSpace(scanner.Text()))
		return answer == "y" || answer == "yes", nil
	}
}

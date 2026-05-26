package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hapyco/dygo/internal/upgrade"
	"github.com/spf13/cobra"
)

var runUpgrade = upgrade.Run

func newUpgradeCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var options upgrade.Options

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the current dygo project",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("detect working directory: %w", err)
			}
			options.CurrentVersion = currentVersion()
			options.WorkingDir = wd
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
	cmd.Flags().BoolVar(&options.DryRun, "dry-run", false, "Show planned upgrade work without writing files")
	cmd.Flags().BoolVar(&options.Yes, "yes", false, "Skip interactive upgrade confirmations")

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

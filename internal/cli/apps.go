package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/dygo-dev/dygo/internal/project"
	"github.com/spf13/cobra"
)

func newAppsCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "Manage dygo apps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newAppsListCommand(stdout))
	cmd.AddCommand(newAppsValidateCommand(stdout))

	return cmd
}

func newAppsListCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List discovered dygo apps",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			apps, err := project.LoadApps(root)
			if err != nil {
				return err
			}
			if len(apps) == 0 {
				if _, err := fmt.Fprintln(stdout, "No apps found."); err != nil {
					return fmt.Errorf("write apps output: %w", err)
				}
				return nil
			}

			table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(table, "NAME\tVERSION\tLABEL"); err != nil {
				return fmt.Errorf("write apps header: %w", err)
			}
			for _, app := range apps {
				if _, err := fmt.Fprintf(table, "%s\t%s\t%s\n", app.Manifest.Name, app.Manifest.Version, app.Manifest.Label); err != nil {
					return fmt.Errorf("write app row: %w", err)
				}
			}
			if err := table.Flush(); err != nil {
				return fmt.Errorf("flush apps output: %w", err)
			}

			return nil
		},
	}
}

func newAppsValidateCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate discovered dygo apps",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			apps, err := project.LoadApps(root)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(stdout, "%d apps are valid\n", len(apps)); err != nil {
				return fmt.Errorf("write apps validation output: %w", err)
			}
			return nil
		},
	}
}

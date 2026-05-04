package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/dygo-dev/dygo/internal/app/manifest"
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

	return cmd
}

func newAppsListCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List discovered dygo apps",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			apps, err := discoverApps([]string{filepath.Join(".dygo", "apps"), "apps"})
			if err != nil {
				return fmt.Errorf("discover apps: %w", err)
			}
			if err := manifest.ValidateSet(apps); err != nil {
				return fmt.Errorf("validate apps: %w", err)
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

func discoverApps(roots []string) ([]manifest.LoadedApp, error) {
	var apps []manifest.LoadedApp
	for _, root := range roots {
		discovered, err := manifest.Discover(root)
		if err != nil {
			return nil, err
		}
		apps = append(apps, discovered...)
	}

	sort.SliceStable(apps, func(i, j int) bool {
		return apps[i].Manifest.Name < apps[j].Manifest.Name
	})

	return apps, nil
}

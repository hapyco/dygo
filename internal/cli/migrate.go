package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/dygo-dev/dygo/internal/secrets"
	"github.com/spf13/cobra"
)

func newMigrateCommand(ctx context.Context, stdout io.Writer, sync schemaSyncRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Sync dygo metadata to the database",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			result, err := sync.Sync(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("migrate metadata: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "metadata synced: %d entities, %d fields (%s)\n", result.Entities, result.Fields, env); err != nil {
				return fmt.Errorf("write migrate output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

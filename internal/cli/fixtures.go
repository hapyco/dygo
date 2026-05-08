package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/dygo-dev/dygo/internal/secrets"
	"github.com/spf13/cobra"
)

func newFixturesCommand(ctx context.Context, stdout io.Writer, runner fixtureRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fixtures",
		Short: "Manage app-owned fixture records",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newFixturesApplyCommand(ctx, stdout, runner))

	return cmd
}

func newFixturesApplyCommand(ctx context.Context, stdout io.Writer, runner fixtureRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply app-owned fixture records",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			result, err := runner.Apply(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("apply fixtures: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "fixtures applied: %d created, %d updated (%s)\n", result.Created, result.Updated, env); err != nil {
				return fmt.Errorf("write fixtures apply output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

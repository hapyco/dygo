package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/dygo-dev/dygo/internal/config"
	"github.com/dygo-dev/dygo/internal/secrets"
	"github.com/spf13/cobra"
)

func newDBCommand(ctx context.Context, stdout io.Writer, checkDatabase databaseChecker) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Manage dygo database connectivity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newDBCheckCommand(ctx, stdout, checkDatabase))

	return cmd
}

func newDBCheckCommand(ctx context.Context, stdout io.Writer, checkDatabase databaseChecker) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check PostgreSQL connectivity",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, err := secrets.ParseEnvironment(envName)
			if err != nil {
				return err
			}
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(root)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			secretName := cfg.Database.URL.Secret
			databaseURL, err := databaseURLForEnvironment(root, env, secretName)
			if err != nil {
				return fmt.Errorf("read database secret %q for %s: %w", secretName, env, err)
			}
			if err := checkDatabase(ctx, databaseURL); err != nil {
				return fmt.Errorf("check database: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "database connected (%s)\n", env); err != nil {
				return fmt.Errorf("write database check output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", string(secrets.EnvironmentDevelopment), "Environment: development, staging, or production")

	return cmd
}

func databaseURLForEnvironment(root string, env secrets.Environment, secretName string) (string, error) {
	secret, err := secrets.NewStore(root).Get(env, secretName)
	if err != nil {
		return "", err
	}
	return secret.Value, nil
}

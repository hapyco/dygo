package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/dygo-dev/dygo/internal/config"
	"github.com/dygo-dev/dygo/internal/secrets"
	"github.com/spf13/cobra"
)

func newMigrateCommand(ctx context.Context, stdout io.Writer, migrate migrationRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage dygo database migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newMigrateStatusCommand(ctx, stdout, migrate))
	cmd.AddCommand(newMigrateUpCommand(ctx, stdout, migrate))
	cmd.AddCommand(newMigrateDownCommand(ctx, stdout, migrate))

	return cmd
}

func newMigrateStatusCommand(ctx context.Context, stdout io.Writer, migrate migrationRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show database migration status",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, root, databaseURL, err := migrationInputs(envName)
			if err != nil {
				return err
			}
			statuses, err := migrate.Status(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("migration status: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "Migrations (%s)\n", env); err != nil {
				return fmt.Errorf("write migrate status output: %w", err)
			}
			if len(statuses) == 0 {
				if _, err := fmt.Fprintln(stdout, "No migrations found."); err != nil {
					return fmt.Errorf("write migrate status output: %w", err)
				}
				return nil
			}
			if _, err := fmt.Fprintln(stdout, "STATUS   SCOPE       VERSION         NAME"); err != nil {
				return fmt.Errorf("write migrate status output: %w", err)
			}
			for _, status := range statuses {
				state := "pending"
				if status.Applied {
					state = "applied"
				}
				if _, err := fmt.Fprintf(stdout, "%-8s %-11s %-15s %s\n", state, status.Scope, status.Version, status.Name); err != nil {
					return fmt.Errorf("write migrate status output: %w", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newMigrateUpCommand(ctx context.Context, stdout io.Writer, migrate migrationRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Apply pending database migrations",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, root, databaseURL, err := migrationInputs(envName)
			if err != nil {
				return err
			}
			result, err := migrate.Up(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("migrate up: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "applied %d migrations (%s)\n", len(result.Applied), env); err != nil {
				return fmt.Errorf("write migrate up output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newMigrateDownCommand(ctx context.Context, stdout io.Writer, migrate migrationRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	steps := 1

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Roll back applied database migrations",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if steps < 1 {
				return fmt.Errorf("steps must be at least 1")
			}
			env, root, databaseURL, err := migrationInputs(envName)
			if err != nil {
				return err
			}
			result, err := migrate.Down(ctx, root, databaseURL, steps)
			if err != nil {
				return fmt.Errorf("migrate down: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "rolled back %d migrations (%s)\n", len(result.RolledBack), env); err != nil {
				return fmt.Errorf("write migrate down output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().IntVar(&steps, "steps", steps, "Number of applied migrations to roll back")

	return cmd
}

func migrationInputs(envName string) (secrets.Environment, string, string, error) {
	env, err := secrets.ParseEnvironment(envName)
	if err != nil {
		return "", "", "", err
	}
	root, err := workingRootPath()
	if err != nil {
		return "", "", "", err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return "", "", "", fmt.Errorf("load config: %w", err)
	}
	secretName := cfg.Database.URL.Secret
	databaseURL, err := databaseURLForEnvironment(root, env, secretName)
	if err != nil {
		return "", "", "", fmt.Errorf("read database secret %q for %s: %w", secretName, env, err)
	}
	return env, root, databaseURL, nil
}

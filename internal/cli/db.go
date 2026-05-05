package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/dygo-dev/dygo/internal/config"
	"github.com/dygo-dev/dygo/internal/secrets"
	"github.com/spf13/cobra"
)

func newDBCommand(ctx context.Context, stdout io.Writer, database databaseRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Manage dygo database lifecycle",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newDBCheckCommand(ctx, stdout, database))
	cmd.AddCommand(newDBCreateCommand(ctx, stdout, database))
	cmd.AddCommand(newDBDropCommand(ctx, stdout, database))
	cmd.AddCommand(newDBPrepareCommand(ctx, stdout, database))
	cmd.AddCommand(newDBResetCommand(ctx, stdout, database))
	cmd.AddCommand(newDBSchemaCommand(ctx, stdout, database))

	return cmd
}

func newDBCheckCommand(ctx context.Context, stdout io.Writer, database databaseRunner) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check PostgreSQL connectivity",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, _, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			if err := database.Check(ctx, databaseURL); err != nil {
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

func newDBCreateCommand(ctx context.Context, stdout io.Writer, database databaseRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create the configured PostgreSQL database",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, _, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			result, err := database.Create(ctx, databaseURL)
			if err != nil {
				return fmt.Errorf("create database: %w", err)
			}
			action := "created"
			if !result.Changed {
				action = "already exists"
			}
			if _, err := fmt.Fprintf(stdout, "database %s: %s (%s)\n", action, result.Name, env); err != nil {
				return fmt.Errorf("write database create output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newDBDropCommand(ctx context.Context, stdout io.Writer, database databaseRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	force := false

	cmd := &cobra.Command{
		Use:   "drop",
		Short: "Drop the configured PostgreSQL database",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !force {
				return fmt.Errorf("db drop requires --force")
			}
			env, _, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			result, err := database.Drop(ctx, databaseURL)
			if err != nil {
				return fmt.Errorf("drop database: %w", err)
			}
			action := "dropped"
			if !result.Changed {
				action = "does not exist"
			}
			if _, err := fmt.Fprintf(stdout, "database %s: %s (%s)\n", action, result.Name, env); err != nil {
				return fmt.Errorf("write database drop output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&force, "force", force, "Confirm the destructive database drop")

	return cmd
}

func newDBPrepareCommand(ctx context.Context, stdout io.Writer, database databaseRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Create and sync the configured PostgreSQL database",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			result, err := database.Prepare(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("prepare database: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "database prepared: synced %d entities, %d fields (%s)\n", result.Entities, result.Fields, env); err != nil {
				return fmt.Errorf("write database prepare output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newDBResetCommand(ctx context.Context, stdout io.Writer, database databaseRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	force := false

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Drop, create, and sync the configured PostgreSQL database",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !force {
				return fmt.Errorf("db reset requires --force")
			}
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			result, err := database.Reset(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("reset database: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "database reset: synced %d entities, %d fields (%s)\n", result.Entities, result.Fields, env); err != nil {
				return fmt.Errorf("write database reset output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&force, "force", force, "Confirm the destructive database reset")

	return cmd
}

func newDBSchemaCommand(ctx context.Context, stdout io.Writer, database databaseRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Manage the PostgreSQL schema snapshot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newDBSchemaDumpCommand(ctx, stdout, database))

	return cmd
}

func newDBSchemaDumpCommand(ctx context.Context, stdout io.Writer, database databaseRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "dump",
		Short: "Write db/schema.sql from the live database",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			if err := database.SchemaDump(ctx, root, databaseURL); err != nil {
				return fmt.Errorf("dump database schema: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "schema dumped to db/schema.sql (%s)\n", env); err != nil {
				return fmt.Errorf("write schema dump output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func databaseInputs(envName string) (secrets.Environment, string, string, error) {
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

func databaseURLForEnvironment(root string, env secrets.Environment, secretName string) (string, error) {
	secret, err := secrets.NewStore(root).Get(env, secretName)
	if err != nil {
		return "", err
	}
	return secret.Value, nil
}

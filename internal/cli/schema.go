package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/dygo-dev/dygo/internal/db"
	"github.com/dygo-dev/dygo/internal/secrets"
	"github.com/spf13/cobra"
)

func newSchemaCommand(ctx context.Context, stdout io.Writer, sync schemaSyncRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Manage explicit schema cleanup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newSchemaPruneCommand(ctx, stdout, sync))

	return cmd
}

func newSchemaPruneCommand(ctx context.Context, stdout io.Writer, sync schemaSyncRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	force := false

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Preview or apply destructive cleanup for metadata-orphaned schema objects",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			if force {
				result, err := sync.Prune(ctx, root, databaseURL)
				if err != nil {
					return fmt.Errorf("schema prune: %w", err)
				}
				if result.Operations == 0 {
					if _, err := fmt.Fprintf(stdout, "no schema objects to prune (%s)\n", env); err != nil {
						return fmt.Errorf("write schema prune output: %w", err)
					}
					return nil
				}
				if _, err := fmt.Fprintf(stdout, "schema pruned: %d destructive %s (%s)\n", result.Operations, noun(result.Operations, "operation"), env); err != nil {
					return fmt.Errorf("write schema prune output: %w", err)
				}
				return nil
			}

			plan, err := sync.PrunePlan(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("schema prune: %w", err)
			}
			if err := writeSchemaPrunePlan(stdout, env, plan); err != nil {
				return fmt.Errorf("write schema prune output: %w", err)
			}
			return plan.BlockerError()
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&force, "force", force, "Apply the destructive schema prune plan")

	return cmd
}

func writeSchemaPrunePlan(stdout io.Writer, env secrets.Environment, plan db.SchemaPrunePlan) error {
	if len(plan.Operations) == 0 && len(plan.Diagnostics) == 0 {
		_, err := fmt.Fprintf(stdout, "no schema objects to prune (%s)\n", env)
		return err
	}
	if _, err := fmt.Fprintf(stdout, "schema prune plan (%s)\n", env); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "destructive operations: %d\n", len(plan.Operations)); err != nil {
		return err
	}
	if len(plan.Diagnostics) > 0 {
		if _, err := fmt.Fprintf(stdout, "blockers: %d\n", len(plan.Diagnostics)); err != nil {
			return err
		}
	}
	if len(plan.Operations) > 0 {
		if _, err := fmt.Fprintln(stdout, "\ndestructive operations:"); err != nil {
			return err
		}
		for _, operation := range plan.Operations {
			if _, err := fmt.Fprintf(stdout, "- %s\n", operation.Description); err != nil {
				return err
			}
		}
	}
	if len(plan.Diagnostics) > 0 {
		if _, err := fmt.Fprintln(stdout, "\nblockers:"); err != nil {
			return err
		}
		for _, diagnostic := range plan.Diagnostics {
			if _, err := fmt.Fprintf(stdout, "- %s\n", diagnostic.String()); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(stdout, "\n%s\n", db.SchemaPruneBlockerHelp); err != nil {
			return err
		}
		return nil
	}
	_, err := fmt.Fprintln(stdout, "\nrerun with --force to apply")
	return err
}

package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/dygo-dev/dygo/internal/db"
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
			if _, err := fmt.Fprintln(stdout, migrateResultLine(result, env)); err != nil {
				return fmt.Errorf("write migrate output: %w", err)
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.AddCommand(newMigratePlanCommand(ctx, stdout, sync, &envName))

	return cmd
}

func newMigratePlanCommand(ctx context.Context, stdout io.Writer, sync schemaSyncRunner, envName *string) *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Preview metadata schema changes",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, root, databaseURL, err := databaseInputs(*envName)
			if err != nil {
				return err
			}
			plan, err := sync.Plan(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("plan metadata: %w", err)
			}
			if err := writeSchemaPlan(stdout, env, plan); err != nil {
				return fmt.Errorf("write migrate plan output: %w", err)
			}
			return nil
		},
	}
}

func migrateResultLine(result db.SchemaSyncResult, env secrets.Environment) string {
	return fmt.Sprintf("metadata synced: %d %s, %d %s, %d %s, %d schema %s (%s)", result.Apps, noun(result.Apps, "app"), result.Entities, noun(result.Entities, "entity"), result.Fields, noun(result.Fields, "field"), result.Operations, noun(result.Operations, "operation"), env)
}

func writeSchemaPlan(stdout io.Writer, env secrets.Environment, plan db.SchemaPlan) error {
	if _, err := fmt.Fprintf(stdout, "metadata schema plan (%s)\n", env); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "safe operations: %d\n", len(plan.Operations)); err != nil {
		return err
	}
	unsafeCount, unsupportedCount := schemaDiagnosticCounts(plan)
	if _, err := fmt.Fprintf(stdout, "unsafe diagnostics: %d\n", unsafeCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "unsupported diagnostics: %d\n", unsupportedCount); err != nil {
		return err
	}
	if len(plan.Operations) == 0 && len(plan.Diagnostics) == 0 {
		_, err := fmt.Fprintln(stdout, "\nno schema changes")
		return err
	}
	if len(plan.Operations) > 0 {
		if _, err := fmt.Fprintln(stdout, "\nsafe operations:"); err != nil {
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
	}
	return nil
}

func schemaDiagnosticCounts(plan db.SchemaPlan) (int, int) {
	unsafeCount := 0
	unsupportedCount := 0
	for _, diagnostic := range plan.Diagnostics {
		switch diagnostic.Classification {
		case db.SchemaDiagnosticUnsafe:
			unsafeCount++
		case db.SchemaDiagnosticUnsupported:
			unsupportedCount++
		}
	}
	return unsafeCount, unsupportedCount
}

func noun(count int, singular string) string {
	if count == 1 {
		return singular
	}
	if singular == "entity" {
		return "entities"
	}
	return singular + "s"
}

package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/hapyco/dygo/internal/config"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/fixtures"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/spf13/cobra"
)

func newDBCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, database databaseRunner, sync schemaSyncRunner, fixture fixtureRunner) *cobra.Command {
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
	cmd.AddCommand(newDBDropCommand(ctx, stdin, stdout, stderr, database))
	cmd.AddCommand(newDBMigrateCommand(ctx, stdin, stdout, stderr, sync, fixture))
	cmd.AddCommand(newDBPruneCommand(ctx, stdin, stdout, stderr, sync))
	cmd.AddCommand(newDBResetCommand(ctx, stdin, stdout, stderr, database, sync, fixture))

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

func newDBDropCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, database databaseRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	yes := false
	force := false

	cmd := &cobra.Command{
		Use:   "drop",
		Short: "Drop the configured PostgreSQL database",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			target, err := destructiveDatabaseTarget(envName)
			if err != nil {
				return err
			}
			if err := requireProtectedDestructiveEnv("db drop", target.Env, force); err != nil {
				return err
			}
			if err := writeDBDropPlan(stdout, target); err != nil {
				return err
			}
			if !yes {
				ok, err := confirm(stdin, stderr, "Drop database? [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					if _, err := fmt.Fprintln(stdout, "database drop cancelled"); err != nil {
						return fmt.Errorf("write database drop cancellation: %w", err)
					}
					return nil
				}
			}
			result, err := database.Drop(ctx, target.DatabaseURL)
			if err != nil {
				return fmt.Errorf("drop database: %w", err)
			}
			action := "dropped"
			if !result.Changed {
				action = "does not exist"
			}
			if _, err := fmt.Fprintf(stdout, "database %s: %s (%s)\n", action, result.Name, target.Env); err != nil {
				return fmt.Errorf("write database drop output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&yes, "yes", false, "Drop the database without an interactive prompt")
	cmd.Flags().BoolVar(&force, "force", false, "Allow destructive database drops outside development")

	return cmd
}

func newDBMigrateCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, sync schemaSyncRunner, fixture fixtureRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	yes := false
	dryRun := false

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Apply dygo database migrations",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			plan, err := planDBMigration(ctx, sync, fixture, root, databaseURL)
			if err != nil {
				return db.SanitizeDatabaseError("plan db migrate", databaseURL, err)
			}
			if err := writeDBMigratePlan(stdout, env, plan); err != nil {
				return fmt.Errorf("write db migrate plan: %w", err)
			}
			if dryRun {
				return nil
			}
			if err := plan.Schema.BlockerError(); err != nil {
				return err
			}
			if !yes {
				ok, err := confirm(stdin, stderr, "Apply database migration? [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					if _, err := fmt.Fprintln(stdout, "database migration cancelled"); err != nil {
						return fmt.Errorf("write db migrate cancellation: %w", err)
					}
					return nil
				}
			}
			result, err := applyDBMigration(ctx, sync, fixture, root, databaseURL)
			if err != nil {
				return db.SanitizeDatabaseError("apply db migrate", databaseURL, err)
			}
			if err := writeDBMigrateResult(stdout, env, result); err != nil {
				return fmt.Errorf("write db migrate output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&yes, "yes", false, "Apply database migrations without an interactive prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the database migration plan without writing")

	return cmd
}

func newDBPruneCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, sync schemaSyncRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	yes := false
	dryRun := false
	force := false

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove metadata-orphaned schema objects",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			target, err := destructiveDatabaseTarget(envName)
			if err != nil {
				return err
			}
			if err := requireProtectedDestructiveEnv("db prune", target.Env, force); err != nil {
				return err
			}
			plan, err := sync.PrunePlan(ctx, target.Root, target.DatabaseURL)
			if err != nil {
				return db.SanitizeDatabaseError("plan db prune", target.DatabaseURL, err)
			}
			if err := writeSchemaPrunePlan(stdout, target.Env, plan); err != nil {
				return fmt.Errorf("write db prune plan: %w", err)
			}
			if dryRun {
				return plan.BlockerError()
			}
			if err := plan.BlockerError(); err != nil {
				return err
			}
			if len(plan.Operations) == 0 {
				return nil
			}
			if !yes {
				ok, err := confirm(stdin, stderr, "Prune schema objects? [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					if _, err := fmt.Fprintln(stdout, "database prune cancelled"); err != nil {
						return fmt.Errorf("write db prune cancellation: %w", err)
					}
					return nil
				}
			}
			result, err := sync.Prune(ctx, target.Root, target.DatabaseURL)
			if err != nil {
				return db.SanitizeDatabaseError("apply db prune", target.DatabaseURL, err)
			}
			if result.Operations == 0 {
				if _, err := fmt.Fprintf(stdout, "no schema objects to prune (%s)\n", target.Env); err != nil {
					return fmt.Errorf("write db prune output: %w", err)
				}
				return nil
			}
			if _, err := fmt.Fprintf(stdout, "schema pruned: %d destructive %s (%s)\n", result.Operations, noun(result.Operations, "operation"), target.Env); err != nil {
				return fmt.Errorf("write db prune output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&yes, "yes", false, "Prune schema objects without an interactive prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the schema prune plan without writing")
	cmd.Flags().BoolVar(&force, "force", false, "Allow destructive schema pruning outside development")

	return cmd
}

func newDBResetCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, database databaseRunner, sync schemaSyncRunner, fixture fixtureRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	yes := false
	dryRun := false
	force := false

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Drop, create, and migrate the configured PostgreSQL database",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			target, err := destructiveDatabaseTarget(envName)
			if err != nil {
				return err
			}
			if err := requireProtectedDestructiveEnv("db reset", target.Env, force); err != nil {
				return err
			}
			if err := writeDBResetPlan(stdout, target); err != nil {
				return err
			}
			if dryRun {
				return nil
			}
			if !yes {
				ok, err := confirm(stdin, stderr, "Reset database? [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					if _, err := fmt.Fprintln(stdout, "database reset cancelled"); err != nil {
						return fmt.Errorf("write database reset cancellation: %w", err)
					}
					return nil
				}
			}
			if _, err := database.Drop(ctx, target.DatabaseURL); err != nil {
				return fmt.Errorf("drop database for reset: %w", err)
			}
			if _, err := database.Create(ctx, target.DatabaseURL); err != nil {
				return fmt.Errorf("create database for reset: %w", err)
			}
			result, err := applyDBMigration(ctx, sync, fixture, target.Root, target.DatabaseURL)
			if err != nil {
				return db.SanitizeDatabaseError("migrate reset database", target.DatabaseURL, err)
			}
			if _, err := fmt.Fprintf(stdout, "database reset complete (%s)\n", target.Env); err != nil {
				return fmt.Errorf("write database reset output: %w", err)
			}
			if err := writeDBMigrateResult(stdout, target.Env, result); err != nil {
				return fmt.Errorf("write reset migration output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&yes, "yes", false, "Reset the database without an interactive prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print reset steps without writing")
	cmd.Flags().BoolVar(&force, "force", false, "Allow destructive database resets outside development")

	return cmd
}

type dbMigrationPlan struct {
	PreSync  db.PatchPlan
	Schema   db.SchemaPlan
	PostSync db.PatchPlan
	Fixtures fixtures.Plan
}

type dbMigrationResult struct {
	PreSync  db.PatchApplyResult
	Schema   db.SchemaSyncResult
	PostSync db.PatchApplyResult
	Fixtures fixtures.Result
}

func planDBMigration(ctx context.Context, sync schemaSyncRunner, fixture fixtureRunner, root string, databaseURL string) (dbMigrationPlan, error) {
	preSync, err := sync.PatchPlan(ctx, root, databaseURL, db.PatchPhasePreSync)
	if err != nil {
		return dbMigrationPlan{}, fmt.Errorf("plan pre-sync patches: %w", err)
	}
	schema, err := sync.Plan(ctx, root, databaseURL)
	if err != nil {
		return dbMigrationPlan{}, fmt.Errorf("plan metadata schema: %w", err)
	}
	postSync, err := sync.PatchPlan(ctx, root, databaseURL, db.PatchPhasePostSync)
	if err != nil {
		return dbMigrationPlan{}, fmt.Errorf("plan post-sync patches: %w", err)
	}
	fixturesPlan, err := fixture.Plan(ctx, root)
	if err != nil {
		return dbMigrationPlan{}, fmt.Errorf("plan fixtures: %w", err)
	}
	return dbMigrationPlan{
		PreSync:  preSync,
		Schema:   schema,
		PostSync: postSync,
		Fixtures: fixturesPlan,
	}, nil
}

func applyDBMigration(ctx context.Context, sync schemaSyncRunner, fixture fixtureRunner, root string, databaseURL string) (dbMigrationResult, error) {
	preSync, err := sync.ApplyPatches(ctx, root, databaseURL, db.PatchPhasePreSync, currentVersion())
	if err != nil {
		return dbMigrationResult{}, fmt.Errorf("apply pre-sync patches: %w", err)
	}
	schema, err := sync.Sync(ctx, root, databaseURL)
	if err != nil {
		return dbMigrationResult{}, fmt.Errorf("sync metadata schema: %w", err)
	}
	postSync, err := sync.ApplyPatches(ctx, root, databaseURL, db.PatchPhasePostSync, currentVersion())
	if err != nil {
		return dbMigrationResult{}, fmt.Errorf("apply post-sync patches: %w", err)
	}
	fixtureResult, err := fixture.Apply(ctx, root, databaseURL)
	if err != nil {
		return dbMigrationResult{}, fmt.Errorf("apply fixtures: %w", err)
	}
	return dbMigrationResult{
		PreSync:  preSync,
		Schema:   schema,
		PostSync: postSync,
		Fixtures: fixtureResult,
	}, nil
}

func writeDBMigratePlan(stdout io.Writer, env secrets.Environment, plan dbMigrationPlan) error {
	unsafeCount, unsupportedCount := schemaDiagnosticCounts(plan.Schema)
	if _, err := fmt.Fprintf(stdout, "db migrate plan (%s)\n", env); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "pre-sync patches: %d pending, %d applied\n", len(plan.PreSync.Pending), len(plan.PreSync.Applied)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "schema safe operations: %d\n", len(plan.Schema.Operations)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "schema unsafe diagnostics: %d\n", unsafeCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "schema unsupported diagnostics: %d\n", unsupportedCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "post-sync patches: %d pending, %d applied\n", len(plan.PostSync.Pending), len(plan.PostSync.Applied)); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "fixtures: %d files, %d records\n", plan.Fixtures.FileCount(), plan.Fixtures.RecordCount())
	return err
}

func writeDBMigrateResult(stdout io.Writer, env secrets.Environment, result dbMigrationResult) error {
	if _, err := fmt.Fprintf(stdout, "database migrated (%s)\n", env); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "pre-sync patches applied: %d\n", len(result.PreSync.Applied)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(stdout, migrateResultLine(result.Schema, env)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "post-sync patches applied: %d\n", len(result.PostSync.Applied)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "fixture records: %d created, %d updated\n", result.Fixtures.Created, result.Fixtures.Updated); err != nil {
		return err
	}
	_, err := fmt.Fprintln(stdout, "schema snapshot: refreshed")
	return err
}

func writeDBDropPlan(stdout io.Writer, target destructiveTarget) error {
	_, err := fmt.Fprintf(stdout, "db drop plan (%s)\ndatabase: %s\n", target.Env, target.Database)
	return err
}

func writeDBResetPlan(stdout io.Writer, target destructiveTarget) error {
	_, err := fmt.Fprintf(stdout, "db reset plan (%s)\ndatabase: %s\nsteps: drop, create, migrate\n", target.Env, target.Database)
	return err
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

type destructiveTarget struct {
	Env         secrets.Environment
	Root        string
	DatabaseURL string
	Database    string
}

func destructiveDatabaseTarget(envName string) (destructiveTarget, error) {
	env, root, databaseURL, err := databaseInputs(envName)
	if err != nil {
		return destructiveTarget{}, err
	}
	target, err := db.ParseDatabaseTarget(databaseURL)
	if err != nil {
		return destructiveTarget{}, fmt.Errorf("parse database target: %w", err)
	}
	return destructiveTarget{
		Env:         env,
		Root:        root,
		DatabaseURL: databaseURL,
		Database:    target.Name,
	}, nil
}

func requireProtectedDestructiveEnv(command string, env secrets.Environment, force bool) error {
	if env == secrets.EnvironmentDevelopment || force {
		return nil
	}
	return fmt.Errorf("%s on %s requires --force", command, env)
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
	_, err := fmt.Fprintln(stdout, "\nrerun with --yes to apply")
	return err
}

func migrateResultLine(result db.SchemaSyncResult, env secrets.Environment) string {
	return fmt.Sprintf("metadata synced: %d %s, %d %s, %d %s, %d schema %s (%s)", result.Apps, noun(result.Apps, "app"), result.Entities, noun(result.Entities, "entity"), result.Fields, noun(result.Fields, "field"), result.Operations, noun(result.Operations, "operation"), env)
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

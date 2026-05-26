package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/hapyco/dygo/internal/fixtures"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/spf13/cobra"
)

func newFixtureCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, runner fixtureRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fixture",
		Short: "Manage app-owned fixture records",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newFixtureApplyCommand(ctx, stdin, stdout, stderr, runner))
	cmd.AddCommand(newFixtureValidateCommand(ctx, stdout, runner))

	return cmd
}

func newFixtureApplyCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, runner fixtureRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	yes := false
	dryRun := false

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Plan and apply app-owned fixture records",
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
			plan, err := runner.Plan(ctx, root)
			if err != nil {
				return fmt.Errorf("plan fixtures: %w", err)
			}
			if err := writeFixturePlan(stdout, "fixture apply plan", env, plan); err != nil {
				return err
			}
			if dryRun {
				if _, err := fmt.Fprintln(stdout, "dry-run: no records will be written"); err != nil {
					return fmt.Errorf("write fixture dry-run output: %w", err)
				}
				return nil
			}
			if !yes {
				ok, err := confirm(stdin, stderr, "Apply fixture records? [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("fixture apply canceled")
				}
			}
			_, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			result, err := runner.Apply(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("apply fixture records: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "fixtures applied: %d created, %d updated (%s)\n", result.Created, result.Updated, env); err != nil {
				return fmt.Errorf("write fixtures apply output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&yes, "yes", yes, "Apply fixtures without an interactive prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", dryRun, "Print the fixture apply plan without writing records")

	return cmd
}

func newFixtureValidateCommand(ctx context.Context, stdout io.Writer, runner fixtureRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate app-owned fixture files",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			plan, err := runner.Plan(ctx, root)
			if err != nil {
				return fmt.Errorf("validate fixtures: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "fixtures valid: %d files, %d records\n", plan.FileCount(), plan.RecordCount()); err != nil {
				return fmt.Errorf("write fixture validate output: %w", err)
			}
			return nil
		},
	}

	return cmd
}

func writeFixturePlan(stdout io.Writer, title string, env secrets.Environment, plan fixtures.Plan) error {
	if _, err := fmt.Fprintf(stdout, "%s (%s)\n", title, env); err != nil {
		return fmt.Errorf("write fixture plan output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "files: %d\n", plan.FileCount()); err != nil {
		return fmt.Errorf("write fixture plan output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "records: %d\n", plan.RecordCount()); err != nil {
		return fmt.Errorf("write fixture plan output: %w", err)
	}
	return nil
}

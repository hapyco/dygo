package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hapyco/dygo/internal/access"
	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/spf13/cobra"
)

func newAccessCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, runner accessRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "access",
		Short: "Manage app access metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newAccessValidateCommand(ctx, stdout, runner))
	cmd.AddCommand(newAccessApplyCommand(ctx, stdin, stdout, stderr, runner))
	cmd.AddCommand(newAccessListCommand(ctx, stdout, runner))
	cmd.AddCommand(newAccessShowCommand(ctx, stdout, runner))
	cmd.AddCommand(newAccessRolesCommand(ctx, stdout, runner))
	cmd.AddCommand(newAccessExportCommand(ctx, stdin, stdout, stderr, runner))

	return cmd
}

func newAccessValidateCommand(ctx context.Context, stdout io.Writer, runner accessRunner) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate app access metadata files",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			plan, err := runner.Plan(ctx, root, nil)
			if err != nil {
				return fmt.Errorf("validate access: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "access valid: %d roles, %d policy files, %d grants\n", len(plan.Roles), len(plan.Policies), len(plan.Grants)); err != nil {
				return fmt.Errorf("write access validate output: %w", err)
			}
			return nil
		},
	}
}

func newAccessApplyCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, runner accessRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	yes := false
	dryRun := false

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply app access metadata to the database",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			plan, err := runner.ApplyPlan(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("plan access: %w", err)
			}
			if err := writeAccessPlan(stdout, "access apply plan", env, plan); err != nil {
				return err
			}
			if dryRun {
				if _, err := fmt.Fprintln(stdout, "dry-run: no access records will be written"); err != nil {
					return fmt.Errorf("write access dry-run output: %w", err)
				}
				return nil
			}
			if !yes {
				ok, err := confirm(stdin, stderr, "Apply access metadata? [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("access apply canceled")
				}
			}
			result, err := runner.Apply(ctx, root, databaseURL)
			if err != nil {
				return fmt.Errorf("apply access metadata: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "access applied: %d roles, %d permissions (%s)\n", result.Roles, result.Permissions, env); err != nil {
				return fmt.Errorf("write access apply output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&yes, "yes", yes, "Apply access metadata without an interactive prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", dryRun, "Print the access apply plan without writing records")
	return cmd
}

func newAccessListCommand(ctx context.Context, stdout io.Writer, runner accessRunner) *cobra.Command {
	return &cobra.Command{
		Use:   "list [app]",
		Short: "List discovered access metadata files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			plan, err := runner.Plan(ctx, root, nil)
			if err != nil {
				return fmt.Errorf("list access: %w", err)
			}
			appFilter := ""
			if len(args) == 1 {
				if err := shape.ValidateMetadataName("app", args[0]); err != nil {
					return err
				}
				appFilter = args[0]
			}
			return writeAccessList(stdout, plan, appFilter)
		},
	}
}

func newAccessShowCommand(ctx context.Context, stdout io.Writer, runner accessRunner) *cobra.Command {
	return &cobra.Command{
		Use:   "show <app>/<entity>",
		Short: "Show resolved access metadata for one Entity",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			target, err := shape.ParseAppRef(args[0])
			if err != nil {
				return err
			}
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			plan, err := runner.Plan(ctx, root, nil)
			if err != nil {
				return fmt.Errorf("show access: %w", err)
			}
			return writeAccessShow(stdout, plan, target)
		},
	}
}

func newAccessRolesCommand(ctx context.Context, stdout io.Writer, runner accessRunner) *cobra.Command {
	return &cobra.Command{
		Use:   "roles [app]",
		Short: "List app-contributed roles",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			plan, err := runner.Plan(ctx, root, nil)
			if err != nil {
				return fmt.Errorf("list access roles: %w", err)
			}
			appFilter := ""
			if len(args) == 1 {
				if err := shape.ValidateMetadataName("app", args[0]); err != nil {
					return err
				}
				appFilter = args[0]
			}
			return writeAccessRoles(stdout, plan, appFilter)
		},
	}
}

func newAccessExportCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, runner accessRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	destinationApp := ""
	yes := false
	dryRun := false

	cmd := &cobra.Command{
		Use:   "export [app/entity]",
		Short: "Export live access records into app access metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if strings.TrimSpace(destinationApp) == "" {
				return fmt.Errorf("pass --in <app>")
			}
			if err := shape.ValidateMetadataName("destination app", destinationApp); err != nil {
				return err
			}
			var target *shape.AppRef
			if len(args) == 1 {
				parsed, err := shape.ParseAppRef(args[0])
				if err != nil {
					return err
				}
				target = &parsed
			}
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			plan, err := runner.ExportPlan(ctx, root, databaseURL, target, destinationApp)
			if err != nil {
				return fmt.Errorf("plan access export: %w", err)
			}
			if err := writeAccessExportPlan(stdout, env, plan); err != nil {
				return err
			}
			if dryRun {
				if _, err := fmt.Fprintln(stdout, "dry-run: no access files will be written"); err != nil {
					return fmt.Errorf("write access export dry-run output: %w", err)
				}
				return nil
			}
			if plan.FileCount() == 0 {
				if _, err := fmt.Fprintf(stdout, "access export: no changes (%s)\n", env); err != nil {
					return fmt.Errorf("write access export output: %w", err)
				}
				return nil
			}
			if !yes {
				ok, err := confirm(stdin, stderr, "Export access metadata? [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					if _, err := fmt.Fprintln(stdout, "access export cancelled"); err != nil {
						return fmt.Errorf("write access export cancellation: %w", err)
					}
					return nil
				}
			}
			result, err := runner.WriteExportPlan(ctx, plan)
			if err != nil {
				return fmt.Errorf("write access export: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "access exported: %d files, %d roles, %d policy items (%s)\n", result.FilesWritten, result.RolesWritten, result.PolicyItemsWritten, env); err != nil {
				return fmt.Errorf("write access export output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().StringVar(&destinationApp, "in", "", "Destination app for exported access metadata")
	cmd.Flags().BoolVar(&yes, "yes", yes, "Export access metadata without an interactive prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", dryRun, "Print the access export plan without writing files")
	return cmd
}

func writeAccessPlan(stdout io.Writer, title string, env secrets.Environment, plan access.Plan) error {
	if _, err := fmt.Fprintf(stdout, "%s (%s)\n", title, env); err != nil {
		return fmt.Errorf("write access plan output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "roles: %d\n", len(plan.Roles)); err != nil {
		return fmt.Errorf("write access plan output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "policy files: %d\n", len(plan.Policies)); err != nil {
		return fmt.Errorf("write access plan output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "permissions: %d\n", len(plan.Grants)); err != nil {
		return fmt.Errorf("write access plan output: %w", err)
	}
	return nil
}

func writeAccessExportPlan(stdout io.Writer, env secrets.Environment, plan access.ExportPlan) error {
	if _, err := fmt.Fprintf(stdout, "access export plan (%s)\n", env); err != nil {
		return fmt.Errorf("write access export output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "destination: %s\n", plan.DestinationApp); err != nil {
		return fmt.Errorf("write access export output: %w", err)
	}
	if plan.Target != nil {
		if _, err := fmt.Fprintf(stdout, "target: %s/%s\n", plan.Target.App, plan.Target.Name); err != nil {
			return fmt.Errorf("write access export output: %w", err)
		}
	} else if _, err := fmt.Fprintln(stdout, "target: roles"); err != nil {
		return fmt.Errorf("write access export output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "files: %d\n", plan.FileCount()); err != nil {
		return fmt.Errorf("write access export output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "roles: %d\n", plan.RoleCount()); err != nil {
		return fmt.Errorf("write access export output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "policy items: %d\n", plan.PolicyCount()); err != nil {
		return fmt.Errorf("write access export output: %w", err)
	}
	for _, file := range plan.Files {
		switch file.Kind {
		case "roles":
			if _, err := fmt.Fprintf(stdout, "file: %s (%d roles)\n", file.ProjectPath, file.Roles); err != nil {
				return fmt.Errorf("write access export output: %w", err)
			}
		case "policy":
			if _, err := fmt.Fprintf(stdout, "file: %s (%d policy items)\n", file.ProjectPath, file.PolicyItems); err != nil {
				return fmt.Errorf("write access export output: %w", err)
			}
		default:
			if _, err := fmt.Fprintf(stdout, "file: %s\n", file.ProjectPath); err != nil {
				return fmt.Errorf("write access export output: %w", err)
			}
		}
	}
	return nil
}

func writeAccessList(stdout io.Writer, plan access.Plan, appFilter string) error {
	for _, file := range plan.Policies {
		if appFilter != "" && file.ContributorApp != appFilter {
			continue
		}
		if _, err := fmt.Fprintf(stdout, "%s %s/%s %s\n", file.ContributorApp, file.TargetApp, file.Entity, file.ProjectPath); err != nil {
			return fmt.Errorf("write access list output: %w", err)
		}
	}
	return nil
}

func writeAccessShow(stdout io.Writer, plan access.Plan, target shape.AppRef) error {
	if _, err := fmt.Fprintf(stdout, "entity: %s/%s\n", target.App, target.Name); err != nil {
		return fmt.Errorf("write access show output: %w", err)
	}
	for _, grant := range plan.Grants {
		if grant.TargetApp != target.App || grant.Entity != target.Name {
			continue
		}
		if _, err := fmt.Fprintf(stdout, "  - %s: %s\n", grant.Role, accessActionList(grant.Can)); err != nil {
			return fmt.Errorf("write access show output: %w", err)
		}
	}
	return nil
}

func writeAccessRoles(stdout io.Writer, plan access.Plan, appFilter string) error {
	for _, role := range plan.Roles {
		if appFilter != "" && role.App != appFilter {
			continue
		}
		if _, err := fmt.Fprintf(stdout, "%s %s %q\n", role.App, role.Name, role.Label); err != nil {
			return fmt.Errorf("write access roles output: %w", err)
		}
	}
	return nil
}

func accessActionList(actions []permissions.Action) string {
	if len(actions) == 0 {
		return "(none)"
	}
	values := make([]string, len(actions))
	for index, action := range actions {
		values[index] = string(action)
	}
	return strings.Join(values, " ")
}

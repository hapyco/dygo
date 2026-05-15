package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dygo-dev/dygo/internal/db"
	"github.com/dygo-dev/dygo/internal/secrets"
	"github.com/spf13/cobra"
)

func newPatchesCommand(ctx context.Context, stdout io.Writer, sync schemaSyncRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patches",
		Short: "Plan explicit app patches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newPatchesPlanCommand(ctx, stdout, sync))

	return cmd
}

func newPatchesPlanCommand(ctx context.Context, stdout io.Writer, sync schemaSyncRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	phase := ""

	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Preview pending explicit app patches",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if phase == "" {
				return fmt.Errorf("patches plan requires --phase %s or %s", db.PatchPhasePreSync, db.PatchPhasePostSync)
			}
			if phase != db.PatchPhasePreSync && phase != db.PatchPhasePostSync {
				return fmt.Errorf("patches plan --phase must be %s or %s", db.PatchPhasePreSync, db.PatchPhasePostSync)
			}
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			plan, err := sync.PatchPlan(ctx, root, databaseURL, phase)
			if err != nil {
				return fmt.Errorf("plan patches: %w", err)
			}
			if err := writePatchPlan(stdout, env, plan); err != nil {
				return fmt.Errorf("write patches plan output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().StringVar(&phase, "phase", phase, "Patch phase: pre-sync or post-sync")

	return cmd
}

func writePatchPlan(stdout io.Writer, env secrets.Environment, plan db.PatchPlan) error {
	if len(plan.Pending) == 0 && len(plan.Applied) == 0 {
		_, err := fmt.Fprintf(stdout, "no patches for %s (%s)\n", plan.Phase, env)
		return err
	}
	if _, err := fmt.Fprintf(stdout, "patches plan (%s, %s)\n", env, plan.Phase); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "pending patches: %d\n", len(plan.Pending)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "applied patches: %d\n", len(plan.Applied)); err != nil {
		return err
	}

	if len(plan.Pending) > 0 {
		if _, err := fmt.Fprintln(stdout, "\npending:"); err != nil {
			return err
		}
		for _, patch := range plan.Pending {
			if _, err := fmt.Fprintf(stdout, "- %s/%s %s\n", patch.AppName, patch.PatchID, patchDisplayPath(patch.AppRelativePath, patch.Path)); err != nil {
				return err
			}
			if patch.Description != "" {
				if _, err := fmt.Fprintf(stdout, "  %s\n", patch.Description); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(stdout, "  operations:"); err != nil {
				return err
			}
			for _, operation := range patch.Operations {
				if _, err := fmt.Fprintf(stdout, "  - %s: %s\n", operation.Type, operation.Description); err != nil {
					return err
				}
				if operation.SQL != "" {
					if _, err := fmt.Fprintln(stdout, "    sql:"); err != nil {
						return err
					}
					if err := writeIndentedSQL(stdout, operation.SQL, "      "); err != nil {
						return err
					}
				}
			}
		}
	}

	if len(plan.Applied) > 0 {
		if _, err := fmt.Fprintln(stdout, "\napplied:"); err != nil {
			return err
		}
		for _, patch := range plan.Applied {
			if _, err := fmt.Fprintf(stdout, "- %s/%s %s%s\n", patch.AppName, patch.PatchID, patchDisplayPath(patch.AppRelativePath, patch.Path), appliedPatchSuffix(patch)); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeIndentedSQL(stdout io.Writer, sql string, indent string) error {
	sql = strings.TrimRight(sql, "\n")
	for _, line := range strings.Split(sql, "\n") {
		if _, err := fmt.Fprintf(stdout, "%s%s\n", indent, line); err != nil {
			return err
		}
	}
	return nil
}

func patchDisplayPath(appRelativePath string, path string) string {
	if appRelativePath != "" {
		return appRelativePath
	}
	return path
}

func appliedPatchSuffix(patch db.AppliedPatch) string {
	suffix := ""
	if !patch.Run.AppliedAt.IsZero() {
		suffix += " applied " + patch.Run.AppliedAt.UTC().Format(time.RFC3339)
	}
	if patch.Run.DygoVersion != "" {
		suffix += " dygo " + patch.Run.DygoVersion
	}
	return suffix
}

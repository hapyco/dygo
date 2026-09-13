package cli

import (
	"fmt"
	"io"

	scaffold "github.com/hapyco/dygo/internal/generate"
	"github.com/hapyco/dygo/internal/hookgen"
	"github.com/hapyco/dygo/internal/jobgen"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/spf13/cobra"
)

func newGenerateCommand(stdout io.Writer) *cobra.Command {
	cmd := newCommandGroup("generate", "Generate dygo source scaffolding", "g")

	cmd.AddCommand(newGenerateAppCommand(stdout))
	cmd.AddCommand(newGenerateEntityCommand(stdout))
	cmd.AddCommand(newGenerateCollectionCommand(stdout))
	cmd.AddCommand(newGenerateHookCommand(stdout))
	cmd.AddCommand(newGenerateJobCommand(stdout))
	cmd.AddCommand(newGenerateFixtureCommand(stdout))
	cmd.AddCommand(newGenerateTestCommand(stdout))

	return cmd
}

func newGenerateAppCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var force bool
	var noAccess bool

	cmd := &cobra.Command{
		Use:   "app <app>",
		Short: "Generate an app skeleton",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := shape.ValidateMetadataName("app", args[0]); err != nil {
				return err
			}
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			plan, err := scaffold.App(scaffold.Options{Root: root, DryRun: dryRun, Force: force, NoAccess: noAccess}, args[0])
			if err != nil {
				return fmt.Errorf("generate app: %w", err)
			}
			return writeGeneratePlan(stdout, "generated app "+args[0], plan)
		},
	}
	addScaffoldWriteFlags(cmd, &dryRun, &force)
	cmd.Flags().BoolVar(&noAccess, "no-access", false, "skip access metadata skeleton creation")
	return cmd
}

func newGenerateEntityCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var force bool
	var noFixture bool
	var noAccess bool

	cmd := &cobra.Command{
		Use:   "entity <app>/<entity>",
		Short: "Generate the standard Entity bundle",
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
			if err := requireGenerateApp(root, target.App); err != nil {
				return err
			}
			plan, err := scaffold.Entity(scaffold.Options{Root: root, DryRun: dryRun, Force: force, NoAccess: noAccess}, target, !noFixture)
			if err != nil {
				return fmt.Errorf("generate entity: %w", err)
			}
			return writeGeneratePlan(stdout, "generated entity "+args[0], plan)
		},
	}
	addScaffoldWriteFlags(cmd, &dryRun, &force)
	cmd.Flags().BoolVar(&noFixture, "no-fixture", false, "skip fixture skeleton creation")
	cmd.Flags().BoolVar(&noAccess, "no-access", false, "skip access metadata skeleton creation")
	return cmd
}

func newGenerateCollectionCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var force bool

	cmd := &cobra.Command{
		Use:   "collection <app>/<collection>",
		Short: "Generate reusable collection row Entity metadata",
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
			if err := requireGenerateApp(root, target.App); err != nil {
				return err
			}
			plan, err := scaffold.Collection(scaffold.Options{Root: root, DryRun: dryRun, Force: force}, target)
			if err != nil {
				return fmt.Errorf("generate collection: %w", err)
			}
			return writeGeneratePlan(stdout, "generated collection "+args[0], plan)
		},
	}
	addScaffoldWriteFlags(cmd, &dryRun, &force)
	return cmd
}

func newGenerateHookCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var force bool

	cmd := &cobra.Command{
		Use:   "hook <app>/<entity>",
		Short: "Generate Entity hook scaffold and runner wiring",
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
			result, err := hookgen.GenerateWithOptions(hookgen.GenerateOptions{
				Root:       root,
				AppName:    target.App,
				EntityName: target.Name,
				DryRun:     dryRun,
			})
			if err != nil {
				return fmt.Errorf("generate hook: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "generated hook for %s/%s\n", result.AppName, result.Entity); err != nil {
				return fmt.Errorf("write generate output: %w", err)
			}
			return writeGenerateHookResult(stdout, root, result)
		},
	}
	addScaffoldWriteFlags(cmd, &dryRun, &force)
	return cmd
}

func newGenerateJobCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var force bool

	cmd := &cobra.Command{
		Use:   "job <app>/<job>",
		Short: "Generate Job scaffold and runner wiring",
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
			result, err := jobgen.GenerateWithOptions(jobgen.GenerateOptions{
				Root:    root,
				AppName: target.App,
				JobName: target.Name,
				DryRun:  dryRun,
				Force:   force,
			})
			if err != nil {
				return fmt.Errorf("generate job: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "generated job for %s/%s\n", result.AppName, result.JobName); err != nil {
				return fmt.Errorf("write generate output: %w", err)
			}
			return writeGenerateJobResult(stdout, root, result)
		},
	}
	addScaffoldWriteFlags(cmd, &dryRun, &force)
	return cmd
}

func newGenerateFixtureCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var force bool

	cmd := &cobra.Command{
		Use:   "fixture <app>/<entity>",
		Short: "Generate a fixture skeleton for an existing Entity",
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
			if err := requireGenerateEntity(root, target); err != nil {
				return err
			}
			plan, err := scaffold.Fixture(scaffold.Options{Root: root, DryRun: dryRun, Force: force}, target)
			if err != nil {
				return fmt.Errorf("generate fixture: %w", err)
			}
			return writeGeneratePlan(stdout, "generated fixture "+args[0], plan)
		},
	}
	addScaffoldWriteFlags(cmd, &dryRun, &force)
	return cmd
}

func newGenerateTestCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var force bool

	cmd := &cobra.Command{
		Use:   "test <app>/<entity>",
		Short: "Generate Go test boilerplate for an existing Entity",
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
			if err := requireGenerateEntity(root, target); err != nil {
				return err
			}
			plan, err := scaffold.Test(scaffold.Options{Root: root, DryRun: dryRun, Force: force}, target)
			if err != nil {
				return fmt.Errorf("generate test: %w", err)
			}
			return writeGeneratePlan(stdout, "generated test "+args[0], plan)
		},
	}
	addScaffoldWriteFlags(cmd, &dryRun, &force)
	return cmd
}

func addScaffoldWriteFlags(cmd *cobra.Command, dryRun *bool, force *bool) {
	cmd.Flags().BoolVar(dryRun, "dry-run", false, "print files that would be written without writing")
	cmd.Flags().BoolVar(force, "force", false, "overwrite dygo-generated files only")
}

func writeGeneratePlan(stdout io.Writer, title string, plan scaffold.Plan) error {
	if _, err := fmt.Fprintln(stdout, title); err != nil {
		return fmt.Errorf("write generate output: %w", err)
	}
	for _, action := range plan.Actions {
		if _, err := fmt.Fprintf(stdout, "file: %s (%s)\n", action.Path, action.Status); err != nil {
			return fmt.Errorf("write generate output: %w", err)
		}
	}
	return nil
}

func writeGenerateHookResult(stdout io.Writer, root string, result hookgen.Result) error {
	if _, err := fmt.Fprintf(stdout, "hook: %s (%s)\n", relToHooksRoot(root, result.HookFile), hookFileResultStatus(result)); err != nil {
		return fmt.Errorf("write generate output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "runner: %s (%s)\n", relToHooksRoot(root, result.RunnerFile), runnerResultStatus(result)); err != nil {
		return fmt.Errorf("write generate output: %w", err)
	}
	return nil
}

func hookFileResultStatus(result hookgen.Result) string {
	return result.HookFileStatus
}

func runnerResultStatus(result hookgen.Result) string {
	return result.RunnerFileStatus
}

func writeGenerateJobResult(stdout io.Writer, root string, result jobgen.Result) error {
	lines := []struct {
		label  string
		path   string
		status string
	}{
		{label: "job", path: result.JobFile, status: result.JobFileStatus},
		{label: "run", path: result.RunFile, status: result.RunFileStatus},
		{label: "runner", path: result.RunnerFile, status: result.RunnerFileStatus},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(stdout, "%s: %s (%s)\n", line.label, relToHooksRoot(root, line.path), line.status); err != nil {
			return fmt.Errorf("write generate output: %w", err)
		}
	}
	return nil
}

func requireGenerateApp(root string, appName string) error {
	apps, err := project.LoadApps(root)
	if err != nil {
		return err
	}
	for _, app := range apps {
		if app.Manifest.Name == appName {
			return nil
		}
	}
	return fmt.Errorf("app %q not found; run dygo generate app %s first", appName, appName)
}

func requireGenerateEntity(root string, target shape.AppRef) error {
	metadata, err := project.LoadMetadata(root)
	if err != nil {
		return err
	}
	entity, ok := findEntity(metadata.Entities, target)
	if !ok {
		return fmt.Errorf("entity %q not found; run dygo generate entity %s/%s first", target.Name, target.App, target.Name)
	}
	if entity.IsCollection() {
		return fmt.Errorf("entity %q in app %q is a collection; generate files for the parent Entity that owns collection row usage", target.Name, target.App)
	}
	return nil
}

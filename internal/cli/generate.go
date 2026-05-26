package cli

import (
	"fmt"
	"io"
	"path/filepath"

	scaffold "github.com/hapyco/dygo/internal/generate"
	"github.com/hapyco/dygo/internal/hookgen"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/spf13/cobra"
)

func newGenerateCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate",
		Aliases: []string{"g"},
		Short:   "Generate dygo source scaffolding",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newGenerateAppCommand(stdout))
	cmd.AddCommand(newGenerateEntityCommand(stdout))
	cmd.AddCommand(newGenerateCollectionCommand(stdout))
	cmd.AddCommand(newGenerateHookCommand(stdout))
	cmd.AddCommand(newGenerateFixtureCommand(stdout))
	cmd.AddCommand(newGenerateTestCommand(stdout))

	return cmd
}

func newGenerateAppCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var force bool

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
			plan, err := scaffold.App(scaffold.Options{Root: root, DryRun: dryRun, Force: force}, args[0])
			if err != nil {
				return fmt.Errorf("generate app: %w", err)
			}
			return writeGeneratePlan(stdout, "generated app "+args[0], plan)
		},
	}
	addScaffoldWriteFlags(cmd, &dryRun, &force)
	return cmd
}

func newGenerateEntityCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var force bool
	var noHook bool
	var noFixture bool
	var noTest bool

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
			plan, err := scaffold.Entity(scaffold.Options{Root: root, DryRun: dryRun, Force: force}, target, !noFixture, !noTest)
			if err != nil {
				return fmt.Errorf("generate entity: %w", err)
			}
			if err := writeGeneratePlan(stdout, "generated entity "+args[0], plan); err != nil {
				return err
			}
			if noHook {
				return nil
			}
			if dryRun {
				return writeGenerateHookDryRun(stdout, root, target)
			}
			// TODO(scaffold): make Entity bundle and hook runner writes atomic once
			// hook planning moves into internal/generate.
			result, err := hookgen.Generate(root, target.App, target.Name)
			if err != nil {
				return fmt.Errorf("generate hook: %w", err)
			}
			return writeGenerateHookResult(stdout, root, result)
		},
	}
	addScaffoldWriteFlags(cmd, &dryRun, &force)
	cmd.Flags().BoolVar(&noHook, "no-hook", false, "skip hook scaffolding and runner wiring")
	cmd.Flags().BoolVar(&noFixture, "no-fixture", false, "skip fixture skeleton creation")
	cmd.Flags().BoolVar(&noTest, "no-test", false, "skip Go test boilerplate")
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
	return &cobra.Command{
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
			result, err := hookgen.Generate(root, target.App, target.Name)
			if err != nil {
				return fmt.Errorf("generate hook: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "generated hook for %s/%s\n", result.AppName, result.Entity); err != nil {
				return fmt.Errorf("write generate output: %w", err)
			}
			return writeGenerateHookResult(stdout, root, result)
		},
	}
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
	if _, err := fmt.Fprintf(stdout, "hook: %s (%s)\n", relToHooksRoot(root, result.HookFile), createdStatus(result.HookFileCreated)); err != nil {
		return fmt.Errorf("write generate output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "runner: %s (%s)\n", relToHooksRoot(root, result.RunnerFile), writtenStatus(result.RunnerFileWritten)); err != nil {
		return fmt.Errorf("write generate output: %w", err)
	}
	return nil
}

func writeGenerateHookDryRun(stdout io.Writer, root string, target shape.AppRef) error {
	hookPath := filepath.Join(root, "apps", target.App, "entities", target.Name, "hooks.go")
	runnerPath := filepath.Join(root, "cmd", "dygo", "main.go")
	if _, err := fmt.Fprintf(stdout, "hook: %s (would create)\n", relToHooksRoot(root, hookPath)); err != nil {
		return fmt.Errorf("write generate output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "runner: %s (would update)\n", relToHooksRoot(root, runnerPath)); err != nil {
		return fmt.Errorf("write generate output: %w", err)
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

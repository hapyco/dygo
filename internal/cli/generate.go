package cli

import (
	"fmt"
	"io"

	"github.com/hapyco/dygo/internal/hookgen"
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

	cmd.AddCommand(newGenerateHookCommand(stdout))

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
			if _, err := fmt.Fprintf(stdout, "hook: %s (%s)\n", relToHooksRoot(root, result.HookFile), createdStatus(result.HookFileCreated)); err != nil {
				return fmt.Errorf("write generate output: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "runner: %s (%s)\n", relToHooksRoot(root, result.RunnerFile), writtenStatus(result.RunnerFileWritten)); err != nil {
				return fmt.Errorf("write generate output: %w", err)
			}
			return nil
		},
	}
}

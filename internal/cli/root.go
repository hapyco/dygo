package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/dygo-dev/dygo/internal/config"
	"github.com/spf13/cobra"
)

const version = "dev"

// Run executes the dygo command-line interface.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd, err := NewRootCommand(ctx, stdin, stdout, stderr)
	if err != nil {
		return err
	}

	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		return fmt.Errorf("run cli: %w", err)
	}

	return nil
}

// NewRootCommand creates the root dygo CLI command.
func NewRootCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) (*cobra.Command, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	if stdin == nil {
		return nil, fmt.Errorf("stdin reader is required")
	}
	if stdout == nil {
		return nil, fmt.Errorf("stdout writer is required")
	}
	if stderr == nil {
		return nil, fmt.Errorf("stderr writer is required")
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("create root command: %w", err)
	}

	root := &cobra.Command{
		Use:           "dygo",
		Short:         "dygo is a metadata-driven business application platform.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newVersionCommand(stdout))
	root.AddCommand(newServeCommand(stdout))
	root.AddCommand(newAppsCommand(stdout))
	root.AddCommand(newEntitiesCommand(stdout))
	root.AddCommand(newSecretsCommand(ctx, stdin, stdout, stderr))

	return root, nil
}

func newVersionCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the dygo version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err := fmt.Fprintf(stdout, "dygo %s\n", version); err != nil {
				return fmt.Errorf("write version: %w", err)
			}
			return nil
		},
	}
}

func newServeCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the dygo server",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg := config.Default()
			if _, err := fmt.Fprintf(stdout, "dygo serve will listen on %s\n", cfg.Server.Address()); err != nil {
				return fmt.Errorf("write serve output: %w", err)
			}
			return nil
		},
	}
}

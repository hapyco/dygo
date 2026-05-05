package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/dygo-dev/dygo/internal/config"
	"github.com/dygo-dev/dygo/internal/db"
	"github.com/dygo-dev/dygo/internal/server"
	"github.com/spf13/cobra"
)

const version = "dev"

type serveRunner func(context.Context, string) error
type databaseChecker func(context.Context, string) error
type databaseRunner interface {
	Check(context.Context, string) error
	Create(context.Context, string) (db.DatabaseResult, error)
	Drop(context.Context, string) (db.DatabaseResult, error)
	Prepare(context.Context, string, string) (db.SchemaSyncResult, error)
	Reset(context.Context, string, string) (db.SchemaSyncResult, error)
	SchemaDump(context.Context, string, string) error
}
type schemaSyncRunner interface {
	Sync(context.Context, string, string) (db.SchemaSyncResult, error)
}

// Run executes the dygo command-line interface.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	migrator := db.NewMigrator()
	return runWithServices(ctx, args, stdin, stdout, stderr, server.Serve, db.NewManager(migrator), migrator)
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, serve serveRunner, checkDatabase databaseChecker) error {
	migrator := db.NewMigrator()
	return runWithServices(ctx, args, stdin, stdout, stderr, serve, checkBackedDatabaseRunner{check: checkDatabase, manager: db.NewManager(migrator)}, migrator)
}

func runWithServices(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, serve serveRunner, database databaseRunner, sync schemaSyncRunner) error {
	cmd, err := newRootCommand(ctx, stdin, stdout, stderr, serve, database, sync)
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
	migrator := db.NewMigrator()
	return newRootCommand(ctx, stdin, stdout, stderr, server.Serve, db.NewManager(migrator), migrator)
}

func newRootCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, serve serveRunner, database databaseRunner, sync schemaSyncRunner) (*cobra.Command, error) {
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
	if serve == nil {
		return nil, fmt.Errorf("serve runner is required")
	}
	if database == nil {
		return nil, fmt.Errorf("database runner is required")
	}
	if sync == nil {
		return nil, fmt.Errorf("schema sync runner is required")
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
	root.AddCommand(newDoctorCommand(ctx, stdout))
	root.AddCommand(newServeCommand(ctx, stdout, serve))
	root.AddCommand(newDBCommand(ctx, stdout, database))
	root.AddCommand(newMigrateCommand(ctx, stdout, sync))
	root.AddCommand(newAppsCommand(stdout))
	root.AddCommand(newEntitiesCommand(stdout))
	root.AddCommand(newSecretsCommand(ctx, stdin, stdout, stderr))

	return root, nil
}

type checkBackedDatabaseRunner struct {
	check   databaseChecker
	manager databaseRunner
}

func (r checkBackedDatabaseRunner) Check(ctx context.Context, databaseURL string) error {
	if r.check != nil {
		return r.check(ctx, databaseURL)
	}
	return r.manager.Check(ctx, databaseURL)
}

func (r checkBackedDatabaseRunner) Create(ctx context.Context, databaseURL string) (db.DatabaseResult, error) {
	return r.manager.Create(ctx, databaseURL)
}

func (r checkBackedDatabaseRunner) Drop(ctx context.Context, databaseURL string) (db.DatabaseResult, error) {
	return r.manager.Drop(ctx, databaseURL)
}

func (r checkBackedDatabaseRunner) Prepare(ctx context.Context, root string, databaseURL string) (db.SchemaSyncResult, error) {
	return r.manager.Prepare(ctx, root, databaseURL)
}

func (r checkBackedDatabaseRunner) Reset(ctx context.Context, root string, databaseURL string) (db.SchemaSyncResult, error) {
	return r.manager.Reset(ctx, root, databaseURL)
}

func (r checkBackedDatabaseRunner) SchemaDump(ctx context.Context, root string, databaseURL string) error {
	return r.manager.SchemaDump(ctx, root, databaseURL)
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

func newServeCommand(ctx context.Context, stdout io.Writer, serve serveRunner) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the dygo server",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(root)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			address := cfg.Server.Address()
			if _, err := fmt.Fprintf(stdout, "dygo serving on %s\n", address); err != nil {
				return fmt.Errorf("write serve output: %w", err)
			}
			if err := serve(ctx, address); err != nil {
				return fmt.Errorf("serve dygo: %w", err)
			}
			return nil
		},
	}
}

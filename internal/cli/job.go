package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hapyco/dygo/internal/db"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/internal/queues"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

type jobExecutionRunEnqueuer interface {
	Enqueue(context.Context, string, string, json.RawMessage, jobstore.EnqueueOptions) (jobstore.Execution, error)
}

var openJobExecutionRunEnqueuer = func(ctx context.Context, databaseURL string, queueConfig queues.Config) (jobExecutionRunEnqueuer, func(), error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, nil, err
	}
	store, err := jobstore.New(pool, queueConfig)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	return store, pool.Close, nil
}

func newJobCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Manage dygo Jobs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newJobExecutionCommand(ctx, stdout))
	return cmd
}

func newJobExecutionCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "execution",
		Aliases: []string{"exec"},
		Short:   "Manage Job Executions",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newJobExecutionRunCommand(ctx, stdout))
	return cmd
}

func newJobExecutionRunCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	payloadValue := "{}"
	idempotencyKey := ""

	cmd := &cobra.Command{
		Use:   "run <app>/<job>",
		Short: "Queue a Job Execution for testing",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			target, err := shape.ParseAppRef(args[0])
			if err != nil {
				return err
			}
			payload, err := parseJobExecutionRunPayload(payloadValue)
			if err != nil {
				return err
			}
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			queueConfig, err := queues.Load(root)
			if err != nil {
				return err
			}
			enqueuer, closeEnqueuer, err := openJobExecutionRunEnqueuer(ctx, databaseURL, queueConfig)
			if err != nil {
				return db.SanitizeDatabaseError("connect job database", databaseURL, err)
			}
			defer closeEnqueuer()
			execution, err := enqueuer.Enqueue(ctx, target.App, target.Name, payload, jobstore.EnqueueOptions{
				IdempotencyKey: idempotencyKey,
			})
			if err != nil {
				return jobExecutionRunCommandError(err)
			}
			if _, err := fmt.Fprintf(stdout, "job execution queued: %s/%s id=%d name=%s queue=%s status=%s (%s)\n", execution.AppName, execution.JobName, execution.ID, execution.Name, execution.Queue, execution.Status, env); err != nil {
				return fmt.Errorf("write job execution run output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().StringVar(&payloadValue, "payload", payloadValue, "JSON payload to enqueue")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", idempotencyKey, "Stable duplicate-prevention key for this Job")

	return cmd
}

func parseJobExecutionRunPayload(value string) (json.RawMessage, error) {
	payload := strings.TrimSpace(value)
	if payload == "" {
		return nil, fmt.Errorf("job payload must be valid JSON")
	}
	if !json.Valid([]byte(payload)) {
		return nil, fmt.Errorf("job payload must be valid JSON")
	}
	return json.RawMessage(payload), nil
}

func jobExecutionRunCommandError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return fmt.Errorf("job schema is not ready; run dygo db migrate: %w", err)
	}
	if strings.Contains(err.Error(), "is not registered") {
		return fmt.Errorf("%w; run dygo db migrate", err)
	}
	return err
}

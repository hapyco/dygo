package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hapyco/dygo/internal/db"
	jobruntime "github.com/hapyco/dygo/internal/jobs/runtime"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/internal/queues"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

type workerPool interface {
	jobstore.Beginner
	jobstore.ListenerPool
	db.RecordQueryer
	Close()
}

var openWorkerPool = func(ctx context.Context, databaseURL string) (workerPool, error) {
	return pgxpool.New(ctx, databaseURL)
}

func newWorkerCommand(ctx context.Context, stdout, stderr io.Writer, recordHooks *db.RecordHookRegistry, jobRegistry *jobruntime.Registry) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	queueNames := []string{}
	concurrency := 0
	pollInterval := jobruntime.DefaultPollInterval
	shutdownTimeout := 30 * time.Second
	once := false
	pollOnly := false

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Run dygo Job workers",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			queueConfig, err := queues.Load(root)
			if err != nil {
				return err
			}
			concurrencyChanged := cmd.Flags().Changed("concurrency")
			selectedQueues, err := effectiveWorkerQueues(queueConfig, queueNames, concurrency, concurrencyChanged)
			if err != nil {
				return err
			}
			pool, err := openWorkerPool(ctx, databaseURL)
			if err != nil {
				return db.SanitizeDatabaseError("connect job worker database", databaseURL, err)
			}
			defer pool.Close()
			store, err := jobstore.New(pool, queueConfig)
			if err != nil {
				return err
			}
			workerID, err := jobruntime.NewWorkerID()
			if err != nil {
				return fmt.Errorf("create worker id: %w", err)
			}
			var listener jobruntime.NotificationListener
			if !once && !pollOnly {
				listener, err = jobstore.NewListener(ctx, pool)
				if err != nil {
					if _, writeErr := fmt.Fprintf(stderr, "dygo worker: job notifications unavailable; polling every %s: %v\n", pollInterval, err); writeErr != nil {
						return fmt.Errorf("write worker output: %w", writeErr)
					}
				}
			}
			if _, err := fmt.Fprintf(stdout, "dygo worker: queues %s (%s)\n", workerQueueList(selectedQueues), env); err != nil {
				return fmt.Errorf("write worker output: %w", err)
			}
			result, err := (jobruntime.Worker{
				Store:       store,
				Registry:    jobRegistry,
				Queryer:     pool,
				RecordHooks: recordHooks,
				Stderr:      stderr,
			}).Run(ctx, jobruntime.Options{
				Queues:               selectedQueues,
				WorkerID:             workerID,
				PollInterval:         pollInterval,
				ShutdownTimeout:      shutdownTimeout,
				Once:                 once,
				PollOnly:             pollOnly,
				NotificationListener: listener,
			})
			if err != nil {
				return workerCommandError(err)
			}
			if once {
				if _, err := fmt.Fprintf(stdout, "dygo worker: processed %d executions\n", result.Claimed); err != nil {
					return fmt.Errorf("write worker output: %w", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().StringArrayVar(&queueNames, "queue", nil, "Queue to process; may be repeated")
	cmd.Flags().IntVar(&concurrency, "concurrency", concurrency, "Override concurrency for each selected queue")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", pollInterval, "How often to poll for queued executions")
	cmd.Flags().BoolVar(&pollOnly, "poll-only", pollOnly, "Poll for queued executions without PostgreSQL notifications")
	cmd.Flags().BoolVar(&once, "once", once, "Run one available batch and exit")
	cmd.Flags().DurationVar(&shutdownTimeout, "shutdown-timeout", shutdownTimeout, "How long to wait for running executions during shutdown")

	return cmd
}

func effectiveWorkerQueues(queueConfig queues.Config, requested []string, override int, overrideSet bool) ([]jobruntime.Queue, error) {
	if overrideSet && override < 1 {
		return nil, fmt.Errorf("--concurrency must be greater than 0")
	}
	if len(requested) == 0 {
		selected := make([]jobruntime.Queue, 0, len(queueConfig.Queues))
		for _, queue := range queueConfig.Queues {
			selected = append(selected, jobruntime.Queue{Name: queue.Name, Concurrency: effectiveConcurrency(queue.Concurrency, override, overrideSet)})
		}
		return selected, nil
	}
	seen := map[string]struct{}{}
	selected := make([]jobruntime.Queue, 0, len(requested))
	for _, name := range requested {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("--queue value is required")
		}
		if _, ok := seen[name]; ok {
			continue
		}
		queue, ok := queueByName(queueConfig, name)
		if !ok {
			return nil, fmt.Errorf("queue %q is not registered", name)
		}
		seen[name] = struct{}{}
		selected = append(selected, jobruntime.Queue{Name: queue.Name, Concurrency: effectiveConcurrency(queue.Concurrency, override, overrideSet)})
	}
	return selected, nil
}

func effectiveConcurrency(configured int, override int, overrideSet bool) int {
	if overrideSet {
		return override
	}
	return configured
}

func queueByName(queueConfig queues.Config, name string) (queues.Queue, bool) {
	for _, queue := range queueConfig.Queues {
		if queue.Name == name {
			return queue, true
		}
	}
	return queues.Queue{}, false
}

func workerQueueList(selected []jobruntime.Queue) string {
	parts := make([]string, len(selected))
	for index, queue := range selected {
		parts[index] = fmt.Sprintf("%s:%d", queue.Name, queue.Concurrency)
	}
	return strings.Join(parts, ",")
}

func workerCommandError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return fmt.Errorf("job worker schema is not ready; run dygo db migrate: %w", err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("job worker schema is not ready; run dygo db migrate: %w", err)
	}
	return err
}

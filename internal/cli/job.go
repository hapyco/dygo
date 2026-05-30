package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/jobs"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	"github.com/hapyco/dygo/internal/queues"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
)

type jobExecutionStore interface {
	ListJobs(context.Context) ([]jobstore.Job, error)
	GetJob(context.Context, string, string) (jobstore.Job, error)
	DisableJob(context.Context, string, string) (jobstore.Job, error)
	EnableJob(context.Context, string, string) (jobstore.Job, error)
	Enqueue(context.Context, string, string, json.RawMessage, jobstore.EnqueueOptions) (jobstore.Execution, error)
	List(context.Context, jobstore.ListOptions) ([]jobstore.Execution, error)
	Get(context.Context, string) (jobstore.Execution, error)
	CancelQueued(context.Context, string, time.Time) (jobstore.Execution, error)
	Retry(context.Context, string, string) (jobstore.Execution, error)
}

var openJobExecutionStore = func(ctx context.Context, databaseURL string, queueConfig ...queues.Config) (jobExecutionStore, func(), error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, nil, err
	}
	store, err := jobstore.New(pool, queueConfig...)
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

	cmd.AddCommand(newJobListCommand(ctx, stdout))
	cmd.AddCommand(newJobShowCommand(ctx, stdout))
	cmd.AddCommand(newJobDisableCommand(ctx, stdout))
	cmd.AddCommand(newJobEnableCommand(ctx, stdout))
	cmd.AddCommand(newJobExecutionCommand(ctx, stdout))
	return cmd
}

func newJobListCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered Jobs",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, store, closeStore, err := jobExecutionStoreForCommand(ctx, envName, false)
			if err != nil {
				return err
			}
			defer closeStore()
			jobs, err := store.ListJobs(ctx)
			if err != nil {
				return jobExecutionCommandError(err)
			}
			if len(jobs) == 0 {
				if _, err := fmt.Fprintf(stdout, "No jobs found (%s).\n", env); err != nil {
					return fmt.Errorf("write job list output: %w", err)
				}
				return nil
			}
			return writeJobList(stdout, jobs)
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newJobShowCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "show <app>/<job>",
		Short: "Show one registered Job",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			target, err := shape.ParseAppRef(args[0])
			if err != nil {
				return err
			}
			_, store, closeStore, err := jobExecutionStoreForCommand(ctx, envName, false)
			if err != nil {
				return err
			}
			defer closeStore()
			job, err := store.GetJob(ctx, target.App, target.Name)
			if err != nil {
				return jobExecutionCommandError(err)
			}
			return writeJobShow(stdout, job)
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newJobDisableCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "disable <app>/<job>",
		Short: "Disable a registered Job",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			target, err := shape.ParseAppRef(args[0])
			if err != nil {
				return err
			}
			env, store, closeStore, err := jobExecutionStoreForCommand(ctx, envName, false)
			if err != nil {
				return err
			}
			defer closeStore()
			job, err := store.DisableJob(ctx, target.App, target.Name)
			if err != nil {
				return jobExecutionCommandError(err)
			}
			if _, err := fmt.Fprintf(stdout, "job disabled: %s/%s status=%s (%s)\n", job.AppName, job.Key, formatJobStatus(job), env); err != nil {
				return fmt.Errorf("write job disable output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newJobEnableCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "enable <app>/<job>",
		Short: "Enable a registered Job",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			target, err := shape.ParseAppRef(args[0])
			if err != nil {
				return err
			}
			env, store, closeStore, err := jobExecutionStoreForCommand(ctx, envName, false)
			if err != nil {
				return err
			}
			defer closeStore()
			job, err := store.EnableJob(ctx, target.App, target.Name)
			if err != nil {
				return jobExecutionCommandError(err)
			}
			if _, err := fmt.Fprintf(stdout, "job enabled: %s/%s status=%s (%s)\n", job.AppName, job.Key, formatJobStatus(job), env); err != nil {
				return fmt.Errorf("write job enable output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

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
	cmd.AddCommand(newJobExecutionListCommand(ctx, stdout))
	cmd.AddCommand(newJobExecutionShowCommand(ctx, stdout))
	cmd.AddCommand(newJobExecutionCancelCommand(ctx, stdout))
	cmd.AddCommand(newJobExecutionRetryCommand(ctx, stdout))
	return cmd
}

func newJobExecutionListCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	limit := 20

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent Job Executions",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if limit < 1 {
				return fmt.Errorf("--limit must be greater than 0")
			}
			env, store, closeStore, err := jobExecutionStoreForCommand(ctx, envName, false)
			if err != nil {
				return err
			}
			defer closeStore()
			executions, err := store.List(ctx, jobstore.ListOptions{Limit: limit})
			if err != nil {
				return jobExecutionCommandError(err)
			}
			if len(executions) == 0 {
				if _, err := fmt.Fprintf(stdout, "No job executions found (%s).\n", env); err != nil {
					return fmt.Errorf("write job execution list output: %w", err)
				}
				return nil
			}
			return writeJobExecutionList(stdout, executions)
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().IntVar(&limit, "limit", limit, "Maximum number of Job Executions to show")

	return cmd
}

func newJobExecutionShowCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "show <id-or-name>",
		Short: "Show one Job Execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			_, store, closeStore, err := jobExecutionStoreForCommand(ctx, envName, false)
			if err != nil {
				return err
			}
			defer closeStore()
			execution, err := store.Get(ctx, args[0])
			if err != nil {
				return jobExecutionCommandError(err)
			}
			return writeJobExecutionShow(stdout, execution)
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newJobExecutionCancelCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

	cmd := &cobra.Command{
		Use:   "cancel <id-or-name>",
		Short: "Cancel a queued Job Execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			env, store, closeStore, err := jobExecutionStoreForCommand(ctx, envName, false)
			if err != nil {
				return err
			}
			defer closeStore()
			execution, err := store.CancelQueued(ctx, args[0], time.Now().UTC())
			if err != nil {
				return jobExecutionCommandError(err)
			}
			if _, err := fmt.Fprintf(stdout, "job execution cancelled: id=%d name=%s status=%s (%s)\n", execution.ID, execution.Name, execution.Status, env); err != nil {
				return fmt.Errorf("write job execution cancel output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newJobExecutionRetryCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	idempotencyKey := ""

	cmd := &cobra.Command{
		Use:   "retry <id-or-name>",
		Short: "Retry a failed Job Execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if strings.TrimSpace(idempotencyKey) == "" {
				return fmt.Errorf("--idempotency-key is required")
			}
			env, store, closeStore, err := jobExecutionStoreForCommand(ctx, envName, true)
			if err != nil {
				return err
			}
			defer closeStore()
			execution, err := store.Retry(ctx, args[0], idempotencyKey)
			if err != nil {
				return jobExecutionCommandError(err)
			}
			return writeJobExecutionQueued(stdout, execution, env)
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", idempotencyKey, "Stable duplicate-prevention key for the retried Job Execution")

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
			env, store, closeStore, err := jobExecutionStoreForCommand(ctx, envName, true)
			if err != nil {
				return err
			}
			defer closeStore()
			execution, err := store.Enqueue(ctx, target.App, target.Name, payload, jobstore.EnqueueOptions{
				IdempotencyKey: idempotencyKey,
			})
			if err != nil {
				return jobExecutionCommandError(err)
			}
			return writeJobExecutionQueued(stdout, execution, env)
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().StringVar(&payloadValue, "payload", payloadValue, "JSON payload to enqueue")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", idempotencyKey, "Stable duplicate-prevention key for this Job")

	return cmd
}

func jobExecutionStoreForCommand(ctx context.Context, envName string, includeQueues bool) (string, jobExecutionStore, func(), error) {
	env, root, databaseURL, err := databaseInputs(envName)
	if err != nil {
		return "", nil, nil, err
	}
	if includeQueues {
		queueConfig, err := queues.Load(root)
		if err != nil {
			return "", nil, nil, err
		}
		store, closeStore, err := openJobExecutionStore(ctx, databaseURL, queueConfig)
		if err != nil {
			return "", nil, nil, db.SanitizeDatabaseError("connect job database", databaseURL, err)
		}
		return string(env), store, closeStore, nil
	}
	store, closeStore, err := openJobExecutionStore(ctx, databaseURL)
	if err != nil {
		return "", nil, nil, db.SanitizeDatabaseError("connect job database", databaseURL, err)
	}
	return string(env), store, closeStore, nil
}

func writeJobExecutionQueued(stdout io.Writer, execution jobstore.Execution, env string) error {
	if _, err := fmt.Fprintf(stdout, "job execution queued: %s/%s id=%d name=%s queue=%s status=%s (%s)\n", execution.AppName, execution.JobName, execution.ID, execution.Name, execution.Queue, execution.Status, env); err != nil {
		return fmt.Errorf("write job execution output: %w", err)
	}
	return nil
}

func writeJobList(stdout io.Writer, jobs []jobstore.Job) error {
	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "APP\tJOB\tSTATUS\tSOURCE\tQUEUE\tTIMEOUT\tLABEL"); err != nil {
		return fmt.Errorf("write job list header: %w", err)
	}
	for _, job := range jobs {
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			job.AppName,
			job.Key,
			formatJobStatus(job),
			emptyDash(job.Source),
			emptyDash(job.Queue),
			emptyDash(job.Timeout),
			emptyDash(job.Label),
		); err != nil {
			return fmt.Errorf("write job list row: %w", err)
		}
	}
	if err := table.Flush(); err != nil {
		return fmt.Errorf("flush job list output: %w", err)
	}
	return nil
}

func writeJobShow(stdout io.Writer, job jobstore.Job) error {
	lines := []struct {
		label string
		value string
	}{
		{"id", strconv.FormatInt(job.ID, 10)},
		{"name", emptyDash(job.Name)},
		{"app", emptyDash(job.AppName)},
		{"job", emptyDash(job.Key)},
		{"status", formatJobStatus(job)},
		{"source", emptyDash(job.Source)},
		{"enabled", strconv.FormatBool(job.Enabled)},
		{"retired", strconv.FormatBool(job.Retired)},
		{"label", emptyDash(job.Label)},
		{"description", emptyDash(job.Description)},
		{"queue", emptyDash(job.Queue)},
		{"timeout", emptyDash(job.Timeout)},
		{"retry", formatJobExecutionRetry(job.Retry)},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(stdout, "%s: %s\n", line.label, line.value); err != nil {
			return fmt.Errorf("write job show output: %w", err)
		}
	}
	return nil
}

func writeJobExecutionList(stdout io.Writer, executions []jobstore.Execution) error {
	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "ID\tNAME\tJOB\tSTATUS\tQUEUE\tATTEMPTS\tRUN_AFTER\tSTARTED_AT\tFINISHED_AT"); err != nil {
		return fmt.Errorf("write job execution list header: %w", err)
	}
	for _, execution := range executions {
		if _, err := fmt.Fprintf(table, "%d\t%s\t%s/%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			execution.ID,
			execution.Name,
			execution.AppName,
			execution.JobName,
			execution.Status,
			execution.Queue,
			formatJobExecutionAttempts(execution),
			formatJobExecutionTime(execution.RunAfter),
			formatOptionalJobExecutionTime(execution.StartedAt),
			formatOptionalJobExecutionTime(execution.FinishedAt),
		); err != nil {
			return fmt.Errorf("write job execution list row: %w", err)
		}
	}
	if err := table.Flush(); err != nil {
		return fmt.Errorf("flush job execution list output: %w", err)
	}
	return nil
}

func writeJobExecutionShow(stdout io.Writer, execution jobstore.Execution) error {
	lines := []struct {
		label string
		value string
	}{
		{"id", strconv.FormatInt(execution.ID, 10)},
		{"name", emptyDash(execution.Name)},
		{"job", execution.AppName + "/" + execution.JobName},
		{"status", emptyDash(execution.Status)},
		{"queue", emptyDash(execution.Queue)},
		{"priority", strconv.Itoa(execution.Priority)},
		{"attempts", formatJobExecutionAttempts(execution)},
		{"run-after", formatJobExecutionTime(execution.RunAfter)},
		{"started-at", formatOptionalJobExecutionTime(execution.StartedAt)},
		{"finished-at", formatOptionalJobExecutionTime(execution.FinishedAt)},
		{"locked-by", emptyDash(execution.LockedBy)},
		{"locked-until", formatOptionalJobExecutionTime(execution.LockedUntil)},
		{"idempotency-key", emptyDash(execution.IdempotencyKey)},
		{"timeout", formatJobExecutionTimeout(execution.Timeout)},
		{"retry", formatJobExecutionRetry(execution.Retry)},
		{"payload", formatJobExecutionJSON(execution.Payload)},
		{"result", formatJobExecutionJSON(execution.Result)},
		{"error", emptyDash(execution.Error)},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(stdout, "%s: %s\n", line.label, line.value); err != nil {
			return fmt.Errorf("write job execution show output: %w", err)
		}
	}
	return nil
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

func jobExecutionCommandError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return fmt.Errorf("job schema is not ready; run dygo db migrate: %w", err)
	}
	if strings.Contains(err.Error(), "is not registered") {
		return fmt.Errorf("%w; run dygo db migrate", err)
	}
	return err
}

func formatJobStatus(job jobstore.Job) string {
	if job.Retired {
		return "retired"
	}
	if !job.Enabled {
		return "disabled"
	}
	return "active"
}

func formatJobExecutionAttempts(execution jobstore.Execution) string {
	return fmt.Sprintf("%d/%d", execution.Attempts, execution.MaxAttempts)
}

func formatJobExecutionTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}

func formatOptionalJobExecutionTime(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return formatJobExecutionTime(*value)
}

func formatJobExecutionTimeout(value time.Duration) string {
	if value <= 0 {
		return "-"
	}
	return value.String()
}

func formatJobExecutionRetry(retry *jobs.Retry) string {
	if retry == nil {
		return "-"
	}
	return fmt.Sprintf("attempts=%d initial-delay=%s max-delay=%s", retry.Attempts, retry.InitialDelay, retry.MaxDelay)
}

func formatJobExecutionJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "-"
	}
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, raw); err != nil {
		return string(raw)
	}
	return buffer.String()
}

func emptyDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

// Package runtime runs compiled Job handlers against durable Job Executions.
package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hapyco/dygo/internal/db"
	jobstore "github.com/hapyco/dygo/internal/jobs/store"
	namegen "github.com/hapyco/dygo/internal/naming"
	"github.com/hapyco/dygo/internal/sdkdata"
	"github.com/hapyco/dygo/pkg/sdk"
)

const (
	// DefaultPollInterval is the worker fallback interval when notifications are unavailable.
	DefaultPollInterval    = 60 * time.Second
	defaultShutdownTimeout = 30 * time.Second
)

// Registry stores compiled Job handlers.
type Registry struct {
	handlers map[string]sdk.JobFunc
}

// Queue is one worker queue with effective concurrency.
type Queue struct {
	Name        string
	Concurrency int
}

// Worker runs Job Executions.
type Worker struct {
	Store       Store
	Registry    *Registry
	Queryer     db.RecordQueryer
	RecordHooks *db.RecordHookRegistry
	Stderr      io.Writer
}

// Store is the durable queue behavior the worker needs.
type Store interface {
	RecoverExpired(context.Context, time.Time) (int, error)
	Claim(context.Context, []string, int, string, time.Time) ([]jobstore.Execution, error)
	NextRunAfter(context.Context, []string, time.Time) (*time.Time, error)
	Complete(context.Context, int64, json.RawMessage, time.Time) error
	Fail(context.Context, int64, string, time.Time) error
	FailFinal(context.Context, int64, string, time.Time) error
	Enqueue(context.Context, string, string, json.RawMessage, jobstore.EnqueueOptions) (jobstore.Execution, error)
}

// NotificationListener waits for PostgreSQL queue notifications.
type NotificationListener interface {
	Wait(context.Context) (string, error)
	Close()
}

// Options configures one worker process.
type Options struct {
	Queues               []Queue
	WorkerID             string
	PollInterval         time.Duration
	ShutdownTimeout      time.Duration
	Once                 bool
	PollOnly             bool
	NotificationListener NotificationListener
	Now                  func() time.Time
}

// Result summarizes worker activity.
type Result struct {
	Recovered int
	Claimed   int
	Succeeded int
	Failed    int
}

// NewRegistry returns a Registry populated by compiled app registrars.
func NewRegistry(registrars []sdk.JobRegistrar) (*Registry, error) {
	registry := &Registry{handlers: map[string]sdk.JobFunc{}}
	adapter := registryAdapter{registry: registry}
	for index, registrar := range registrars {
		if registrar == nil {
			return nil, fmt.Errorf("job registrar %d is required", index+1)
		}
		if err := registrar(adapter); err != nil {
			return nil, fmt.Errorf("register job registrar %d: %w", index+1, err)
		}
	}
	return registry, nil
}

// RegisterJob stores one compiled Job handler.
func (r *Registry) RegisterJob(appName string, jobName string, fn sdk.JobFunc) error {
	if fn == nil {
		return fmt.Errorf("job %s/%s function is required", appName, jobName)
	}
	key, err := handlerKey(appName, jobName)
	if err != nil {
		return err
	}
	if r.handlers == nil {
		r.handlers = map[string]sdk.JobFunc{}
	}
	if _, ok := r.handlers[key]; ok {
		return fmt.Errorf("job %s/%s is already registered", appName, jobName)
	}
	r.handlers[key] = fn
	return nil
}

func (r *Registry) handler(appName string, jobName string) (sdk.JobFunc, bool) {
	if r == nil {
		return nil, false
	}
	fn, ok := r.handlers[appName+"\x00"+jobName]
	return fn, ok
}

type registryAdapter struct {
	registry *Registry
}

func (r registryAdapter) RegisterJob(appName string, jobName string, fn sdk.JobFunc) error {
	return r.registry.RegisterJob(appName, jobName, fn)
}

// NewWorkerID returns a readable process-scoped worker identifier.
func NewWorkerID() (string, error) {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "localhost"
	}
	suffix, err := randomSuffix()
	if err != nil {
		return "", err
	}
	return host + ":" + strconv.Itoa(os.Getpid()) + ":" + suffix, nil
}

// Run starts the worker.
func (w Worker) Run(ctx context.Context, options Options) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("context is required")
	}
	if len(options.Queues) == 0 {
		return Result{}, fmt.Errorf("worker requires at least one queue")
	}
	for _, queue := range options.Queues {
		if strings.TrimSpace(queue.Name) == "" {
			return Result{}, fmt.Errorf("worker queue name is required")
		}
		if queue.Concurrency < 1 {
			return Result{}, fmt.Errorf("worker queue %q concurrency must be greater than 0", queue.Name)
		}
	}
	options = normalizeOptions(options)
	if options.Once {
		return w.runOnce(ctx, options)
	}
	return w.runContinuous(ctx, options)
}

func (w Worker) runOnce(ctx context.Context, options Options) (Result, error) {
	var total Result
	for _, queue := range options.Queues {
		result, err := w.runQueueBatch(ctx, queue, options)
		total.add(result)
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func (w Worker) runContinuous(ctx context.Context, options Options) (Result, error) {
	var total safeResult
	errs := make(chan error, len(options.Queues))
	var wg sync.WaitGroup
	var listenerWG sync.WaitGroup
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	wakeups := newQueueWakeups(options.Queues)
	defer wakeups.close()
	if options.NotificationListener != nil && !options.PollOnly {
		listenerWG.Add(1)
		go func() {
			defer listenerWG.Done()
			w.runNotificationFanout(runCtx, options.NotificationListener, wakeups, options)
		}()
	}
	for _, queue := range options.Queues {
		queue := queue
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- w.runQueueLoop(runCtx, queue, options, &total, wakeups.channel(queue.Name))
		}()
	}

	select {
	case err := <-errs:
		cancel()
		wg.Wait()
		waitForGroup(&listenerWG, options.ShutdownTimeout)
		if err != nil && !errorsIsContext(err) {
			return total.snapshot(), err
		}
		return total.snapshot(), nil
	case <-ctx.Done():
		cancel()
		waitForGroup(&wg, options.ShutdownTimeout)
		waitForGroup(&listenerWG, options.ShutdownTimeout)
		return total.snapshot(), nil
	}
}

func (w Worker) runQueueLoop(ctx context.Context, queue Queue, options Options, total *safeResult, wakeup <-chan struct{}) error {
	slots := make(chan struct{}, queue.Concurrency)
	var inFlight sync.WaitGroup
	errs := make(chan error, 1)
	slotReleased := make(chan struct{}, 1)
	defer waitForGroup(&inFlight, options.ShutdownTimeout)

	for {
		available := queue.Concurrency - len(slots)
		if available > 0 {
			now := options.Now()
			recovered, err := w.Store.RecoverExpired(ctx, now)
			if err != nil {
				return err
			}
			executions, err := w.Store.Claim(ctx, []string{queue.Name}, available, options.WorkerID, now)
			if err != nil {
				return err
			}
			total.add(Result{Recovered: recovered, Claimed: len(executions)})
			for _, execution := range executions {
				execution := execution
				slots <- struct{}{}
				inFlight.Add(1)
				go func() {
					defer inFlight.Done()
					defer func() {
						<-slots
						signal(slotReleased)
					}()
					result, err := w.runExecution(ctx, execution, options)
					total.add(result)
					if err != nil {
						select {
						case errs <- err:
						default:
						}
					}
				}()
			}
		}
		waitFor := options.PollInterval
		if available > 0 {
			now := options.Now()
			next, err := w.Store.NextRunAfter(ctx, []string{queue.Name}, now)
			if err != nil {
				return err
			}
			waitFor = soonerDuration(waitFor, next, now)
		}
		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			stopTimer(timer)
			return ctx.Err()
		case err := <-errs:
			stopTimer(timer)
			return err
		case <-slotReleased:
			stopTimer(timer)
		case _, ok := <-wakeup:
			stopTimer(timer)
			if !ok {
				wakeup = nil
			}
		case <-timer.C:
		}
	}
}

func (w Worker) runNotificationFanout(ctx context.Context, listener NotificationListener, wakeups *queueWakeups, options Options) {
	defer listener.Close()
	defer wakeups.close()
	for {
		queue, err := listener.Wait(ctx)
		if err != nil {
			if !errorsIsContext(err) {
				w.log("job notifications unavailable; polling every %s: %v", options.PollInterval, err)
			}
			return
		}
		queue = strings.TrimSpace(queue)
		if queue == "" {
			wakeups.wakeAll()
			continue
		}
		wakeups.wake(queue)
	}
}

func (w Worker) runQueueBatch(ctx context.Context, queue Queue, options Options) (Result, error) {
	now := options.Now()
	recovered, err := w.Store.RecoverExpired(ctx, now)
	if err != nil {
		return Result{}, err
	}
	executions, err := w.Store.Claim(ctx, []string{queue.Name}, queue.Concurrency, options.WorkerID, now)
	if err != nil {
		return Result{Recovered: recovered}, err
	}
	result := Result{Recovered: recovered, Claimed: len(executions)}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, execution := range executions {
		execution := execution
		wg.Add(1)
		go func() {
			defer wg.Done()
			runResult, err := w.runExecution(ctx, execution, options)
			mu.Lock()
			result.add(runResult)
			if err != nil && firstErr == nil {
				firstErr = err
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return result, firstErr
}

func (w Worker) runExecution(ctx context.Context, execution jobstore.Execution, options Options) (Result, error) {
	fn, ok := w.Registry.handler(execution.AppName, execution.JobName)
	if !ok {
		message := fmt.Sprintf("missing job handler for %s/%s", execution.AppName, execution.JobName)
		w.log("%s execution %d", message, execution.ID)
		if err := w.Store.FailFinal(context.WithoutCancel(ctx), execution.ID, message, options.Now()); err != nil {
			return Result{Failed: 1}, err
		}
		return Result{Failed: 1}, nil
	}

	w.log("running %s/%s execution %d attempt %d", execution.AppName, execution.JobName, execution.ID, execution.Attempts)
	executionCtx := context.WithoutCancel(ctx)
	runCtx, cancel := context.WithTimeout(executionCtx, execution.Timeout)
	defer cancel()
	err := fn(runCtx, sdk.JobExecution{
		ID:      execution.ID,
		AppName: execution.AppName,
		JobName: execution.JobName,
		Queue:   execution.Queue,
		Attempt: execution.Attempts,
		Payload: execution.Payload,
		Records: sdkdata.NewRecordData(w.Queryer, w.RecordHooks),
		Jobs:    sdkdata.NewJobData(w.Store),
	})
	if err == nil {
		if err := w.Store.Complete(executionCtx, execution.ID, nil, options.Now()); err != nil {
			return Result{}, err
		}
		w.log("succeeded %s/%s execution %d", execution.AppName, execution.JobName, execution.ID)
		return Result{Succeeded: 1}, nil
	}
	message := strings.TrimSpace(err.Error())
	if runCtx.Err() == context.DeadlineExceeded {
		message = "timeout"
	}
	if failErr := w.Store.Fail(executionCtx, execution.ID, message, options.Now()); failErr != nil {
		return Result{Failed: 1}, failErr
	}
	w.log("failed %s/%s execution %d: %s", execution.AppName, execution.JobName, execution.ID, message)
	return Result{Failed: 1}, nil
}

func (w Worker) log(format string, args ...any) {
	if w.Stderr == nil {
		return
	}
	_, _ = fmt.Fprintf(w.Stderr, "dygo worker: "+format+"\n", args...)
}

func normalizeOptions(options Options) Options {
	if strings.TrimSpace(options.WorkerID) == "" {
		if workerID, err := NewWorkerID(); err == nil {
			options.WorkerID = workerID
		} else {
			options.WorkerID = "localhost:" + strconv.Itoa(os.Getpid())
		}
	}
	if options.PollInterval <= 0 {
		options.PollInterval = DefaultPollInterval
	}
	if options.ShutdownTimeout <= 0 {
		options.ShutdownTimeout = defaultShutdownTimeout
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	return options
}

func handlerKey(appName string, jobName string) (string, error) {
	appName = strings.TrimSpace(appName)
	jobName = strings.TrimSpace(jobName)
	if appName == "" || jobName == "" {
		return "", fmt.Errorf("job app and name are required")
	}
	return appName + "\x00" + jobName, nil
}

func randomSuffix() (string, error) {
	return namegen.Random(6)
}

type safeResult struct {
	mu     sync.Mutex
	result Result
}

func (r *safeResult) add(result Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.result.add(result)
}

func (r *safeResult) snapshot() Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.result
}

func (r *Result) add(other Result) {
	r.Recovered += other.Recovered
	r.Claimed += other.Claimed
	r.Succeeded += other.Succeeded
	r.Failed += other.Failed
}

func waitForGroup(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}

func errorsIsContext(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func soonerDuration(fallback time.Duration, next *time.Time, now time.Time) time.Duration {
	if next == nil {
		return fallback
	}
	waitFor := next.Sub(now)
	if waitFor < 0 {
		waitFor = 0
	}
	if waitFor < fallback {
		return waitFor
	}
	return fallback
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func signal(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

type queueWakeups struct {
	mu       sync.RWMutex
	channels map[string]chan struct{}
	closed   bool
}

func newQueueWakeups(queues []Queue) *queueWakeups {
	wakeups := &queueWakeups{channels: map[string]chan struct{}{}}
	for _, queue := range queues {
		if _, ok := wakeups.channels[queue.Name]; !ok {
			wakeups.channels[queue.Name] = make(chan struct{}, 1)
		}
	}
	return wakeups
}

func (w *queueWakeups) channel(queue string) <-chan struct{} {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.channels[queue]
}

func (w *queueWakeups) wake(queue string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		return
	}
	ch := w.channels[queue]
	if ch != nil {
		signal(ch)
	}
}

func (w *queueWakeups) wakeAll() {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		return
	}
	for _, ch := range w.channels {
		signal(ch)
	}
}

func (w *queueWakeups) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	for _, ch := range w.channels {
		close(ch)
	}
	w.closed = true
}

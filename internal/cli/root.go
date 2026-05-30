package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hapyco/dygo/internal/auth"
	"github.com/hapyco/dygo/internal/config"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/fixtures"
	recordhooks "github.com/hapyco/dygo/internal/hooks"
	jobruntime "github.com/hapyco/dygo/internal/jobs/runtime"
	"github.com/hapyco/dygo/internal/server"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/studio"
	"github.com/hapyco/dygo/pkg/sdk"
	"github.com/spf13/cobra"
)

const defaultStudioDevURL = "http://127.0.0.1:6791"

type serveRunner func(context.Context, server.Options) error
type studioDevStop func() error
type databaseChecker func(context.Context, string) error
type adminSetupRunner interface {
	SetupAdmin(context.Context, string, auth.SetupAdminInput) (auth.User, error)
}
type fixtureRunner interface {
	Plan(context.Context, string) (fixtures.Plan, error)
	Apply(context.Context, string, string) (fixtures.Result, error)
	ExportPlan(context.Context, string, string, shape.AppRef, bool) (fixtures.ExportPlan, error)
	WriteExportPlan(context.Context, fixtures.ExportPlan) (fixtures.ExportResult, error)
}
type databaseRunner interface {
	Check(context.Context, string) error
	Exists(context.Context, string) (db.DatabaseStatus, error)
	Create(context.Context, string) (db.DatabaseResult, error)
	Drop(context.Context, string) (db.DatabaseResult, error)
}
type schemaSyncRunner interface {
	ApplyPatches(context.Context, string, string, string, string) (db.PatchApplyResult, error)
	PatchPlan(context.Context, string, string, string) (db.PatchPlan, error)
	Plan(context.Context, string, string) (db.SchemaPlan, error)
	Prune(context.Context, string, string) (db.SchemaPruneResult, error)
	PrunePlan(context.Context, string, string) (db.SchemaPrunePlan, error)
	Sync(context.Context, string, string) (db.SchemaSyncResult, error)
}

// Options configures dygo CLI runtime extensions.
type Options struct {
	RecordHooks []sdk.RecordHookRegistrar
	Jobs        []sdk.JobRegistrar
}

var startStudioDevServer = startStudioDevServerProcess

// Run executes the dygo command-line interface.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return RunWithOptions(ctx, args, stdin, stdout, stderr, Options{})
}

// RunWithOptions executes the dygo command-line interface with compiled extensions.
func RunWithOptions(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, options Options) error {
	migrator := db.NewMigrator()
	recordHooks, err := recordhooks.NewRecordHookRegistry(options.RecordHooks)
	if err != nil {
		return fmt.Errorf("configure record hooks: %w", err)
	}
	jobRegistry, err := jobruntime.NewRegistry(options.Jobs)
	if err != nil {
		return fmt.Errorf("configure jobs: %w", err)
	}
	return runWithServicesAndSetupAndFixturesAndHooks(ctx, args, stdin, stdout, stderr, server.Serve, db.NewManager(migrator), migrator, defaultAdminSetupRunner{}, defaultFixtureRunner{recordHooks: recordHooks}, defaultPermissionRunner{}, recordHooks, jobRegistry)
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, serve serveRunner, checkDatabase databaseChecker) error {
	migrator := db.NewMigrator()
	return runWithServices(ctx, args, stdin, stdout, stderr, serve, checkBackedDatabaseRunner{check: checkDatabase, manager: db.NewManager(migrator)}, migrator)
}

func runWithServices(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, serve serveRunner, database databaseRunner, sync schemaSyncRunner) error {
	return runWithServicesAndSetup(ctx, args, stdin, stdout, stderr, serve, database, sync, defaultAdminSetupRunner{})
}

func runWithServicesAndSetup(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, serve serveRunner, database databaseRunner, sync schemaSyncRunner, setup adminSetupRunner) error {
	return runWithServicesAndSetupAndFixtures(ctx, args, stdin, stdout, stderr, serve, database, sync, setup, defaultFixtureRunner{})
}

func runWithServicesAndSetupAndFixtures(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, serve serveRunner, database databaseRunner, sync schemaSyncRunner, setup adminSetupRunner, fixture fixtureRunner) error {
	return runWithServicesAndSetupAndFixturesAndHooks(ctx, args, stdin, stdout, stderr, serve, database, sync, setup, fixture, defaultPermissionRunner{}, nil, nil)
}

func runWithServicesAndSetupAndFixturesAndHooks(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, serve serveRunner, database databaseRunner, sync schemaSyncRunner, setup adminSetupRunner, fixture fixtureRunner, permission permissionRunner, recordHooks *db.RecordHookRegistry, jobRegistry *jobruntime.Registry) error {
	cmd, err := newRootCommand(ctx, stdin, stdout, stderr, serve, database, sync, setup, fixture, permission, recordHooks, jobRegistry)
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
	recordHooks, err := recordhooks.NewRecordHookRegistry(nil)
	if err != nil {
		return nil, fmt.Errorf("configure record hooks: %w", err)
	}
	jobRegistry, err := jobruntime.NewRegistry(nil)
	if err != nil {
		return nil, fmt.Errorf("configure jobs: %w", err)
	}
	return newRootCommand(ctx, stdin, stdout, stderr, server.Serve, db.NewManager(migrator), migrator, defaultAdminSetupRunner{}, defaultFixtureRunner{recordHooks: recordHooks}, defaultPermissionRunner{}, recordHooks, jobRegistry)
}

func newRootCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, serve serveRunner, database databaseRunner, sync schemaSyncRunner, setup adminSetupRunner, fixture fixtureRunner, permission permissionRunner, recordHooks *db.RecordHookRegistry, jobRegistry *jobruntime.Registry) (*cobra.Command, error) {
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
	if setup == nil {
		return nil, fmt.Errorf("admin setup runner is required")
	}
	if fixture == nil {
		return nil, fmt.Errorf("fixture runner is required")
	}
	if permission == nil {
		return nil, fmt.Errorf("permission runner is required")
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

	root.AddCommand(newProjectCommand(ctx, stdout))
	root.AddCommand(newUpgradeCommand(ctx, stdin, stdout, stderr))
	root.AddCommand(newVersionCommand(stdout))
	root.AddCommand(newDoctorCommand(ctx, stdout))
	root.AddCommand(newDevCommand(ctx, stdout, stderr, serve, recordHooks))
	root.AddCommand(newServeCommand(ctx, stdout, stderr, serve, recordHooks))
	root.AddCommand(newDBCommand(ctx, stdin, stdout, stderr, database, sync, fixture))
	root.AddCommand(newSetupCommand(ctx, stdin, stdout, stderr, setup))
	root.AddCommand(newFixtureCommand(ctx, stdin, stdout, stderr, fixture))
	root.AddCommand(newAppCommand(stdout))
	root.AddCommand(newEntityCommand(stdout))
	root.AddCommand(newHookCommand(stdout))
	root.AddCommand(newGenerateCommand(stdout))
	root.AddCommand(newRouteCommand(stdout))
	root.AddCommand(newPermissionCommand(ctx, stdout, permission))
	root.AddCommand(newSecretCommand(ctx, stdin, stdout, stderr))
	root.AddCommand(newWorkerCommand(ctx, stdout, stderr, recordHooks, jobRegistry))

	return root, nil
}

type checkBackedDatabaseRunner struct {
	check   databaseChecker
	manager databaseRunner
}

type defaultFixtureRunner struct {
	recordHooks *db.RecordHookRegistry
}

func (r defaultFixtureRunner) Apply(ctx context.Context, root string, databaseURL string) (fixtures.Result, error) {
	if r.recordHooks != nil {
		return fixtures.NewRunnerWithHooks(r.recordHooks).Apply(ctx, root, databaseURL)
	}
	return fixtures.NewRunner().Apply(ctx, root, databaseURL)
}

func (r defaultFixtureRunner) Plan(ctx context.Context, root string) (fixtures.Plan, error) {
	if r.recordHooks != nil {
		return fixtures.NewRunnerWithHooks(r.recordHooks).Plan(ctx, root)
	}
	return fixtures.NewRunner().Plan(ctx, root)
}

func (r defaultFixtureRunner) ExportPlan(ctx context.Context, root string, databaseURL string, target shape.AppRef, includeLinks bool) (fixtures.ExportPlan, error) {
	return fixtures.NewRunner().ExportPlan(ctx, root, databaseURL, target, includeLinks)
}

func (r defaultFixtureRunner) WriteExportPlan(ctx context.Context, plan fixtures.ExportPlan) (fixtures.ExportResult, error) {
	if err := ctx.Err(); err != nil {
		return fixtures.ExportResult{}, fmt.Errorf("write fixture export plan: %w", err)
	}
	return fixtures.WriteExportPlan(plan)
}

func (r checkBackedDatabaseRunner) Check(ctx context.Context, databaseURL string) error {
	if r.check != nil {
		return r.check(ctx, databaseURL)
	}
	return r.manager.Check(ctx, databaseURL)
}

func (r checkBackedDatabaseRunner) Exists(ctx context.Context, databaseURL string) (db.DatabaseStatus, error) {
	return r.manager.Exists(ctx, databaseURL)
}

func (r checkBackedDatabaseRunner) Create(ctx context.Context, databaseURL string) (db.DatabaseResult, error) {
	return r.manager.Create(ctx, databaseURL)
}

func (r checkBackedDatabaseRunner) Drop(ctx context.Context, databaseURL string) (db.DatabaseResult, error) {
	return r.manager.Drop(ctx, databaseURL)
}

func newVersionCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the dygo version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err := fmt.Fprintf(stdout, "dygo %s\n", currentVersion()); err != nil {
				return fmt.Errorf("write version: %w", err)
			}
			return nil
		},
	}
}

func newServeCommand(ctx context.Context, stdout, stderr io.Writer, serve serveRunner, recordHooks *db.RecordHookRegistry) *cobra.Command {
	envName := "development"

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the dygo server",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runServerCommand(ctx, stdout, stderr, serve, recordHooks, envName, false, "")
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newDevCommand(ctx context.Context, stdout, stderr io.Writer, serve serveRunner, recordHooks *db.RecordHookRegistry) *cobra.Command {
	envName := "development"
	studioDevURL := ""

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the local dygo development server",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runServerCommand(ctx, stdout, stderr, serve, recordHooks, envName, true, studioDevURL)
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().StringVar(&studioDevURL, "studio-dev-url", studioDevURL, "Proxy Studio UI requests to a frontend dev server")

	return cmd
}

func runServerCommand(ctx context.Context, stdout, stderr io.Writer, serve serveRunner, recordHooks *db.RecordHookRegistry, envName string, devMode bool, studioDevURL string) error {
	env, root, databaseURL, err := databaseInputs(envName)
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	address := cfg.Server.Address()
	if devMode {
		if err := writeDevDiagnostic(stderr, "root %s", root); err != nil {
			return err
		}
		if err := writeDevDiagnostic(stderr, "environment %s", env); err != nil {
			return err
		}
	}
	studioHandler, stopStudio, err := studioHandlerForCommand(ctx, root, stdout, stderr, devMode, studioDevURL)
	if err != nil {
		return err
	}
	if stopStudio != nil {
		defer func() {
			_ = stopStudio()
		}()
	}
	readyPrefix := "dygo serving"
	if devMode {
		readyPrefix = "dygo dev serving"
	}
	if err := serve(ctx, server.Options{
		Address:     address,
		DatabaseURL: databaseURL,
		RecordHooks: recordHooks,
		Studio:      studioHandler,
		OnReady: func(address string) error {
			if devMode {
				if _, err := fmt.Fprintf(stdout, "%s at %s\n", readyPrefix, displayDevServerURL(address)); err != nil {
					return fmt.Errorf("write serve output: %w", err)
				}
				return nil
			}
			if _, err := fmt.Fprintf(stdout, "%s on %s\n", readyPrefix, address); err != nil {
				return fmt.Errorf("write serve output: %w", err)
			}
			return nil
		},
	}); err != nil {
		return fmt.Errorf("serve dygo: %w", err)
	}
	return nil
}

func studioHandlerForCommand(ctx context.Context, root string, stdout, stderr io.Writer, devMode bool, studioDevURL string) (http.Handler, studioDevStop, error) {
	if !devMode {
		handler, _, err := studio.HandlerForProject(root)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve Studio UI: %w", err)
		}
		return handler, nil, nil
	}
	studioURL := studioDevURL
	var stopStudio studioDevStop
	if studioURL == "" {
		var err error
		studioURL, stopStudio, err = startStudioDevServer(ctx, root, stdout, stderr)
		if err != nil {
			return nil, nil, err
		}
	} else if err := writeDevDiagnostic(stderr, "using Studio UI dev server %s", displayDevURL(studioURL)); err != nil {
		return nil, nil, err
	}
	if studioURL == "" {
		handler, _, err := studio.HandlerForProject(root)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve Studio UI: %w", err)
		}
		return handler, nil, nil
	}
	handler, err := server.NewStudioDevProxy(studioURL)
	if err != nil {
		return nil, nil, err
	}
	return handler, stopStudio, nil
}

func startStudioDevServerProcess(ctx context.Context, root string, _ io.Writer, stderr io.Writer) (string, studioDevStop, error) {
	studioDir := filepath.Join(root, "apps", "studio", "ui")
	packagePath := filepath.Join(studioDir, "package.json")
	if _, err := os.Stat(packagePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, nil
		}
		return "", nil, fmt.Errorf("check Studio UI package: %w", err)
	}

	if err := writeDevDiagnostic(stderr, "starting Studio UI dev server in %s", relDevPath(root, studioDir)); err != nil {
		return "", nil, err
	}

	cmd := exec.CommandContext(ctx, "npm", "run", "--silent", "dev", "--", "--clearScreen", "false")
	cmd.Dir = studioDir
	output := newBoundedOutput(32 * 1024)
	cmd.Stdout = io.MultiWriter(output, stderr)
	cmd.Stderr = io.MultiWriter(output, stderr)
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("start Studio UI dev server: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	if err := waitForStudioDevServer(ctx, defaultStudioDevURL, done, output); err != nil {
		_ = stopStudioDevProcess(cmd, done)
		return "", nil, err
	}
	stop := func() error {
		return stopStudioDevProcess(cmd, done)
	}
	return defaultStudioDevURL, stop, nil
}

func writeDevDiagnostic(stderr io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(stderr, "dygo dev: "+format+"\n", args...); err != nil {
		return fmt.Errorf("write dev diagnostic: %w", err)
	}
	return nil
}

func displayDevServerURL(address string) string {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	return "http://" + net.JoinHostPort(displayLoopbackHost(host), port)
}

func displayDevURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return rawURL
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err == nil {
		parsed.Host = net.JoinHostPort(displayLoopbackHost(host), port)
		return parsed.String()
	}
	parsed.Host = displayLoopbackHost(parsed.Host)
	return parsed.String()
}

func displayLoopbackHost(host string) string {
	if host == "127.0.0.1" || host == "::1" {
		return "localhost"
	}
	return host
}

func relDevPath(root string, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.ToSlash(relative)
}

func waitForStudioDevServer(ctx context.Context, studioURL string, done chan error, output *boundedOutput) error {
	client := &http.Client{Timeout: 200 * time.Millisecond}
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, studioURL, nil)
		if err != nil {
			return fmt.Errorf("create Studio readiness request: %w", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < http.StatusInternalServerError {
				return nil
			}
		}

		select {
		case err := <-done:
			done <- err
			if err != nil {
				return studioDevStartupError(err, output.String())
			}
			return studioDevStartupError(nil, output.String())
		case <-ctx.Done():
			return fmt.Errorf("start Studio UI dev server: %w", ctx.Err())
		case <-timeout.C:
			return fmt.Errorf("studio UI dev server did not become ready on %s", studioURL)
		case <-ticker.C:
		}
	}
}

func studioDevStartupError(err error, output string) error {
	output = strings.TrimSpace(output)
	if err != nil {
		if output != "" {
			return fmt.Errorf("start Studio UI dev server: %w\n%s", err, output)
		}
		return fmt.Errorf("start Studio UI dev server: %w", err)
	}
	if output != "" {
		return fmt.Errorf("studio UI dev server exited before dygo started\n%s", output)
	}
	return fmt.Errorf("studio UI dev server exited before dygo started")
}

func stopStudioDevProcess(cmd *exec.Cmd, done <-chan error) error {
	if cmd.Process != nil && cmd.ProcessState == nil {
		_ = cmd.Process.Kill()
	}
	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "signal: killed") && !strings.Contains(err.Error(), "signal: interrupt") {
			return fmt.Errorf("stop Studio UI dev server: %w", err)
		}
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("stop Studio UI dev server: timed out")
	}
}

type boundedOutput struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newBoundedOutput(limit int) *boundedOutput {
	return &boundedOutput{limit: limit}
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if b.limit > 0 && len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return len(p), nil
}

func (b *boundedOutput) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

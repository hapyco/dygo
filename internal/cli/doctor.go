package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/config"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/fixtures"
	"github.com/hapyco/dygo/internal/health"
	"github.com/hapyco/dygo/internal/hookgen"
	"github.com/hapyco/dygo/internal/project"
	routeplan "github.com/hapyco/dygo/internal/routes"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/studio"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
)

type doctorStatus string

const (
	doctorPass doctorStatus = "PASS"
	doctorFail doctorStatus = "FAIL"
	doctorSkip doctorStatus = "SKIP"
)

type doctorResult struct {
	Status doctorStatus
	Name   string
	Detail string
}

type doctorError struct {
	Problems int
}

type doctorRuntimePool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Close()
}

var openDoctorRuntimePool = func(ctx context.Context, databaseURL string) (doctorRuntimePool, error) {
	return db.OpenRuntimePool(ctx, databaseURL)
}

var checkDoctorSchemaSnapshotFreshness = func(ctx context.Context, root string, databaseURL string) error {
	return db.NewMigrator().CheckSchemaSnapshot(ctx, root, databaseURL)
}

func (e doctorError) Error() string {
	if e.Problems == 1 {
		return "dygo doctor found 1 problem"
	}
	return fmt.Sprintf("dygo doctor found %d problems", e.Problems)
}

func newDoctorCommand(ctx context.Context, stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose the current dygo project",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDoctor(ctx, stdout)
		},
	}
}

func runDoctor(ctx context.Context, stdout io.Writer) error {
	root, rootResult := checkProjectRoot()
	results := []doctorResult{
		rootResult,
		checkGoToolchain(ctx),
	}

	if rootResult.Status != doctorPass {
		results = append(results,
			doctorResult{Status: doctorSkip, Name: "app manifests", Detail: "project root not found"},
			doctorResult{Status: doctorSkip, Name: "entity metadata", Detail: "project root not found"},
			doctorResult{Status: doctorSkip, Name: "route registry", Detail: "project root not found"},
			doctorResult{Status: doctorSkip, Name: "fixture files", Detail: "project root not found"},
			doctorResult{Status: doctorSkip, Name: "hook wiring", Detail: "project root not found"},
			doctorResult{Status: doctorSkip, Name: "schema snapshot", Detail: "project root not found"},
			doctorResult{Status: doctorSkip, Name: "Studio assets", Detail: "project root not found"},
			doctorResult{Status: doctorSkip, Name: "config", Detail: "project root not found"},
			doctorResult{Status: doctorSkip, Name: "secrets layout", Detail: "project root not found"},
		)
		return writeDoctorResults(stdout, results)
	}

	apps, appResult := checkAppManifests(root)
	results = append(results, appResult)
	entityResult := doctorResult{Status: doctorSkip, Name: "entity metadata", Detail: "app manifests are invalid"}
	var entities []catalog.LoadedEntity
	if appResult.Status == doctorPass {
		entities, entityResult = checkEntityMetadata(apps)
	}
	results = append(results, entityResult)
	if appResult.Status == doctorPass && entityResult.Status == doctorPass {
		results = append(results,
			checkRouteRegistry(entities),
			checkFixtureFiles(ctx, root),
			checkHookWiring(root),
		)
	} else {
		results = append(results,
			doctorResult{Status: doctorSkip, Name: "route registry", Detail: "app manifests or entity metadata are invalid"},
			doctorResult{Status: doctorSkip, Name: "fixture files", Detail: "app manifests or entity metadata are invalid"},
			doctorResult{Status: doctorSkip, Name: "hook wiring", Detail: "app manifests or entity metadata are invalid"},
		)
	}
	results = append(results,
		checkSchemaSnapshot(root),
		checkStudioAssets(root),
	)
	configResult := checkConfig(root)
	results = append(results, configResult)
	secretsResult := checkSecretsLayout(root)
	results = append(results, secretsResult)

	if appResult.Status == doctorPass && entityResult.Status == doctorPass && configResult.Status == doctorPass && secretsResult.Status == doctorPass {
		results = append(results, checkRuntimeReadiness(ctx, root)...)
	} else {
		results = append(results, doctorResult{Status: doctorSkip, Name: "runtime database", Detail: "config, secrets, or metadata are not ready"})
	}

	return writeDoctorResults(stdout, results)
}

func checkProjectRoot() (string, doctorResult) {
	wd, err := os.Getwd()
	if err != nil {
		return "", doctorResult{Status: doctorFail, Name: "project root", Detail: fmt.Sprintf("detect working directory: %v", err)}
	}
	root, err := project.DiscoverRoot(wd)
	if err != nil {
		return "", doctorResult{Status: doctorFail, Name: "project root", Detail: err.Error()}
	}
	return root.Path, doctorResult{Status: doctorPass, Name: "project root", Detail: root.Path}
}

func checkGoToolchain(ctx context.Context) doctorResult {
	cmd := exec.CommandContext(ctx, "go", "version")
	output, err := cmd.Output()
	if err != nil {
		return doctorResult{Status: doctorFail, Name: "go toolchain", Detail: fmt.Sprintf("go version: %v", err)}
	}
	return doctorResult{Status: doctorPass, Name: "go toolchain", Detail: strings.TrimSpace(string(output))}
}

func checkAppManifests(root string) ([]manifest.LoadedApp, doctorResult) {
	apps, err := project.LoadApps(root)
	if err != nil {
		return nil, doctorResult{Status: doctorFail, Name: "app manifests", Detail: err.Error()}
	}
	return apps, doctorResult{Status: doctorPass, Name: "app manifests", Detail: fmt.Sprintf("%d apps valid", len(apps))}
}

func checkEntityMetadata(apps []manifest.LoadedApp) ([]catalog.LoadedEntity, doctorResult) {
	entities, err := project.LoadEntities(apps)
	if err != nil {
		return nil, doctorResult{Status: doctorFail, Name: "entity metadata", Detail: err.Error()}
	}
	return entities, doctorResult{Status: doctorPass, Name: "entity metadata", Detail: fmt.Sprintf("%d entities valid", len(entities))}
}

func checkRouteRegistry(entities []catalog.LoadedEntity) doctorResult {
	result, err := routeplan.Validate(entities)
	if err != nil {
		return doctorResult{Status: doctorFail, Name: "route registry", Detail: err.Error()}
	}
	return doctorResult{
		Status: doctorPass,
		Name:   "route registry",
		Detail: fmt.Sprintf("%d reserved routes, %d entity routes, %d conflicts", result.ReservedRoutes, result.EntityRoutes, result.Conflicts),
	}
}

func checkFixtureFiles(ctx context.Context, root string) doctorResult {
	plan, err := fixtures.NewRunner().Plan(ctx, root)
	if err != nil {
		return doctorResult{Status: doctorFail, Name: "fixture files", Detail: err.Error()}
	}
	return doctorResult{Status: doctorPass, Name: "fixture files", Detail: fmt.Sprintf("%d files, %d records valid", plan.FileCount(), plan.RecordCount())}
}

func checkHookWiring(root string) doctorResult {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		if os.IsNotExist(err) {
			return doctorResult{Status: doctorSkip, Name: "hook wiring", Detail: "go.mod not found"}
		}
		return doctorResult{Status: doctorFail, Name: "hook wiring", Detail: fmt.Sprintf("stat go.mod: %v", err)}
	}
	if _, err := os.Stat(filepath.Join(root, "cmd", "dygo", "main.go")); err != nil {
		if os.IsNotExist(err) {
			return doctorResult{Status: doctorSkip, Name: "hook wiring", Detail: "generated runner not found"}
		}
		return doctorResult{Status: doctorFail, Name: "hook wiring", Detail: fmt.Sprintf("stat generated runner: %v", err)}
	}
	problems, err := hookgen.Validate(root)
	if err != nil {
		return doctorResult{Status: doctorFail, Name: "hook wiring", Detail: err.Error()}
	}
	if len(problems) > 0 {
		return doctorResult{Status: doctorFail, Name: "hook wiring", Detail: strings.Join(problems, "; ")}
	}
	hooks, err := hookgen.Discover(root)
	if err != nil {
		return doctorResult{Status: doctorFail, Name: "hook wiring", Detail: err.Error()}
	}
	return doctorResult{Status: doctorPass, Name: "hook wiring", Detail: fmt.Sprintf("%d hook files wired", len(hooks))}
}

func checkSchemaSnapshot(root string) doctorResult {
	path := filepath.Join(root, filepath.FromSlash(shape.SchemaSnapshot))
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return doctorResult{Status: doctorFail, Name: "schema snapshot", Detail: fmt.Sprintf("missing %s; run dygo db migrate", shape.SchemaSnapshot)}
		}
		return doctorResult{Status: doctorFail, Name: "schema snapshot", Detail: fmt.Sprintf("stat %s: %v", shape.SchemaSnapshot, err)}
	}
	if info.IsDir() {
		return doctorResult{Status: doctorFail, Name: "schema snapshot", Detail: fmt.Sprintf("%s is a directory; run dygo db migrate", shape.SchemaSnapshot)}
	}
	return doctorResult{Status: doctorPass, Name: "schema snapshot", Detail: fmt.Sprintf("%s present", shape.SchemaSnapshot)}
}

func checkSchemaSnapshotFreshness(ctx context.Context, root string, databaseURL string) doctorResult {
	path := filepath.Join(root, filepath.FromSlash(shape.SchemaSnapshot))
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return doctorResult{Status: doctorSkip, Name: "schema snapshot freshness", Detail: fmt.Sprintf("%s is missing", shape.SchemaSnapshot)}
		}
		return doctorResult{Status: doctorFail, Name: "schema snapshot freshness", Detail: fmt.Sprintf("stat %s: %v", shape.SchemaSnapshot, err)}
	}
	if info.IsDir() {
		return doctorResult{Status: doctorSkip, Name: "schema snapshot freshness", Detail: fmt.Sprintf("%s is not a file", shape.SchemaSnapshot)}
	}
	if err := checkDoctorSchemaSnapshotFreshness(ctx, root, databaseURL); err != nil {
		if errors.Is(err, db.ErrSchemaSnapshotOutOfDate) {
			return doctorResult{Status: doctorFail, Name: "schema snapshot freshness", Detail: fmt.Sprintf("%s is out of date; run dygo db migrate", shape.SchemaSnapshot)}
		}
		if errors.Is(err, db.ErrSchemaSnapshotMissing) {
			return doctorResult{Status: doctorFail, Name: "schema snapshot freshness", Detail: fmt.Sprintf("%s is missing; run dygo db migrate", shape.SchemaSnapshot)}
		}
		return doctorResult{Status: doctorFail, Name: "schema snapshot freshness", Detail: err.Error()}
	}
	return doctorResult{Status: doctorPass, Name: "schema snapshot freshness", Detail: fmt.Sprintf("%s matches live database", shape.SchemaSnapshot)}
}

func checkStudioAssets(root string) doctorResult {
	_, source, err := studio.HandlerForProject(root)
	if err != nil {
		return doctorResult{Status: doctorFail, Name: "Studio assets", Detail: err.Error()}
	}
	return doctorResult{Status: doctorPass, Name: "Studio assets", Detail: source + " available"}
}

func checkConfig(root string) doctorResult {
	cfg, err := config.Load(root)
	if err != nil {
		return doctorResult{Status: doctorFail, Name: "config", Detail: err.Error()}
	}
	return doctorResult{Status: doctorPass, Name: "config", Detail: fmt.Sprintf("%s server=%s database=%s secret=%s", config.FilePath, cfg.Server.Address(), cfg.Database.Driver, cfg.Database.URL.Secret)}
}

func checkSecretsLayout(root string) doctorResult {
	store := secrets.NewStore(root)
	envs := []secrets.Environment{
		secrets.EnvironmentDevelopment,
		secrets.EnvironmentStaging,
		secrets.EnvironmentProduction,
	}

	var required []requiredPath
	required = append(required, requiredPath{Path: relToRoot(root, store.Paths(secrets.EnvironmentDevelopment).MasterKeyFile)})
	for _, env := range envs {
		paths := store.Paths(env)
		required = append(required,
			requiredPath{Path: relToRoot(root, paths.SecretFile)},
		)
	}

	missing := missingRequiredPaths(root, required)
	if len(missing) > 0 {
		return doctorResult{Status: doctorFail, Name: "secrets layout", Detail: "missing " + strings.Join(missing, ", ")}
	}
	return doctorResult{Status: doctorPass, Name: "secrets layout", Detail: fmt.Sprintf("%d environments configured", len(envs))}
}

func checkRuntimeReadiness(ctx context.Context, root string) []doctorResult {
	env := secrets.EnvironmentDevelopment
	databaseURL, err := doctorDatabaseURL(root, env)
	if err != nil {
		return []doctorResult{
			{Status: doctorFail, Name: "runtime database", Detail: fmt.Sprintf("%v; run dygo secret edit", err)},
			{Status: doctorSkip, Name: "core fixtures", Detail: "runtime database is not ready"},
			{Status: doctorSkip, Name: "administrator account", Detail: "runtime database is not ready"},
		}
	}

	pool, err := openDoctorRuntimePool(ctx, databaseURL)
	if err != nil {
		return []doctorResult{
			{Status: doctorFail, Name: "runtime database", Detail: fmt.Sprintf("%v; run dygo db migrate", err)},
			{Status: doctorSkip, Name: "core fixtures", Detail: "runtime database is not ready"},
			{Status: doctorSkip, Name: "administrator account", Detail: "runtime database is not ready"},
		}
	}
	defer pool.Close()

	results := []doctorResult{
		{Status: doctorPass, Name: "runtime database", Detail: string(env) + " database reachable"},
		checkSchemaSnapshotFreshness(ctx, root, databaseURL),
	}
	for _, check := range health.CoreRuntimeChecks(ctx, pool) {
		results = append(results, doctorResultFromHealth(check))
	}
	return results
}

func doctorDatabaseURL(root string, env secrets.Environment) (string, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}
	secretName := cfg.Database.URL.Secret
	databaseURL, err := databaseURLForEnvironment(root, env, secretName)
	if err != nil {
		return "", fmt.Errorf("read database secret %q for %s: %w", secretName, env, err)
	}
	return databaseURL, nil
}

func doctorResultFromHealth(check health.CheckResult) doctorResult {
	status := doctorFail
	if check.Ready {
		status = doctorPass
	}
	return doctorResult{Status: status, Name: check.Name, Detail: check.Detail}
}

func writeDoctorResults(stdout io.Writer, results []doctorResult) error {
	problems := 0
	for _, result := range results {
		if result.Status == doctorFail {
			problems++
		}
		if _, err := fmt.Fprintf(stdout, "%s %s: %s\n", result.Status, result.Name, result.Detail); err != nil {
			return fmt.Errorf("write doctor output: %w", err)
		}
	}

	if problems > 0 {
		err := doctorError{Problems: problems}
		if _, writeErr := fmt.Fprintln(stdout, err.Error()); writeErr != nil {
			return fmt.Errorf("write doctor summary: %w", writeErr)
		}
		return err
	}

	if _, err := fmt.Fprintln(stdout, "dygo doctor passed"); err != nil {
		return fmt.Errorf("write doctor summary: %w", err)
	}
	return nil
}

type requiredPath struct {
	Path      string
	Directory bool
}

func missingRequiredPaths(root string, paths []requiredPath) []string {
	var missing []string
	for _, path := range paths {
		clean := filepath.Clean(path.Path)
		info, err := os.Stat(filepath.Join(root, clean))
		if err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, filepath.ToSlash(clean))
				continue
			}
			missing = append(missing, fmt.Sprintf("%s (%v)", filepath.ToSlash(clean), err))
			continue
		}
		if path.Directory && !info.IsDir() {
			missing = append(missing, filepath.ToSlash(clean)+" (not a directory)")
		}
		if !path.Directory && info.IsDir() {
			missing = append(missing, filepath.ToSlash(clean)+" (not a file)")
		}
	}
	return missing
}

func relToRoot(root string, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Clean(path)
	}
	return relative
}

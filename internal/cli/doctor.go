package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dygo-dev/dygo/internal/app/manifest"
	appregistry "github.com/dygo-dev/dygo/internal/app/registry"
	"github.com/dygo-dev/dygo/internal/config"
	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/dygo-dev/dygo/internal/project"
	"github.com/dygo-dev/dygo/internal/secrets"
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
			doctorResult{Status: doctorSkip, Name: "config", Detail: "project root not found"},
			doctorResult{Status: doctorSkip, Name: "secrets layout", Detail: "project root not found"},
		)
		return writeDoctorResults(stdout, results)
	}

	apps, appResult := checkAppManifests(root)
	results = append(results, appResult)
	if appResult.Status == doctorPass {
		results = append(results, checkEntityMetadata(apps))
	} else {
		results = append(results, doctorResult{Status: doctorSkip, Name: "entity metadata", Detail: "app manifests are invalid"})
	}
	results = append(results, checkConfig(root))
	results = append(results, checkSecretsLayout(root))

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
	apps, err := appregistry.New(root).Validate()
	if err != nil {
		return nil, doctorResult{Status: doctorFail, Name: "app manifests", Detail: err.Error()}
	}
	return apps, doctorResult{Status: doctorPass, Name: "app manifests", Detail: fmt.Sprintf("%d apps valid", len(apps))}
}

func checkEntityMetadata(apps []manifest.LoadedApp) doctorResult {
	entities, err := catalog.New(apps, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		return doctorResult{Status: doctorFail, Name: "entity metadata", Detail: err.Error()}
	}
	return doctorResult{Status: doctorPass, Name: "entity metadata", Detail: fmt.Sprintf("%d entities valid", len(entities))}
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

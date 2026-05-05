package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/db"
	"github.com/dygo-dev/dygo/internal/secrets"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantStdout string
		wantStderr string
		wantErr    bool
	}{
		{
			name:       "prints help without args",
			args:       nil,
			wantStdout: "Usage:",
		},
		{
			name:       "prints version",
			args:       []string{"version"},
			wantStdout: "dygo dev",
		},
		{
			name:       "prints no apps message",
			args:       []string{"apps", "list"},
			wantStdout: "No apps found.",
		},
		{
			name:       "validates empty app set",
			args:       []string{"apps", "validate"},
			wantStdout: "0 apps are valid",
		},
		{
			name:       "validates empty entity set",
			args:       []string{"entities", "validate"},
			wantStdout: "0 entities are valid",
		},
		{
			name:       "prints no apps message for entity list",
			args:       []string{"entities", "list"},
			wantStdout: "No apps found.",
		},
		{
			name:    "rejects unknown command",
			args:    []string{"missing"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeCLIProjectRoot(t, root)
			writeCLIConfig(t, root)
			t.Chdir(root)

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			err := Run(context.Background(), tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if tt.wantErr && err == nil {
				t.Fatal("Run() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Fatalf("stdout = %q, want substring %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRootHelpIncludesServeAndDB(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), nil, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(help) error = %v, want nil", err)
	}
	for _, want := range []string{
		"Available Commands:",
		"serve",
		"Start the dygo server",
		"db",
		"Manage dygo database lifecycle",
		"migrate",
		"Sync dygo metadata to the database",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}

func TestServeCommandLoadsProjectConfig(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfigBody(t, root, `
server:
  host: 0.0.0.0
  port: 7777
database:
  url:
    secret: DATABASE_URL
`)
	nested := filepath.Join(root, "apps", "sales")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll(nested) error = %v", err)
	}
	t.Chdir(nested)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotAddress string
	err := run(context.Background(), []string{"serve"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, address string) error {
		gotAddress = address
		return nil
	}, noopDatabaseChecker)
	if err != nil {
		t.Fatalf("Run(serve) error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "dygo serving on 0.0.0.0:7777") {
		t.Fatalf("serve stdout = %q, want configured address", stdout.String())
	}
	if gotAddress != "0.0.0.0:7777" {
		t.Fatalf("serve address = %q, want configured address", gotAddress)
	}
}

func TestServeCommandRequiresProjectConfig(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	called := false
	err := run(context.Background(), []string{"serve"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, _ string) error {
		called = true
		return nil
	}, noopDatabaseChecker)
	if err == nil {
		t.Fatal("Run(serve) error = nil, want missing config error")
	}
	for _, want := range []string{"load config", "configs/dygo.yaml"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(serve) error = %q, want substring %q", err.Error(), want)
		}
	}
	if called {
		t.Fatal("serve runner was called for missing config")
	}
}

func TestServeCommandReturnsRunnerError(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"serve"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, _ string) error {
		return errors.New("listen failed")
	}, noopDatabaseChecker)
	if err == nil {
		t.Fatal("Run(serve) error = nil, want runner error")
	}
	for _, want := range []string{"serve dygo", "listen failed"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(serve) error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestDBCheckCommandDefaultsToDevelopment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotURL string
	err := run(context.Background(), []string{"db", "check"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, func(_ context.Context, url string) error {
		gotURL = url
		return nil
	})
	if err != nil {
		t.Fatalf("Run(db check) error = %v, want nil", err)
	}
	if stdout.String() != "database connected (development)\n" {
		t.Fatalf("db check stdout = %q, want development success", stdout.String())
	}
	if gotURL != databaseURL {
		t.Fatalf("database checker URL = %q, want %q", gotURL, databaseURL)
	}
}

func TestDBCheckCommandUsesSelectedEnvironment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://staging-user:secret-password@localhost:5432/dygo_staging"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentStaging, databaseURL)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotURL string
	err := run(context.Background(), []string{"db", "check", "--env", "staging"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, func(_ context.Context, url string) error {
		gotURL = url
		return nil
	})
	if err != nil {
		t.Fatalf("Run(db check --env staging) error = %v, want nil", err)
	}
	if stdout.String() != "database connected (staging)\n" {
		t.Fatalf("db check stdout = %q, want staging success", stdout.String())
	}
	if gotURL != databaseURL {
		t.Fatalf("database checker URL = %q, want %q", gotURL, databaseURL)
	}
}

func TestDBCheckCommandRequiresSecret(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(true); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	called := false
	err := run(context.Background(), []string{"db", "check"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, func(context.Context, string) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("Run(db check) error = nil, want missing secret error")
	}
	for _, want := range []string{`read database secret "DATABASE_URL" for development`, `secret "DATABASE_URL" is not defined`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(db check) error = %q, want substring %q", err.Error(), want)
		}
	}
	if called {
		t.Fatal("database checker was called without DATABASE_URL")
	}
}

func TestDBCheckCommandReturnsConnectionFailure(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"db", "check"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, func(context.Context, string) error {
		return errors.New("ping postgres failed")
	})
	if err == nil {
		t.Fatal("Run(db check) error = nil, want checker error")
	}
	if !strings.Contains(err.Error(), "check database: ping postgres failed") {
		t.Fatalf("Run(db check) error = %q, want checker context", err.Error())
	}
	if strings.Contains(err.Error(), databaseURL) || strings.Contains(err.Error(), "secret-password") {
		t.Fatalf("Run(db check) error = %q, leaked database URL", err.Error())
	}
}

func TestDBCreateCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	fake := &fakeDatabaseRunner{
		createResult: db.DatabaseResult{Name: "dygo", Changed: true},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "create"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, fake, &fakeSchemaSyncRunner{})
	if err != nil {
		t.Fatalf("Run(db create) error = %v, want nil", err)
	}
	if stdout.String() != "database created: dygo (development)\n" {
		t.Fatalf("db create stdout = %q, want created output", stdout.String())
	}
	if fake.operation != "create" || fake.databaseURL != databaseURL {
		t.Fatalf("database runner = operation %q URL %q, want create and URL", fake.operation, fake.databaseURL)
	}
}

func TestDBCreateCommandAlreadyExists(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeDatabaseRunner{
		createResult: db.DatabaseResult{Name: "dygo", Changed: false},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "create"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, fake, &fakeSchemaSyncRunner{})
	if err != nil {
		t.Fatalf("Run(db create exists) error = %v, want nil", err)
	}
	if stdout.String() != "database already exists: dygo (development)\n" {
		t.Fatalf("db create stdout = %q, want already exists output", stdout.String())
	}
}

func TestDBDropCommandRequiresForce(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	fake := &fakeDatabaseRunner{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "drop"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, fake, &fakeSchemaSyncRunner{})
	if err == nil {
		t.Fatal("Run(db drop) error = nil, want force error")
	}
	if !strings.Contains(err.Error(), "db drop requires --force") {
		t.Fatalf("Run(db drop) error = %q, want force context", err.Error())
	}
	if fake.calls != 0 {
		t.Fatalf("database runner calls = %d, want 0", fake.calls)
	}
}

func TestDBDropCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://staging-user:secret-password@localhost:5432/dygo_staging"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentStaging, databaseURL)
	t.Chdir(root)

	fake := &fakeDatabaseRunner{
		dropResult: db.DatabaseResult{Name: "dygo_staging", Changed: true},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "drop", "--force", "--env", "staging"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, fake, &fakeSchemaSyncRunner{})
	if err != nil {
		t.Fatalf("Run(db drop) error = %v, want nil", err)
	}
	if stdout.String() != "database dropped: dygo_staging (staging)\n" {
		t.Fatalf("db drop stdout = %q, want dropped output", stdout.String())
	}
	if fake.operation != "drop" || fake.databaseURL != databaseURL {
		t.Fatalf("database runner = operation %q URL %q, want drop and URL", fake.operation, fake.databaseURL)
	}
}

func TestDBPrepareCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	fake := &fakeDatabaseRunner{
		prepareResult: db.SchemaSyncResult{Entities: 8, Fields: 34},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "prepare"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, fake, &fakeSchemaSyncRunner{})
	if err != nil {
		t.Fatalf("Run(db prepare) error = %v, want nil", err)
	}
	if stdout.String() != "database prepared: synced 8 entities, 34 fields (development)\n" {
		t.Fatalf("db prepare stdout = %q, want prepare output", stdout.String())
	}
	if fake.operation != "prepare" || fake.root != root || fake.databaseURL != databaseURL {
		t.Fatalf("database runner = operation %q root %q URL %q, want prepare/root/URL", fake.operation, fake.root, fake.databaseURL)
	}
}

func TestDBResetCommandRequiresForce(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	fake := &fakeDatabaseRunner{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "reset"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, fake, &fakeSchemaSyncRunner{})
	if err == nil {
		t.Fatal("Run(db reset) error = nil, want force error")
	}
	if !strings.Contains(err.Error(), "db reset requires --force") {
		t.Fatalf("Run(db reset) error = %q, want force context", err.Error())
	}
	if fake.calls != 0 {
		t.Fatalf("database runner calls = %d, want 0", fake.calls)
	}
}

func TestDBResetCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeDatabaseRunner{
		resetResult: db.SchemaSyncResult{Entities: 8, Fields: 34},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "reset", "--force"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, fake, &fakeSchemaSyncRunner{})
	if err != nil {
		t.Fatalf("Run(db reset) error = %v, want nil", err)
	}
	if stdout.String() != "database reset: synced 8 entities, 34 fields (development)\n" {
		t.Fatalf("db reset stdout = %q, want reset output", stdout.String())
	}
}

func TestDBSchemaDumpCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	fake := &fakeDatabaseRunner{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "schema", "dump"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, fake, &fakeSchemaSyncRunner{})
	if err != nil {
		t.Fatalf("Run(db schema dump) error = %v, want nil", err)
	}
	if stdout.String() != "schema dumped to db/schema.sql (development)\n" {
		t.Fatalf("db schema dump stdout = %q, want dump output", stdout.String())
	}
	if fake.operation != "schema-dump" || fake.root != root || fake.databaseURL != databaseURL {
		t.Fatalf("database runner = operation %q root %q URL %q, want schema dump/root/URL", fake.operation, fake.root, fake.databaseURL)
	}
}

func TestMigrateCommandDefaultsToDevelopment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	fake := &fakeSchemaSyncRunner{
		result: db.SchemaSyncResult{Entities: 8, Fields: 34},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"migrate"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), fake)
	if err != nil {
		t.Fatalf("Run(migrate) error = %v, want nil", err)
	}
	if stdout.String() != "metadata synced: 8 entities, 34 fields (development)\n" {
		t.Fatalf("migrate stdout = %q, want synced output", stdout.String())
	}
	if fake.root != root {
		t.Fatalf("schema sync root = %q, want %q", fake.root, root)
	}
	if fake.databaseURL != databaseURL {
		t.Fatalf("schema sync database URL = %q, want %q", fake.databaseURL, databaseURL)
	}
}

func TestMigrateCommandUsesEnvironment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://staging-user:secret-password@localhost:5432/dygo_staging"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentStaging, databaseURL)
	t.Chdir(root)

	fake := &fakeSchemaSyncRunner{
		result: db.SchemaSyncResult{Entities: 3, Fields: 9},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"migrate", "--env", "staging"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), fake)
	if err != nil {
		t.Fatalf("Run(migrate --env staging) error = %v, want nil", err)
	}
	if stdout.String() != "metadata synced: 3 entities, 9 fields (staging)\n" {
		t.Fatalf("migrate stdout = %q, want synced output", stdout.String())
	}
	if fake.databaseURL != databaseURL {
		t.Fatalf("schema sync database URL = %q, want %q", fake.databaseURL, databaseURL)
	}
}

func TestMigrateCommandRequiresSecret(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(true); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	t.Chdir(root)

	fake := &fakeSchemaSyncRunner{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"migrate"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), fake)
	if err == nil {
		t.Fatal("Run(migrate) error = nil, want missing secret error")
	}
	for _, want := range []string{`read database secret "DATABASE_URL" for development`, `secret "DATABASE_URL" is not defined`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(migrate) error = %q, want substring %q", err.Error(), want)
		}
	}
	if fake.calls != 0 {
		t.Fatalf("schema sync runner calls = %d, want 0", fake.calls)
	}
}

func TestAppsListCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	if err := os.MkdirAll(filepath.Join(root, ".dygo", "apps", "core"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.dygo/apps/core) error = %v", err)
	}
	corePath := filepath.Join(root, ".dygo", "apps", "core", "app.yml")
	core := []byte("name: core\nlabel: Core\nversion: 0.1.0\n")
	if err := os.WriteFile(corePath, core, 0o644); err != nil {
		t.Fatalf("WriteFile(core app.yml) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, "apps", "sales"), 0o755); err != nil {
		t.Fatalf("MkdirAll(apps/sales) error = %v", err)
	}
	manifestPath := filepath.Join(root, "apps", "sales", "app.yml")
	manifest := []byte("name: sales\nlabel: Sales\nversion: 0.1.0\ndependencies:\n  - core\n")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("WriteFile(app.yml) error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"apps", "list"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(apps list) error = %v, want nil", err)
	}

	output := stdout.String()
	for _, want := range []string{"NAME", "VERSION", "LABEL", "core", "Core", "sales", "Sales", "0.1.0"} {
		if !strings.Contains(output, want) {
			t.Fatalf("apps list stdout = %q, want substring %q", output, want)
		}
	}
}

func TestAppsValidateCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	if err := os.MkdirAll(filepath.Join(root, ".dygo", "apps", "core"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.dygo/apps/core) error = %v", err)
	}
	corePath := filepath.Join(root, ".dygo", "apps", "core", "app.yml")
	core := []byte("name: core\nlabel: Core\nversion: 0.1.0\n")
	if err := os.WriteFile(corePath, core, 0o644); err != nil {
		t.Fatalf("WriteFile(core app.yml) error = %v", err)
	}

	if err := os.MkdirAll(filepath.Join(root, "apps", "sales"), 0o755); err != nil {
		t.Fatalf("MkdirAll(apps/sales) error = %v", err)
	}
	manifestPath := filepath.Join(root, "apps", "sales", "app.yml")
	manifest := []byte("name: sales\nlabel: Sales\nversion: 0.1.0\ndependencies:\n  - core\n")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("WriteFile(app.yml) error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"apps", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(apps validate) error = %v, want nil", err)
	}
	if stdout.String() != "2 apps are valid\n" {
		t.Fatalf("apps validate stdout = %q, want success count", stdout.String())
	}
}

func TestAppsValidateCommandRejectsInvalidAppSet(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	if err := os.MkdirAll(filepath.Join(root, "apps", "sales"), 0o755); err != nil {
		t.Fatalf("MkdirAll(apps/sales) error = %v", err)
	}
	manifestPath := filepath.Join(root, "apps", "sales", "app.yml")
	manifest := []byte("name: sales\nlabel: Sales\nversion: 0.1.0\ndependencies:\n  - core\n")
	if err := os.WriteFile(manifestPath, manifest, 0o644); err != nil {
		t.Fatalf("WriteFile(app.yml) error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"apps", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(apps validate) error = nil, want missing dependency error")
	}
	if !strings.Contains(err.Error(), "unknown app") {
		t.Fatalf("Run(apps validate) error = %q, want unknown app", err.Error())
	}
}

func TestAppsValidateCommandRejectsMissingProjectRoot(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"apps", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(apps validate) error = nil, want missing project root error")
	}
	if !strings.Contains(err.Error(), "no dygo project root found") {
		t.Fatalf("Run(apps validate) error = %q, want missing project root", err.Error())
	}
}

func TestDoctorCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLISecretsLayout(t, root)
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "company.yml"), `
name: company
label: Company
plural-name: companies
plural-label: Companies
fields:
  - name: title
    label: Title
    type: text
`)
	t.Chdir(filepath.Join(root, "apps", "sales", "entities"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"doctor"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(doctor) error = %v, want nil", err)
	}

	output := stdout.String()
	for _, want := range []string{
		"PASS project root:",
		"PASS go toolchain:",
		"PASS app manifests: 1 apps valid",
		"PASS entity metadata: 1 entities valid",
		"PASS config: configs/dygo.yaml server=127.0.0.1:6790",
		"PASS secrets layout: 3 environments configured",
		"dygo doctor passed",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor stdout = %q, want substring %q", output, want)
		}
	}
}

func TestDoctorCommandReportsFailures(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	writeCLIAppWithBody(t, filepath.Join(root, "apps", "sales"), `
name: sales
label: Sales
version: 0.1.0
dependencies:
  - core
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"doctor"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(doctor) error = nil, want diagnostics failure")
	}

	output := stdout.String()
	for _, want := range []string{
		"PASS project root:",
		"PASS go toolchain:",
		"FAIL app manifests:",
		"SKIP entity metadata: app manifests are invalid",
		"FAIL config:",
		"configs/dygo.yaml",
		"FAIL secrets layout:",
		"dygo doctor found",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor stdout = %q, want substring %q", output, want)
		}
	}
}

func TestDoctorCommandReportsInvalidConfig(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfigBody(t, root, `
server:
  port: 70000
database:
  url:
    secret: DATABASE_URL
`)
	writeCLISecretsLayout(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"doctor"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(doctor) error = nil, want invalid config failure")
	}

	output := stdout.String()
	for _, want := range []string{
		"PASS project root:",
		"PASS app manifests: 0 apps valid",
		"PASS entity metadata: 0 entities valid",
		"FAIL config:",
		"server.port must be between 1 and 65535",
		"PASS secrets layout: 3 environments configured",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor stdout = %q, want substring %q", output, want)
		}
	}
}

func TestEntitiesValidateCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "company.yml"), `
name: company
label: Company
plural-name: companies
plural-label: Companies
fields:
  - name: title
    label: Title
    type: text
`)
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "lead.yml"), `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: company
    label: Company
    type: link
    options:
      entity: company
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"entities", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(entities validate) error = %v, want nil", err)
	}
	if stdout.String() != "2 entities are valid\n" {
		t.Fatalf("entities validate stdout = %q, want success count", stdout.String())
	}
}

func TestEntitiesListCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)

	writeCLIApp(t, filepath.Join(root, "apps", "core"), "core")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "company.yml"), `
name: company
label: Company
plural-name: companies
plural-label: Companies
fields:
  - name: title
    label: Title
    type: text
`)
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "lead.yml"), `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: company
    label: Company
    type: link
    options:
      entity: company
`)

	t.Chdir(filepath.Join(root, "apps", "sales", "entities"))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"entities", "list"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(entities list) error = %v, want nil", err)
	}

	want := "core\n  (no entities)\nsales\n  - company\n  - lead\n"
	if stdout.String() != want {
		t.Fatalf("entities list stdout = %q, want %q", stdout.String(), want)
	}
}

func TestEntitiesValidateCommandRejectsInvalidTargets(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	entityPath := filepath.Join(root, "apps", "sales", "entities", "lead.yml")
	writeCLIEntity(t, entityPath, `
name: lead
label: Lead
plural-name: leads
plural-label: Leads
fields:
  - name: company
    label: Company
    type: link
    options:
      entity: company
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"entities", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(entities validate) error = nil, want missing target error")
	}
	wantPath := filepath.ToSlash(filepath.Join("apps", "sales", "entities", "lead.yml")) + ":6"
	for _, want := range []string{wantPath, `field "company"`, `unknown entity target "company"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(entities validate) error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestSecretsCommandSurface(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	run := func(args []string, stdin string) (string, string, error) {
		t.Helper()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		err := Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
		return stdout.String(), stderr.String(), err
	}

	stdout, _, err := run([]string{"secrets", "--help"}, "")
	if err != nil {
		t.Fatalf("secrets --help error = %v", err)
	}
	for _, want := range []string{"init", "edit", "validate", "rotate-key"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("secrets help = %q, want command %q", stdout, want)
		}
	}
	for _, removed := range []string{"set", "get", "show", "list", "remove"} {
		if strings.Contains(stdout, removed+" ") {
			t.Fatalf("secrets help = %q, should not include removed command %q", stdout, removed)
		}
		if _, _, err := run([]string{"secrets", removed}, ""); err == nil {
			t.Fatalf("secrets %s error = nil, want unknown command", removed)
		}
	}
}

func TestSecretsInitCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secrets", "init"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secrets init) error = %v, want nil", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"initialized secrets",
		"key: master.key",
		"configs/secrets/development.age.yaml",
		"configs/secrets/staging.age.yaml",
		"configs/secrets/production.age.yaml",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("secrets init stdout = %q, want substring %q", output, want)
		}
	}

	info, err := os.Stat(filepath.Join(root, "master.key"))
	if err != nil {
		t.Fatalf("Stat(master.key) error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("master.key mode = %v, want 0600", got)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run(context.Background(), []string{"secrets", "init"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("second secrets init error = %v, want nil", err)
	}
}

func TestSecretsEditDefaultsToDevelopment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(false); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	editor := writeEditorScript(t, root, `
cat > "$1" <<'YAML'
version: 1
environment: development
secrets:
  DATABASE_URL:
    value: postgres://development
    updated_at: 2026-05-03T08:00:00Z
YAML
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secrets", "edit", "--editor", editor}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secrets edit) error = %v, want nil", err)
	}
	if stdout.String() != "updated development secrets\n" {
		t.Fatalf("secrets edit stdout = %q, want development update", stdout.String())
	}
	secret, err := store.Get(secrets.EnvironmentDevelopment, "DATABASE_URL")
	if err != nil {
		t.Fatalf("Get(development DATABASE_URL) error = %v", err)
	}
	if secret.Value != "postgres://development" {
		t.Fatalf("DATABASE_URL = %q, want development value", secret.Value)
	}
}

func TestSecretsEditSelectedEnvironmentAndEditorArgs(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(false); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	editor := writeEditorScript(t, root, `
if [ "$1" != "--flag" ]; then
  exit 12
fi
cat > "$2" <<'YAML'
version: 1
environment: staging
secrets:
  DATABASE_URL:
    value: postgres://staging
    updated_at: 2026-05-03T08:00:00Z
YAML
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secrets", "edit", "--env", "staging", "--editor", editor + " --flag"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secrets edit --env staging) error = %v, want nil", err)
	}
	if stdout.String() != "updated staging secrets\n" {
		t.Fatalf("secrets edit stdout = %q, want staging update", stdout.String())
	}
	secret, err := store.Get(secrets.EnvironmentStaging, "DATABASE_URL")
	if err != nil {
		t.Fatalf("Get(staging DATABASE_URL) error = %v", err)
	}
	if secret.Value != "postgres://staging" {
		t.Fatalf("DATABASE_URL = %q, want staging value", secret.Value)
	}
}

func TestSecretsEditInvalidYAMLDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(false); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(secrets.EnvironmentDevelopment, "DATABASE_URL", "postgres://old"); err != nil {
		t.Fatalf("Set(old DATABASE_URL) error = %v", err)
	}
	editor := writeEditorScript(t, root, `
cat > "$1" <<'YAML'
version: 1
environment: development
secrets:
  database_url:
    value: bad
YAML
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secrets", "edit", "--editor", editor}, strings.NewReader("no\n"), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(secrets edit invalid) error = nil, want validation error")
	}
	if !strings.Contains(stderr.String(), "invalid secrets document") {
		t.Fatalf("secrets edit stderr = %q, want validation diagnostic", stderr.String())
	}
	secret, err := store.Get(secrets.EnvironmentDevelopment, "DATABASE_URL")
	if err != nil {
		t.Fatalf("Get(old DATABASE_URL) error = %v", err)
	}
	if secret.Value != "postgres://old" {
		t.Fatalf("DATABASE_URL after invalid edit = %q, want original value", secret.Value)
	}
}

func TestSecretsEditorDefaultsToNano(t *testing.T) {
	root := t.TempDir()
	nanoPath := filepath.Join(root, "nano")
	if err := os.WriteFile(nanoPath, []byte("#!/bin/sh\nprintf 'nano-default\\n' > \"$1\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(nano) error = %v", err)
	}
	targetPath := filepath.Join(root, "secrets.yaml")
	if err := os.WriteFile(targetPath, []byte("original\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}

	t.Setenv("EDITOR", "vim")
	t.Setenv("PATH", root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := openEditor(context.Background(), "", targetPath, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("openEditor() error = %v, want nil", err)
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile(target) error = %v", err)
	}
	if string(data) != "nano-default\n" {
		t.Fatalf("edited file = %q, want nano default output", data)
	}
}

func TestSecretsValidateDefaultsToDevelopment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(false); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(secrets.EnvironmentDevelopment, "DATABASE_URL", "postgres://development"); err != nil {
		t.Fatalf("Set(DATABASE_URL) error = %v", err)
	}
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secrets", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secrets validate) error = %v, want nil", err)
	}
	if stdout.String() != "development secrets are valid\n" {
		t.Fatalf("secrets validate stdout = %q, want development success", stdout.String())
	}
}

func TestSecretsRotateKeyCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(false); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(secrets.EnvironmentProduction, "DATABASE_URL", "postgres://production"); err != nil {
		t.Fatalf("Set(production DATABASE_URL) error = %v", err)
	}
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secrets", "rotate-key"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secrets rotate-key) error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "rotated secrets master key") || !strings.Contains(stdout.String(), "master.key") {
		t.Fatalf("secrets rotate-key stdout = %q, want rotate output", stdout.String())
	}
	secret, err := store.Get(secrets.EnvironmentProduction, "DATABASE_URL")
	if err != nil {
		t.Fatalf("Get(production DATABASE_URL) error = %v", err)
	}
	if secret.Value != "postgres://production" {
		t.Fatalf("production DATABASE_URL after rotate = %q, want preserved value", secret.Value)
	}
}

func writeCLIApp(t *testing.T, dir string, name string) {
	t.Helper()

	writeCLIAppWithBody(t, dir, "name: "+name+"\nlabel: "+strings.ToUpper(name[:1])+name[1:]+"\nversion: 0.1.0\n")
}

func writeCLIAppWithBody(t *testing.T, dir string, body string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.yml"), []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(app.yml) error = %v", err)
	}
}

func writeCLIProjectRoot(t *testing.T, root string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(root, "dygo.yml"), []byte("name: test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(dygo.yml) error = %v", err)
	}
}

func writeCLIConfig(t *testing.T, root string) {
	t.Helper()

	writeCLIConfigBody(t, root, `
server:
  port: 6790
database:
  url:
    secret: DATABASE_URL
`)
}

func writeCLIConfigBody(t *testing.T, root string, body string) {
	t.Helper()

	configPath := filepath.Join(root, "configs", "dygo.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(configs) error = %v", err)
	}
	if err := os.WriteFile(configPath, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(dygo.yaml) error = %v", err)
	}
}

func writeCLISecretsLayout(t *testing.T, root string) {
	t.Helper()

	store := secrets.NewStore(root)
	if _, err := store.Init(true); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
}

func writeCLIDatabaseSecret(t *testing.T, root string, env secrets.Environment, value string) {
	t.Helper()

	store := secrets.NewStore(root)
	if _, err := store.Init(true); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(env, "DATABASE_URL", value); err != nil {
		t.Fatalf("Set(DATABASE_URL) error = %v", err)
	}
}

func noopServeRunner(context.Context, string) error {
	return nil
}

func noopDatabaseChecker(context.Context, string) error {
	return nil
}

func noopDatabaseRunner() *fakeDatabaseRunner {
	return &fakeDatabaseRunner{}
}

type fakeDatabaseRunner struct {
	checkErr      error
	createResult  db.DatabaseResult
	createErr     error
	dropResult    db.DatabaseResult
	dropErr       error
	prepareResult db.SchemaSyncResult
	prepareErr    error
	resetResult   db.SchemaSyncResult
	resetErr      error
	schemaDumpErr error
	operation     string
	root          string
	databaseURL   string
	calls         int
}

func (r *fakeDatabaseRunner) Check(_ context.Context, databaseURL string) error {
	r.calls++
	r.operation = "check"
	r.databaseURL = databaseURL
	return r.checkErr
}

func (r *fakeDatabaseRunner) Create(_ context.Context, databaseURL string) (db.DatabaseResult, error) {
	r.calls++
	r.operation = "create"
	r.databaseURL = databaseURL
	return r.createResult, r.createErr
}

func (r *fakeDatabaseRunner) Drop(_ context.Context, databaseURL string) (db.DatabaseResult, error) {
	r.calls++
	r.operation = "drop"
	r.databaseURL = databaseURL
	return r.dropResult, r.dropErr
}

func (r *fakeDatabaseRunner) Prepare(_ context.Context, root string, databaseURL string) (db.SchemaSyncResult, error) {
	r.calls++
	r.operation = "prepare"
	r.root = root
	r.databaseURL = databaseURL
	return r.prepareResult, r.prepareErr
}

func (r *fakeDatabaseRunner) Reset(_ context.Context, root string, databaseURL string) (db.SchemaSyncResult, error) {
	r.calls++
	r.operation = "reset"
	r.root = root
	r.databaseURL = databaseURL
	return r.resetResult, r.resetErr
}

func (r *fakeDatabaseRunner) SchemaDump(_ context.Context, root string, databaseURL string) error {
	r.calls++
	r.operation = "schema-dump"
	r.root = root
	r.databaseURL = databaseURL
	return r.schemaDumpErr
}

type fakeSchemaSyncRunner struct {
	result      db.SchemaSyncResult
	err         error
	root        string
	databaseURL string
	calls       int
}

func (r *fakeSchemaSyncRunner) Sync(_ context.Context, root string, databaseURL string) (db.SchemaSyncResult, error) {
	r.calls++
	r.root = root
	r.databaseURL = databaseURL
	return r.result, r.err
}

func writeCLIEntity(t *testing.T, path string, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func writeEditorScript(t *testing.T, root string, body string) string {
	t.Helper()

	path := filepath.Join(root, "editor.sh")
	script := "#!/bin/sh\nset -eu\n" + strings.TrimSpace(body) + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(editor) error = %v", err)
	}
	return path
}

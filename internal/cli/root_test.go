package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/auth"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/fixtures"
	recordhooks "github.com/hapyco/dygo/internal/hooks"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/hapyco/dygo/internal/server"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/studio"
	"github.com/hapyco/dygo/pkg/sdk"
	"github.com/jackc/pgx/v5"
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
			args:       []string{"app", "list"},
			wantStdout: "No apps found.",
		},
		{
			name:       "validates empty app set",
			args:       []string{"app", "validate"},
			wantStdout: "0 apps are valid",
		},
		{
			name:       "validates empty entity set",
			args:       []string{"entity", "validate"},
			wantStdout: "0 entities are valid",
		},
		{
			name:       "prints no apps message for entity list",
			args:       []string{"entity", "list"},
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

func TestCommandSurfaceRegistersTargetCommands(t *testing.T) {
	commands := [][]string{
		{"new"},
		{"upgrade"},
		{"version"},
		{"completion"},
		{"doctor"},
		{"setup"},
		{"dev"},
		{"serve"},
		{"db"},
		{"db", "check"},
		{"db", "create"},
		{"db", "drop"},
		{"db", "migrate"},
		{"db", "prune"},
		{"db", "reset"},
		{"app"},
		{"app", "list"},
		{"app", "validate"},
		{"entity"},
		{"entity", "list"},
		{"entity", "validate"},
		{"entity", "show"},
		{"entity", "graph"},
		{"fixture"},
		{"fixture", "apply"},
		{"fixture", "validate"},
		{"fixture", "export"},
		{"hook"},
		{"hook", "list"},
		{"hook", "validate"},
		{"hook", "sync"},
		{"generate"},
		{"generate", "app"},
		{"generate", "entity"},
		{"generate", "collection"},
		{"generate", "hook"},
		{"generate", "fixture"},
		{"generate", "test"},
		{"g"},
		{"route"},
		{"route", "list"},
		{"route", "validate"},
		{"route", "resolve"},
		{"route", "reserved"},
		{"permission"},
		{"permission", "list"},
		{"permission", "check"},
		{"permission", "explain"},
		{"secret"},
		{"secret", "init"},
		{"secret", "get"},
		{"secret", "edit"},
		{"secret", "validate"},
		{"secret", "rotate-key"},
	}

	for _, command := range commands {
		command := command
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			root := t.TempDir()
			writeCLIProjectRoot(t, root)
			writeCLIConfig(t, root)
			t.Chdir(root)

			args := append([]string{}, command...)
			args = append(args, "--help")

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if err := Run(context.Background(), args, strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatalf("Run(%v) error = %v, want help success", args, err)
			}
			if !strings.Contains(stdout.String(), "Usage:") {
				t.Fatalf("Run(%v) stdout = %q, want help usage", args, stdout.String())
			}
		})
	}
}

func TestCommandSurfaceRejectsRemovedPublicPaths(t *testing.T) {
	commands := [][]string{
		{"apps"},
		{"entities"},
		{"fixtures"},
		{"hooks"},
		{"secrets"},
		{"migrate"},
		{"migrate", "plan"},
		{"patches"},
		{"patch"},
		{"schema"},
		{"schema", "prune"},
		{"db", "prepare"},
		{"db", "schema"},
		{"db", "schema", "dump"},
		{"db", "schema", "check"},
		{"setup", "admin"},
		{"upgrade", "--cli-only"},
		{"upgrade", "--project-only"},
		{"upgrade", "--install-dir", "/tmp/dygo"},
		{"serve", "--studio-dev-url", "http://127.0.0.1:6791"},
	}

	for _, command := range commands {
		command := command
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			root := t.TempDir()
			writeCLIProjectRoot(t, root)
			writeCLIConfig(t, root)
			t.Chdir(root)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if err := Run(context.Background(), command, strings.NewReader(""), &stdout, &stderr); err == nil {
				t.Fatalf("Run(%v) error = nil, want removed command failure", command)
			}
		})
	}
}

func TestSetupCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	fake := &fakeAdminSetupRunner{
		user: auth.User{ID: 7, Email: "admin@example.com", FullName: "Admin User", Enabled: true, Administrator: true},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetup(context.Background(), []string{"setup", "--email", "admin@example.com", "--full-name", "Admin User", "--password-stdin"}, strings.NewReader("secret\n"), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, fake)
	if err != nil {
		t.Fatalf("Run(setup) error = %v, want nil", err)
	}
	if stdout.String() != "administrator account ready: admin@example.com (development)\n" {
		t.Fatalf("setup stdout = %q, want ready output", stdout.String())
	}
	if fake.databaseURL != databaseURL {
		t.Fatalf("setup database URL = %q, want %q", fake.databaseURL, databaseURL)
	}
	if fake.input.Email != "admin@example.com" || fake.input.FullName != "Admin User" || fake.input.Password != "secret" {
		t.Fatalf("setup input = %+v, want flag/stdin values", fake.input)
	}
}

func TestSetupCommandUsesSelectedEnvironment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://staging-user:secret-password@localhost:5432/dygo_staging"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentStaging, databaseURL)
	t.Chdir(root)

	fake := &fakeAdminSetupRunner{
		user: auth.User{ID: 7, Email: "admin@example.com", FullName: "Admin User", Enabled: true, Administrator: true},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetup(context.Background(), []string{"setup", "--env", "staging", "--email", "admin@example.com", "--full-name", "Admin User", "--password-stdin"}, strings.NewReader("secret\n"), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, fake)
	if err != nil {
		t.Fatalf("Run(setup --env staging) error = %v, want nil", err)
	}
	if stdout.String() != "administrator account ready: admin@example.com (staging)\n" {
		t.Fatalf("setup stdout = %q, want staging output", stdout.String())
	}
	if fake.databaseURL != databaseURL {
		t.Fatalf("setup database URL = %q, want %q", fake.databaseURL, databaseURL)
	}
}

func TestSetupCommandPromptsForMissingValues(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeAdminSetupRunner{
		user: auth.User{ID: 7, Email: "admin@example.com", FullName: "Admin User", Enabled: true, Administrator: true},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetup(context.Background(), []string{"setup"}, strings.NewReader("admin@example.com\nAdmin User\nsecret\n"), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, fake)
	if err != nil {
		t.Fatalf("Run(setup prompts) error = %v, want nil", err)
	}
	for _, want := range []string{"Admin email:", "Admin full name:", "Admin password:"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("setup stderr = %q, want prompt %q", stderr.String(), want)
		}
	}
	if fake.input.Password != "secret" {
		t.Fatalf("setup password = %q, want stdin password", fake.input.Password)
	}
}

func TestSetupCommandRequiresDatabaseSecret(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLISecretsLayout(t, root)
	t.Chdir(root)

	fake := &fakeAdminSetupRunner{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetup(context.Background(), []string{"setup", "--email", "admin@example.com", "--full-name", "Admin User", "--password-stdin"}, strings.NewReader("secret\n"), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, fake)
	if err == nil {
		t.Fatal("Run(setup) error = nil, want missing secret error")
	}
	for _, want := range []string{`read database secret "DATABASE_URL" for development`, `secret "DATABASE_URL" is not defined`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(setup) error = %q, want substring %q", err.Error(), want)
		}
	}
	if fake.calls != 0 {
		t.Fatalf("setup runner calls = %d, want 0", fake.calls)
	}
}

func TestSetupCommandReturnsRunnerError(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeAdminSetupRunner{err: auth.Error{Code: auth.ErrorSchemaNotReady, Message: "auth schema is not ready; run dygo db migrate"}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetup(context.Background(), []string{"setup", "--email", "admin@example.com", "--full-name", "Admin User", "--password-stdin"}, strings.NewReader("secret\n"), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, fake)
	if err == nil {
		t.Fatal("Run(setup) error = nil, want runner error")
	}
	for _, want := range []string{"setup administrator account", "auth schema is not ready"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(setup) error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestFixtureApplyCommandDefaultsToDevelopment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	fake := &fakeFixtureRunner{
		plan:   fixturePlan(2, 3),
		result: fixtures.Result{Created: 3, Updated: 2},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"fixture", "apply", "--yes"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, &fakeAdminSetupRunner{}, fake)
	if err != nil {
		t.Fatalf("Run(fixture apply) error = %v, want nil", err)
	}
	wantStdout := "fixture apply plan (development)\nfiles: 2\nrecords: 3\nfixtures applied: 3 created, 2 updated (development)\n"
	if stdout.String() != wantStdout {
		t.Fatalf("fixture apply stdout = %q, want %q", stdout.String(), wantStdout)
	}
	if stderr.String() != "" {
		t.Fatalf("fixture apply stderr = %q, want empty", stderr.String())
	}
	if fake.root != root || fake.databaseURL != databaseURL {
		t.Fatalf("fixture runner root/url = %q/%q, want %q/%q", fake.root, fake.databaseURL, root, databaseURL)
	}
	if fake.planCalls != 1 || fake.calls != 1 {
		t.Fatalf("fixture runner plan/apply calls = %d/%d, want 1/1", fake.planCalls, fake.calls)
	}
}

func TestFixtureApplyCommandUsesSelectedEnvironment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://staging-user:secret-password@localhost:5432/dygo_staging"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentStaging, databaseURL)
	t.Chdir(root)

	fake := &fakeFixtureRunner{plan: fixturePlan(1, 1)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"fixture", "apply", "--env", "staging"}, strings.NewReader("yes\n"), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, &fakeAdminSetupRunner{}, fake)
	if err != nil {
		t.Fatalf("Run(fixture apply --env staging) error = %v, want nil", err)
	}
	wantStdout := "fixture apply plan (staging)\nfiles: 1\nrecords: 1\nfixtures applied: 0 created, 0 updated (staging)\n"
	if stdout.String() != wantStdout {
		t.Fatalf("fixture apply stdout = %q, want %q", stdout.String(), wantStdout)
	}
	if stderr.String() != "Apply fixture records? [y/N] " {
		t.Fatalf("fixture apply stderr = %q, want prompt", stderr.String())
	}
	if fake.databaseURL != databaseURL {
		t.Fatalf("fixture runner URL = %q, want staging URL %q", fake.databaseURL, databaseURL)
	}
}

func TestFixtureApplyDryRunDoesNotRequireDatabaseSecret(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLISecretsLayout(t, root)
	t.Chdir(root)

	fake := &fakeFixtureRunner{plan: fixturePlan(1, 2)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"fixture", "apply", "--dry-run"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, &fakeAdminSetupRunner{}, fake)
	if err != nil {
		t.Fatalf("Run(fixture apply --dry-run) error = %v, want nil", err)
	}
	wantStdout := "fixture apply plan (development)\nfiles: 1\nrecords: 2\ndry-run: no records will be written\n"
	if stdout.String() != wantStdout {
		t.Fatalf("fixture apply stdout = %q, want %q", stdout.String(), wantStdout)
	}
	if fake.calls != 0 {
		t.Fatalf("fixture runner calls = %d, want 0", fake.calls)
	}
}

func TestFixtureApplyCommandRequiresDatabaseSecretAfterConfirmation(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLISecretsLayout(t, root)
	t.Chdir(root)

	fake := &fakeFixtureRunner{plan: fixturePlan(1, 1)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"fixture", "apply", "--yes"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, &fakeAdminSetupRunner{}, fake)
	if err == nil {
		t.Fatal("Run(fixture apply --yes) error = nil, want missing secret error")
	}
	for _, want := range []string{`read database secret "DATABASE_URL" for development`, `secret "DATABASE_URL" is not defined`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(fixture apply --yes) error = %q, want substring %q", err.Error(), want)
		}
	}
	if fake.calls != 0 {
		t.Fatalf("fixture runner apply calls = %d, want 0", fake.calls)
	}
}

func TestFixtureApplyCommandReturnsRunnerError(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fake := &fakeFixtureRunner{plan: fixturePlan(1, 1), err: errors.New("invalid fixtures")}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"fixture", "apply", "--yes"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, &fakeAdminSetupRunner{}, fake)
	if err == nil {
		t.Fatal("Run(fixture apply) error = nil, want runner error")
	}
	if !strings.Contains(err.Error(), "apply fixture records: invalid fixtures") {
		t.Fatalf("Run(fixture apply) error = %q, want apply context", err.Error())
	}
}

func TestFixtureExportDryRunPrintsPlanWithoutWriting(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	fake := &fakeFixtureRunner{
		exportPlan: fixtures.ExportPlan{
			Files: []fixtures.ExportFile{{
				ProjectPath: "apps/crm/entities/lead/fixtures.yml",
				Records:     []db.Record{{"name": "lead-one"}, {"name": "lead-two"}},
			}},
			UnresolvedLinks: []fixtures.ExportLink{{
				SourceApp:    "crm",
				SourceEntity: "lead",
				SourceRecord: "lead-one",
				Field:        "owner",
				TargetApp:    "core",
				TargetEntity: "user",
				TargetRecord: "admin",
				Reason:       "target record is not included in this export",
			}},
		},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"fixture", "export", "crm/lead", "--dry-run"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, &fakeAdminSetupRunner{}, fake)
	if err != nil {
		t.Fatalf("Run(fixture export --dry-run) error = %v, want nil", err)
	}
	wantStdout := "fixture export plan (development)\nfiles: 1\nrecords: 2\nunresolved links: 1\nfile: apps/crm/entities/lead/fixtures.yml (2 records)\nunresolved link: crm/lead \"lead-one\" field \"owner\" -> core/user \"admin\" (target record is not included in this export)\ndry-run: no fixture files will be written\n"
	if stdout.String() != wantStdout {
		t.Fatalf("fixture export stdout = %q, want %q", stdout.String(), wantStdout)
	}
	if stderr.String() != "" {
		t.Fatalf("fixture export stderr = %q, want empty", stderr.String())
	}
	if fake.exportPlanCalls != 1 || fake.exportCalls != 0 {
		t.Fatalf("fixture export calls = plan %d write %d, want 1/0", fake.exportPlanCalls, fake.exportCalls)
	}
	if fake.root != root || fake.databaseURL != databaseURL || fake.exportTarget != (shape.AppRef{App: "crm", Name: "lead"}) {
		t.Fatalf("fixture export inputs = root %q url %q target %+v, want %q %q crm/lead", fake.root, fake.databaseURL, fake.exportTarget, root, databaseURL)
	}
}

func TestFixtureExportYesWritesPlannedFiles(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://staging-user:secret-password@localhost:5432/dygo_staging"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentStaging, databaseURL)
	t.Chdir(root)

	fake := &fakeFixtureRunner{
		exportPlan: fixtures.ExportPlan{
			Files: []fixtures.ExportFile{{
				ProjectPath: "apps/crm/entities/lead/fixtures.yml",
				Records:     []db.Record{{"name": "lead-one"}},
			}},
		},
		exportResult: fixtures.ExportResult{FilesWritten: 1, RecordsWritten: 1},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"fixture", "export", "crm/lead", "--env", "staging", "--yes", "--include-links"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, &fakeAdminSetupRunner{}, fake)
	if err != nil {
		t.Fatalf("Run(fixture export --yes) error = %v, want nil", err)
	}
	wantStdout := "fixture export plan (staging)\nfiles: 1\nrecords: 1\nunresolved links: 0\nfile: apps/crm/entities/lead/fixtures.yml (1 records)\nfixtures exported: 1 files, 1 records (staging)\n"
	if stdout.String() != wantStdout {
		t.Fatalf("fixture export stdout = %q, want %q", stdout.String(), wantStdout)
	}
	if stderr.String() != "" {
		t.Fatalf("fixture export stderr = %q, want empty", stderr.String())
	}
	if fake.exportPlanCalls != 1 || fake.exportCalls != 1 {
		t.Fatalf("fixture export calls = plan %d write %d, want 1/1", fake.exportPlanCalls, fake.exportCalls)
	}
	if !fake.includeLinks {
		t.Fatal("fixture export includeLinks = false, want true")
	}
	if fake.databaseURL != databaseURL {
		t.Fatalf("fixture export database URL = %q, want %q", fake.databaseURL, databaseURL)
	}
}

func TestFixtureValidateCommandPlansFixtures(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	fake := &fakeFixtureRunner{plan: fixturePlan(2, 5)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"fixture", "validate"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, &fakeAdminSetupRunner{}, fake)
	if err != nil {
		t.Fatalf("Run(fixture validate) error = %v, want nil", err)
	}
	if stdout.String() != "fixtures valid: 2 files, 5 records\n" {
		t.Fatalf("fixture validate stdout = %q, want valid output", stdout.String())
	}
	if fake.planCalls != 1 || fake.calls != 0 {
		t.Fatalf("fixture runner plan/apply calls = %d/%d, want 1/0", fake.planCalls, fake.calls)
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
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	nested := filepath.Join(root, "apps", "sales")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll(nested) error = %v", err)
	}
	t.Chdir(nested)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotOptions server.Options
	err := run(context.Background(), []string{"serve"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, options server.Options) error {
		gotOptions = options
		if options.OnReady != nil {
			return options.OnReady(options.Address)
		}
		return nil
	}, noopDatabaseChecker)
	if err != nil {
		t.Fatalf("Run(serve) error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "dygo serving on 0.0.0.0:7777") {
		t.Fatalf("serve stdout = %q, want configured address", stdout.String())
	}
	if gotOptions.Address != "0.0.0.0:7777" {
		t.Fatalf("serve address = %q, want configured address", gotOptions.Address)
	}
	if gotOptions.DatabaseURL != databaseURL {
		t.Fatalf("serve database URL = %q, want %q", gotOptions.DatabaseURL, databaseURL)
	}
}

func TestServeCommandUsesEnvironment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://staging-user:secret-password@localhost:5432/dygo_staging"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentStaging, databaseURL)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotOptions server.Options
	err := run(context.Background(), []string{"serve", "--env", "staging"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, options server.Options) error {
		gotOptions = options
		if options.OnReady != nil {
			return options.OnReady(options.Address)
		}
		return nil
	}, noopDatabaseChecker)
	if err != nil {
		t.Fatalf("Run(serve --env staging) error = %v, want nil", err)
	}
	if gotOptions.DatabaseURL != databaseURL {
		t.Fatalf("serve database URL = %q, want staging URL %q", gotOptions.DatabaseURL, databaseURL)
	}
}

func TestRunWithOptionsPassesRecordHooksToServe(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	registrarCalled := false
	var gotOptions server.Options
	err := runWithOptionsForTest(context.Background(), []string{"serve"}, strings.NewReader(""), io.Discard, io.Discard, func(_ context.Context, options server.Options) error {
		gotOptions = options
		return nil
	}, Options{
		RecordHooks: []sdk.RecordHookRegistrar{
			func(registry sdk.RecordHookRegistry) error {
				registrarCalled = true
				return registry.RegisterEntity("sales", "lead", sdk.RecordBeforeCreate, "test", func(context.Context, sdk.RecordHook) error {
					return nil
				})
			},
		},
	})
	if err != nil {
		t.Fatalf("runWithOptionsForTest(serve) error = %v, want nil", err)
	}
	if !registrarCalled {
		t.Fatal("registrar was not called")
	}
	if gotOptions.RecordHooks == nil {
		t.Fatal("serve RecordHooks = nil, want configured registry")
	}
}

func TestDevCommandConfiguresStudioDevProxy(t *testing.T) {
	previousStarter := startStudioDevServer
	startStudioDevServer = func(context.Context, string, io.Writer, io.Writer) (string, studioDevStop, error) {
		return "", nil, errors.New("auto Studio starter should not run when --studio-dev-url is provided")
	}
	defer func() {
		startStudioDevServer = previousStarter
	}()

	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotOptions server.Options
	err := run(context.Background(), []string{"dev", "--studio-dev-url", "http://127.0.0.1:6791"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, options server.Options) error {
		gotOptions = options
		if options.OnReady != nil {
			return options.OnReady(options.Address)
		}
		return nil
	}, noopDatabaseChecker)
	if err != nil {
		t.Fatalf("Run(dev --studio-dev-url) error = %v, want nil", err)
	}
	if gotOptions.Studio == nil {
		t.Fatal("dev Studio handler = nil, want dev proxy handler")
	}
}

func TestDevCommandAutoStartsStudioDevServer(t *testing.T) {
	previousStarter := startStudioDevServer
	started := 0
	stopped := 0
	gotRoot := ""
	startStudioDevServer = func(_ context.Context, root string, _ io.Writer, _ io.Writer) (string, studioDevStop, error) {
		started++
		gotRoot = root
		return "http://127.0.0.1:6791", func() error {
			stopped++
			return nil
		}, nil
	}
	defer func() {
		startStudioDevServer = previousStarter
	}()

	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotOptions server.Options
	err := run(context.Background(), []string{"dev"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, options server.Options) error {
		gotOptions = options
		if options.OnReady != nil {
			return options.OnReady(options.Address)
		}
		return nil
	}, noopDatabaseChecker)
	if err != nil {
		t.Fatalf("Run(dev) error = %v, want nil", err)
	}
	if started != 1 {
		t.Fatalf("Studio starter calls = %d, want 1", started)
	}
	if stopped != 1 {
		t.Fatalf("Studio stop calls = %d, want 1", stopped)
	}
	if gotRoot != root {
		t.Fatalf("Studio starter root = %q, want %q", gotRoot, root)
	}
	if gotOptions.Studio == nil {
		t.Fatal("dev Studio handler = nil, want auto dev proxy handler")
	}
	if !strings.Contains(stdout.String(), "dygo dev serving on 127.0.0.1:6790") {
		t.Fatalf("dev stdout = %q, want ready output", stdout.String())
	}
}

func TestServeCommandRequiresProjectConfig(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	called := false
	err := run(context.Background(), []string{"serve"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, _ server.Options) error {
		called = true
		return nil
	}, noopDatabaseChecker)
	if err == nil {
		t.Fatal("Run(serve) error = nil, want missing config error")
	}
	for _, want := range []string{"load config", "dygo.yml"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(serve) error = %q, want substring %q", err.Error(), want)
		}
	}
	if called {
		t.Fatal("serve runner was called for missing config")
	}
}

func TestServeCommandRequiresDatabaseSecret(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLISecretsLayout(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	called := false
	err := run(context.Background(), []string{"serve"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, _ server.Options) error {
		called = true
		return nil
	}, noopDatabaseChecker)
	if err == nil {
		t.Fatal("Run(serve) error = nil, want missing secret error")
	}
	for _, want := range []string{`read database secret "DATABASE_URL" for development`, `secret "DATABASE_URL" is not defined`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(serve) error = %q, want substring %q", err.Error(), want)
		}
	}
	if called {
		t.Fatal("serve runner was called for missing secret")
	}
}

func TestServeCommandReturnsRunnerError(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"serve"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, _ server.Options) error {
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
	if strings.Contains(stdout.String(), "dygo serving on") {
		t.Fatalf("serve stdout = %q, want no ready message when runner fails before startup", stdout.String())
	}
}

func TestServeCommandFailsWhenStudioAssetsAreUnavailable(t *testing.T) {
	restoreEmbedded := studio.SetEmbeddedSourceForTest(func() (studio.Source, bool, error) {
		return studio.Source{}, false, nil
	})
	defer restoreEmbedded()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "dygo.yml"), []byte("name: test\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(dygo.yml) error = %v", err)
	}
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	called := false
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := run(context.Background(), []string{"serve"}, strings.NewReader(""), &stdout, &stderr, func(_ context.Context, _ server.Options) error {
		called = true
		return nil
	}, noopDatabaseChecker)
	if err == nil {
		t.Fatal("Run(serve) error = nil, want missing Studio assets error")
	}
	for _, want := range []string{"resolve Studio UI", "Studio UI assets are unavailable", ".dygo/apps/studio/ui/dist", "dygo dev"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(serve) error = %q, want substring %q", err.Error(), want)
		}
	}
	if called {
		t.Fatal("serve runner was called without Studio assets")
	}
	if strings.Contains(stdout.String(), "dygo serving on") {
		t.Fatalf("serve stdout = %q, want no ready message", stdout.String())
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
	if _, err := store.Init(); err != nil {
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

func TestDBDropCommandPromptsBeforeDrop(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	fake := &fakeDatabaseRunner{dropResult: db.DatabaseResult{Name: "dygo", Changed: true}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "drop"}, strings.NewReader("\n"), &stdout, &stderr, noopServeRunner, fake, &fakeSchemaSyncRunner{})
	if err != nil {
		t.Fatalf("Run(db drop cancel) error = %v, want nil", err)
	}
	if fake.calls != 0 {
		t.Fatalf("database runner calls = %d, want 0 after declined prompt", fake.calls)
	}
	for _, want := range []string{"db drop plan (development)", "database: dygo", "database drop cancelled"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("db drop stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if !strings.Contains(stderr.String(), "Drop database? [y/N] ") {
		t.Fatalf("db drop stderr = %q, want prompt", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	err = runWithServices(context.Background(), []string{"db", "drop", "--yes"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, fake, &fakeSchemaSyncRunner{})
	if err != nil {
		t.Fatalf("Run(db drop --yes) error = %v, want nil", err)
	}
	if fake.operation != "drop" || fake.databaseURL != databaseURL {
		t.Fatalf("database runner = operation %q URL %q, want drop/%q", fake.operation, fake.databaseURL, databaseURL)
	}
	if !strings.Contains(stdout.String(), "database dropped: dygo (development)") {
		t.Fatalf("db drop stdout = %q, want applied output", stdout.String())
	}
}

func TestDBMigrateDryRunPlansFullWorkflow(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	fakeSync := &fakeSchemaSyncRunner{
		patchPlan: db.PatchPlan{Pending: []db.PlannedPatch{{AppName: "sales", PatchID: "001"}}},
		plan:      db.SchemaPlan{Operations: []db.SchemaOperation{{Description: "create table"}}},
	}
	fakeFixture := &fakeFixtureRunner{plan: fixturePlan(2, 5)}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"db", "migrate", "--dry-run"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), fakeSync, &fakeAdminSetupRunner{}, fakeFixture)
	if err != nil {
		t.Fatalf("Run(db migrate --dry-run) error = %v, want nil", err)
	}
	for _, want := range []string{"db migrate plan (development)", "pre-sync patches: 1 pending", "schema safe operations: 1", "fixtures: 2 files, 5 records"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("db migrate dry-run stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if fakeSync.patchPlanCalls != 2 || fakeSync.planCalls != 1 || fakeSync.calls != 0 || fakeFixture.planCalls != 1 || fakeFixture.calls != 0 {
		t.Fatalf("plan/apply calls = patchPlan %d plan %d sync %d fixturePlan %d fixtureApply %d, want dry-run only", fakeSync.patchPlanCalls, fakeSync.planCalls, fakeSync.calls, fakeFixture.planCalls, fakeFixture.calls)
	}
}

func TestDBMigrateYesAppliesFullWorkflow(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, databaseURL)
	t.Chdir(root)

	fakeSync := &fakeSchemaSyncRunner{
		patchApplyResult: db.PatchApplyResult{Applied: []db.PatchRun{{AppName: "sales", PatchID: "001"}}},
		result:           db.SchemaSyncResult{Apps: 2, Entities: 8, Fields: 34, Operations: 3},
	}
	fakeFixture := &fakeFixtureRunner{
		plan:   fixturePlan(1, 2),
		result: fixtures.Result{Created: 1, Updated: 1},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"db", "migrate", "--yes"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), fakeSync, &fakeAdminSetupRunner{}, fakeFixture)
	if err != nil {
		t.Fatalf("Run(db migrate --yes) error = %v, want nil", err)
	}
	for _, want := range []string{"db migrate plan (development)", "database migrated (development)", "metadata synced: 2 apps, 8 entities, 34 fields, 3 schema operations", "fixture records: 1 created, 1 updated", "schema snapshot: refreshed"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("db migrate stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if fakeSync.patchApplyCalls != 2 || fakeSync.calls != 1 || fakeFixture.calls != 1 {
		t.Fatalf("apply calls = patch %d sync %d fixture %d, want full workflow", fakeSync.patchApplyCalls, fakeSync.calls, fakeFixture.calls)
	}
}

func TestDBResetDryRunPrintsSteps(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fakeDB := &fakeDatabaseRunner{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServicesAndSetupAndFixtures(context.Background(), []string{"db", "reset", "--dry-run"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, fakeDB, &fakeSchemaSyncRunner{}, &fakeAdminSetupRunner{}, &fakeFixtureRunner{})
	if err != nil {
		t.Fatalf("Run(db reset --dry-run) error = %v, want nil", err)
	}
	for _, want := range []string{"db reset plan (development)", "database: dygo", "steps: drop, create, migrate"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("db reset dry-run stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if fakeDB.calls != 0 {
		t.Fatalf("database runner calls = %d, want dry-run only", fakeDB.calls)
	}
}

func TestDBPruneDryRunPlansSchemaCleanup(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fakeSync := &fakeSchemaSyncRunner{prunePlan: db.SchemaPrunePlan{Operations: []db.SchemaPruneOperation{{Description: "drop column"}}}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "prune", "--dry-run"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), fakeSync)
	if err != nil {
		t.Fatalf("Run(db prune --dry-run) error = %v, want nil", err)
	}
	for _, want := range []string{"schema prune plan (development)", "destructive operations: 1", "rerun with --yes to apply"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("db prune dry-run stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if fakeSync.prunePlanCalls != 1 || fakeSync.pruneCalls != 0 {
		t.Fatalf("prune calls = plan %d apply %d, want dry-run only", fakeSync.prunePlanCalls, fakeSync.pruneCalls)
	}
}

func TestDBPruneYesAppliesSchemaCleanup(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	t.Chdir(root)

	fakeSync := &fakeSchemaSyncRunner{
		prunePlan:   db.SchemaPrunePlan{Operations: []db.SchemaPruneOperation{{Description: "drop column"}}},
		pruneResult: db.SchemaPruneResult{Operations: 1},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithServices(context.Background(), []string{"db", "prune", "--yes"}, strings.NewReader(""), &stdout, &stderr, noopServeRunner, noopDatabaseRunner(), fakeSync)
	if err != nil {
		t.Fatalf("Run(db prune --yes) error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "schema pruned: 1 destructive operation (development)") {
		t.Fatalf("db prune stdout = %q, want applied output", stdout.String())
	}
	if fakeSync.prunePlanCalls != 1 || fakeSync.pruneCalls != 1 {
		t.Fatalf("prune calls = plan %d apply %d, want plan and apply", fakeSync.prunePlanCalls, fakeSync.pruneCalls)
	}
}

func TestAppListCommand(t *testing.T) {
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
	err := Run(context.Background(), []string{"app", "list"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(app list) error = %v, want nil", err)
	}

	output := stdout.String()
	for _, want := range []string{"NAME", "VERSION", "LABEL", "core", "Core", "sales", "Sales", "0.1.0"} {
		if !strings.Contains(output, want) {
			t.Fatalf("app list stdout = %q, want substring %q", output, want)
		}
	}
}

func TestAppValidateCommand(t *testing.T) {
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
	err := Run(context.Background(), []string{"app", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(app validate) error = %v, want nil", err)
	}
	if stdout.String() != "2 apps are valid\n" {
		t.Fatalf("app validate stdout = %q, want success count", stdout.String())
	}
}

func TestAppValidateCommandRejectsInvalidAppSet(t *testing.T) {
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
	err := Run(context.Background(), []string{"app", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(app validate) error = nil, want missing dependency error")
	}
	if !strings.Contains(err.Error(), "unknown app") {
		t.Fatalf("Run(app validate) error = %q, want unknown app", err.Error())
	}
}

func TestAppValidateCommandRejectsMissingProjectRoot(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"app", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(app validate) error = nil, want missing project root error")
	}
	if !strings.Contains(err.Error(), "no dygo project root found") {
		t.Fatalf("Run(app validate) error = %q, want missing project root", err.Error())
	}
}

func TestDoctorCommand(t *testing.T) {
	withDoctorRuntimePool(t, &fakeDoctorRuntimePool{
		roleCount:       2,
		permissionCount: 17,
		adminExists:     true,
	})

	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, cliEntityPath(root, "sales", "company"), `
label: Company
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
		"PASS route registry: 1 routeable entities",
		"PASS fixture files: 0 files, 0 records valid",
		"SKIP hook wiring: go.mod not found",
		"PASS schema snapshot: db/schema.sql present",
		"PASS Studio assets: project Studio cache available",
		"PASS config: dygo.yml server=127.0.0.1:6790",
		"PASS secrets layout: 3 environments configured",
		"PASS runtime database: development database reachable",
		"PASS core fixtures: 2 roles and 17 permissions ready",
		"PASS administrator account: Administrator account exists",
		"dygo doctor passed",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor stdout = %q, want substring %q", output, want)
		}
	}
}

func TestDoctorCommandReportsMissingSchemaSnapshot(t *testing.T) {
	withDoctorRuntimePool(t, &fakeDoctorRuntimePool{
		roleCount:       2,
		permissionCount: 17,
		adminExists:     true,
	})

	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	if err := os.Remove(filepath.Join(root, "db", "schema.sql")); err != nil {
		t.Fatalf("Remove(db/schema.sql) error = %v", err)
	}
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"doctor"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(doctor) error = nil, want missing schema snapshot diagnostic")
	}

	output := stdout.String()
	for _, want := range []string{
		"FAIL schema snapshot: missing db/schema.sql; run dygo db migrate",
		"PASS Studio assets: project Studio cache available",
		"dygo doctor found 1 problem",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor stdout = %q, want substring %q", output, want)
		}
	}
}

func TestDoctorCommandReportsMissingFirstRunSetup(t *testing.T) {
	withDoctorRuntimePool(t, &fakeDoctorRuntimePool{})

	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"doctor"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(doctor) error = nil, want first-run setup diagnostics")
	}

	output := stdout.String()
	for _, want := range []string{
		"PASS runtime database: development database reachable",
		"FAIL core fixtures: missing Core roles and permissions; run dygo fixture apply",
		"FAIL administrator account: missing Administrator account; run dygo setup",
		"dygo doctor found 2 problems",
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
		"dygo.yml",
		"FAIL secrets layout:",
		"SKIP runtime database:",
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

func TestEntityValidateCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, cliEntityPath(root, "sales", "company"), `
label: Company
fields:
  - name: title
    label: Title
    type: text
`)
	writeCLIEntity(t, cliEntityPath(root, "sales", "lead"), `
label: Lead
fields:
  - name: company
    label: Company
    type: link
    options:
      entity: company
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"entity", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(entity validate) error = %v, want nil", err)
	}
	if stdout.String() != "2 entities are valid\n" {
		t.Fatalf("entity validate stdout = %q, want success count", stdout.String())
	}
}

func TestEntityListCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)

	writeCLIApp(t, filepath.Join(root, "apps", "core"), "core")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, cliEntityPath(root, "sales", "company"), `
label: Company
fields:
  - name: title
    label: Title
    type: text
`)
	writeCLIEntity(t, cliEntityPath(root, "sales", "lead"), `
label: Lead
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
	err := Run(context.Background(), []string{"entity", "list"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(entity list) error = %v, want nil", err)
	}

	want := "core\n  (no entities)\nsales\n  - company\n  - lead\n"
	if stdout.String() != want {
		t.Fatalf("entity list stdout = %q, want %q", stdout.String(), want)
	}
}

func TestEntityShowCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)

	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "company", "entity.yml"), `
label: Company
fields:
  - name: title
    label: Title
    type: text
`)
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "lead", "entity.yml"), `
label: Lead
fields:
  - name: company
    label: Company
    type: link
    options:
      entity: company
  - name: notes
    label: Notes
    type: text
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"entity", "show", "sales/lead"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(entity show sales/lead) error = %v, want nil", err)
	}
	want := "entity: sales/lead\nkind: normal\npath: apps/sales/entities/lead/entity.yml\nroute: /lead\nnaming: random\nfields:\n  - company: link -> sales/company\n  - notes: text\n"
	if stdout.String() != want {
		t.Fatalf("entity show stdout = %q, want %q", stdout.String(), want)
	}
}

func TestEntityGraphCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)

	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "company", "entity.yml"), `
label: Company
fields:
  - name: title
    label: Title
    type: text
`)
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "lead", "entity.yml"), `
label: Lead
fields:
  - name: company
    label: Company
    type: link
    options:
      entity: company
  - name: contacts
    label: Contacts
    type: collection
    options:
      entity: lead-contact
`)
	writeCLIEntity(t, filepath.Join(root, "apps", "sales", "entities", "_collections", "lead-contact.yml"), `
label: Lead Contact
fields:
  - name: title
    label: Title
    type: text
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"entity", "graph", "sales/lead"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(entity graph sales/lead) error = %v, want nil", err)
	}
	want := "sales/lead (normal)\n  -> link company -> sales/company\n  -> collection contacts -> sales/lead-contact\n"
	if stdout.String() != want {
		t.Fatalf("entity graph stdout = %q, want %q", stdout.String(), want)
	}
}

func TestEntityValidateCommandRejectsInvalidTargets(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	entityPath := cliEntityPath(root, "sales", "lead")
	writeCLIEntity(t, entityPath, `
label: Lead
fields:
  - name: company
    label: Company
    type: link
    options:
      entity: company
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"entity", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(entity validate) error = nil, want missing target error")
	}
	wantPath := filepath.ToSlash(filepath.Join("apps", "sales", "entities", "lead", "entity.yml")) + ":5"
	for _, want := range []string{wantPath, `field "company"`, `unknown entity target "company"`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(entity validate) error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestEntityValidateCommandRejectsDuplicateRouteSlugsAcrossApps(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIApp(t, filepath.Join(root, "apps", "support"), "support")
	writeCLIEntity(t, cliEntityPath(root, "sales", "customer"), `
label: Customer
fields:
  - name: title
    label: Title
    type: text
`)
	duplicatePath := cliEntityPath(root, "support", "customer")
	writeCLIEntity(t, duplicatePath, `
label: Customer
fields:
  - name: title
    label: Title
    type: text
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"entity", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(entity validate) error = nil, want duplicate entity error")
	}
	wantPath := filepath.ToSlash(filepath.Join("apps", "support", "entities", "customer", "entity.yml")) + ":1"
	for _, want := range []string{wantPath, `app "support"`, `entity "customer"`, `route slug "customer" conflicts`, `set route.slug`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(entity validate) error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestEntityValidateCommandRejectsReservedRootSlugs(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	entityPath := cliEntityPath(root, "sales", "login")
	writeCLIEntity(t, entityPath, `
label: Login
fields:
  - name: title
    label: Title
    type: text
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"entity", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(entity validate) error = nil, want reserved slug error")
	}
	wantPath := filepath.ToSlash(filepath.Join("apps", "sales", "entities", "login", "entity.yml")) + ":1"
	for _, want := range []string{wantPath, `app "sales"`, `entity "login"`, `reserved root route slug "login"`, `set route.slug`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Run(entity validate) error = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestDoctorReportsEntityMetadataFailureForInvalidRouteSlug(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	t.Chdir(root)

	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, cliEntityPath(root, "sales", "api"), `
label: API
route:
  slug: BadSlug
fields:
  - name: title
    label: Title
    type: text
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"doctor"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(doctor) error = nil, want entity metadata failure")
	}
	output := stdout.String()
	for _, want := range []string{"FAIL entity metadata:", `route slug "BadSlug" must be kebab-case`, "dygo doctor found"} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor stdout = %q, want substring %q", output, want)
		}
	}
}

func TestSecretsInitCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secret", "init"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secrets init) error = %v, want nil", err)
	}
	output := stdout.String()
	for _, want := range []string{
		"initialized secrets",
		"key: .dygo/secrets/master.key",
		"config/secrets/development.yml.age",
		"config/secrets/staging.yml.age",
		"config/secrets/production.yml.age",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("secrets init stdout = %q, want substring %q", output, want)
		}
	}

	info, err := os.Stat(filepath.Join(root, ".dygo", "secrets", "master.key"))
	if err != nil {
		t.Fatalf("Stat(master.key) error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("master.key mode = %v, want 0600", got)
	}

	stdout.Reset()
	stderr.Reset()
	if err := Run(context.Background(), []string{"secret", "init"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("second secrets init error = %v, want nil", err)
	}
}

func TestSecretsEditDefaultsToDevelopment(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	editor := writeEditorScript(t, root, `
cat > "$1" <<'YAML'
DATABASE_URL: postgres://development
YAML
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secret", "edit", "--editor", editor}, strings.NewReader(""), &stdout, &stderr)
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
	if _, err := store.Init(); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	editor := writeEditorScript(t, root, `
if [ "$1" != "--flag" ]; then
  exit 12
fi
cat > "$2" <<'YAML'
DATABASE_URL: postgres://staging
YAML
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secret", "edit", "--env", "staging", "--editor", editor + " --flag"}, strings.NewReader(""), &stdout, &stderr)
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

func TestSecretsEditUnchangedDoesNotRewrite(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(secrets.EnvironmentDevelopment, "DATABASE_URL", "postgres://development"); err != nil {
		t.Fatalf("Set(development DATABASE_URL) error = %v", err)
	}
	secretFile := store.Paths(secrets.EnvironmentDevelopment).SecretFile
	before, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatalf("ReadFile(secret before edit) error = %v", err)
	}
	editor := writeEditorScript(t, root, `:`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = Run(context.Background(), []string{"secret", "edit", "--editor", editor}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secrets edit unchanged) error = %v, want nil", err)
	}
	if stdout.String() != "development secrets unchanged\n" {
		t.Fatalf("secrets edit stdout = %q, want unchanged message", stdout.String())
	}
	after, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatalf("ReadFile(secret after edit) error = %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("development encrypted secrets changed after unchanged edit")
	}
}

func TestSecretsEditInvalidYAMLDoesNotOverwrite(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(secrets.EnvironmentDevelopment, "DATABASE_URL", "postgres://old"); err != nil {
		t.Fatalf("Set(old DATABASE_URL) error = %v", err)
	}
	editor := writeEditorScript(t, root, `
cat > "$1" <<'YAML'
- bad
YAML
`)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secret", "edit", "--editor", editor}, strings.NewReader("no\n"), &stdout, &stderr)
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
	if _, err := store.Init(); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(secrets.EnvironmentDevelopment, "DATABASE_URL", "postgres://development"); err != nil {
		t.Fatalf("Set(DATABASE_URL) error = %v", err)
	}
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secret", "validate"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secrets validate) error = %v, want nil", err)
	}
	if stdout.String() != "development secrets are valid\n" {
		t.Fatalf("secrets validate stdout = %q, want development success", stdout.String())
	}
}

func TestSecretGetPrintsRawValue(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(secrets.EnvironmentStaging, "database.url", "postgres://staging"); err != nil {
		t.Fatalf("Set(database.url) error = %v", err)
	}
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secret", "get", "database.url", "--env", "staging"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secret get) error = %v, want nil", err)
	}
	if stdout.String() != "postgres://staging\n" {
		t.Fatalf("secret get stdout = %q, want raw secret value", stdout.String())
	}
	if stderr.String() != "" {
		t.Fatalf("secret get stderr = %q, want empty", stderr.String())
	}
}

func TestSecretsRotateKeyCommand(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	store := secrets.NewStore(root)
	if _, err := store.Init(); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(secrets.EnvironmentProduction, "DATABASE_URL", "postgres://production"); err != nil {
		t.Fatalf("Set(production DATABASE_URL) error = %v", err)
	}
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"secret", "rotate-key", "--yes"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secrets rotate-key) error = %v, want nil", err)
	}
	for _, want := range []string{"secret rotate-key plan", "key: .dygo/secrets/master.key", "rotated secrets master key"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("secrets rotate-key stdout = %q, want substring %q", stdout.String(), want)
		}
	}
	if stderr.String() != "" {
		t.Fatalf("secrets rotate-key stderr = %q, want empty for --yes", stderr.String())
	}
	secret, err := store.Get(secrets.EnvironmentProduction, "DATABASE_URL")
	if err != nil {
		t.Fatalf("Get(production DATABASE_URL) error = %v", err)
	}
	if secret.Value != "postgres://production" {
		t.Fatalf("production DATABASE_URL after rotate = %q, want preserved value", secret.Value)
	}
}

func TestSecretsRotateKeyPromptsBeforeRotation(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	store := secrets.NewStore(root)
	paths, err := store.Init()
	if err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(secrets.EnvironmentDevelopment, "DATABASE_URL", "postgres://development"); err != nil {
		t.Fatalf("Set(development DATABASE_URL) error = %v", err)
	}
	before, err := os.ReadFile(paths.MasterKeyFile)
	if err != nil {
		t.Fatalf("ReadFile(master.key) error = %v", err)
	}
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = Run(context.Background(), []string{"secret", "rotate-key"}, strings.NewReader("\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secret rotate-key cancel) error = %v, want nil", err)
	}
	if !strings.Contains(stderr.String(), "Rotate secrets master key? [y/N] ") {
		t.Fatalf("secret rotate-key stderr = %q, want prompt", stderr.String())
	}
	if !strings.Contains(stdout.String(), "secret key rotation cancelled") {
		t.Fatalf("secret rotate-key stdout = %q, want cancellation", stdout.String())
	}
	after, err := os.ReadFile(paths.MasterKeyFile)
	if err != nil {
		t.Fatalf("ReadFile(master.key after cancel) error = %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("master.key changed after cancelled rotation")
	}

	stdout.Reset()
	stderr.Reset()
	err = Run(context.Background(), []string{"secret", "rotate-key"}, strings.NewReader("yes\n"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run(secret rotate-key confirm) error = %v, want nil", err)
	}
	if !strings.Contains(stdout.String(), "rotated secrets master key") {
		t.Fatalf("secret rotate-key stdout = %q, want rotate output", stdout.String())
	}
	secret, err := store.Get(secrets.EnvironmentDevelopment, "DATABASE_URL")
	if err != nil {
		t.Fatalf("Get(development DATABASE_URL) error = %v", err)
	}
	if secret.Value != "postgres://development" {
		t.Fatalf("development DATABASE_URL after rotate = %q, want preserved value", secret.Value)
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
	writeCLISchemaSnapshot(t, root)
	writeCLIStudioCache(t, root)
}

func writeCLISchemaSnapshot(t *testing.T, root string) {
	t.Helper()

	path := filepath.Join(root, "db", "schema.sql")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(db) error = %v", err)
	}
	if err := os.WriteFile(path, []byte("-- test schema snapshot\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(db/schema.sql) error = %v", err)
	}
}

func writeCLIStudioCache(t *testing.T, root string) {
	t.Helper()

	files := map[string]string{
		"index.html":       "<html><body><div id=\"app\"></div><script type=\"module\" src=\"/assets/index.js\"></script></body></html>",
		"assets/index.js":  "console.log('studio')",
		"assets/index.css": "body { margin: 0; }",
	}
	for name, body := range files {
		path := filepath.Join(studio.ProjectCachePath(root), filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
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

	configPath := filepath.Join(root, "dygo.yml")
	configBody := "name: test\n" + strings.TrimSpace(body) + "\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o644); err != nil {
		t.Fatalf("WriteFile(dygo.yml) error = %v", err)
	}
}

func writeCLISecretsLayout(t *testing.T, root string) {
	t.Helper()

	store := secrets.NewStore(root)
	if _, err := store.Init(); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
}

func writeCLIDatabaseSecret(t *testing.T, root string, env secrets.Environment, value string) {
	t.Helper()

	store := secrets.NewStore(root)
	if _, err := store.Init(); err != nil {
		t.Fatalf("Init(secrets) error = %v", err)
	}
	if err := store.Set(env, "DATABASE_URL", value); err != nil {
		t.Fatalf("Set(DATABASE_URL) error = %v", err)
	}
}

func noopServeRunner(context.Context, server.Options) error {
	return nil
}

func noopDatabaseChecker(context.Context, string) error {
	return nil
}

func noopDatabaseRunner() *fakeDatabaseRunner {
	return &fakeDatabaseRunner{}
}

func runWithOptionsForTest(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, serve serveRunner, options Options) error {
	migrator := db.NewMigrator()
	recordHooks, err := recordhooksForTest(options.RecordHooks)
	if err != nil {
		return err
	}
	return runWithServicesAndSetupAndFixturesAndHooks(ctx, args, stdin, stdout, stderr, serve, noopDatabaseRunner(), migrator, &fakeAdminSetupRunner{}, &fakeFixtureRunner{}, &fakePermissionRunner{}, recordHooks)
}

func recordhooksForTest(registrars []sdk.RecordHookRegistrar) (*db.RecordHookRegistry, error) {
	return recordhooks.NewRecordHookRegistry(registrars)
}

func withDoctorRuntimePool(t *testing.T, pool *fakeDoctorRuntimePool) {
	t.Helper()
	previous := openDoctorRuntimePool
	openDoctorRuntimePool = func(_ context.Context, databaseURL string) (doctorRuntimePool, error) {
		pool.opened++
		pool.databaseURL = databaseURL
		if pool.openErr != nil {
			return nil, pool.openErr
		}
		return pool, nil
	}
	t.Cleanup(func() {
		openDoctorRuntimePool = previous
	})
}

type fakeDoctorRuntimePool struct {
	roleCount       int
	permissionCount int
	adminExists     bool
	openErr         error
	roleErr         error
	permissionErr   error
	adminErr        error
	databaseURL     string
	opened          int
	closed          bool
}

func (p *fakeDoctorRuntimePool) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(sql, `FROM "role"`):
		return fakeDoctorRow{value: p.roleCount, err: p.roleErr}
	case strings.Contains(sql, `FROM "permission"`):
		return fakeDoctorRow{value: p.permissionCount, err: p.permissionErr}
	case strings.Contains(sql, `FROM "user"`):
		return fakeDoctorRow{value: p.adminExists, err: p.adminErr}
	default:
		return fakeDoctorRow{err: errors.New("unexpected doctor query")}
	}
}

func (p *fakeDoctorRuntimePool) Close() {
	p.closed = true
}

type fakeDoctorRow struct {
	value any
	err   error
}

func (r fakeDoctorRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("fake doctor row expects one destination")
	}
	switch target := dest[0].(type) {
	case *int:
		value, ok := r.value.(int)
		if !ok {
			return errors.New("fake doctor row value is not int")
		}
		*target = value
	case *bool:
		value, ok := r.value.(bool)
		if !ok {
			return errors.New("fake doctor row value is not bool")
		}
		*target = value
	default:
		return errors.New("unsupported fake doctor row destination")
	}
	return nil
}

type fakeAdminSetupRunner struct {
	user        auth.User
	err         error
	input       auth.SetupAdminInput
	databaseURL string
	calls       int
}

func (r *fakeAdminSetupRunner) SetupAdmin(_ context.Context, databaseURL string, input auth.SetupAdminInput) (auth.User, error) {
	r.calls++
	r.databaseURL = databaseURL
	r.input = input
	return r.user, r.err
}

type fakeFixtureRunner struct {
	result          fixtures.Result
	plan            fixtures.Plan
	exportPlan      fixtures.ExportPlan
	exportResult    fixtures.ExportResult
	planErr         error
	exportPlanErr   error
	err             error
	exportErr       error
	root            string
	databaseURL     string
	exportTarget    shape.AppRef
	includeLinks    bool
	planCalls       int
	calls           int
	exportPlanCalls int
	exportCalls     int
}

func (r *fakeFixtureRunner) Plan(_ context.Context, root string) (fixtures.Plan, error) {
	r.planCalls++
	r.root = root
	return r.plan, r.planErr
}

func (r *fakeFixtureRunner) Apply(_ context.Context, root string, databaseURL string) (fixtures.Result, error) {
	r.calls++
	r.root = root
	r.databaseURL = databaseURL
	return r.result, r.err
}

func (r *fakeFixtureRunner) ExportPlan(_ context.Context, root string, databaseURL string, target shape.AppRef, includeLinks bool) (fixtures.ExportPlan, error) {
	r.exportPlanCalls++
	r.root = root
	r.databaseURL = databaseURL
	r.exportTarget = target
	r.includeLinks = includeLinks
	return r.exportPlan, r.exportPlanErr
}

func (r *fakeFixtureRunner) WriteExportPlan(_ context.Context, plan fixtures.ExportPlan) (fixtures.ExportResult, error) {
	r.exportCalls++
	r.exportPlan = plan
	return r.exportResult, r.exportErr
}

func fixturePlan(fileCount int, recordCount int) fixtures.Plan {
	files := make([]fixtures.LoadedFile, fileCount)
	if fileCount == 0 {
		return fixtures.Plan{Files: files}
	}
	for i := 0; i < recordCount; i++ {
		index := i % fileCount
		files[index].Fixture.Records = append(files[index].Fixture.Records, fixtures.Record{})
	}
	return fixtures.Plan{Files: files}
}

type fakeDatabaseRunner struct {
	checkErr     error
	createResult db.DatabaseResult
	createErr    error
	dropResult   db.DatabaseResult
	dropErr      error
	operation    string
	root         string
	databaseURL  string
	calls        int
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

type fakeSchemaSyncRunner struct {
	result                db.SchemaSyncResult
	err                   error
	patchApplyResult      db.PatchApplyResult
	patchApplyErr         error
	patchPlan             db.PatchPlan
	patchPlanErr          error
	plan                  db.SchemaPlan
	planErr               error
	pruneResult           db.SchemaPruneResult
	pruneErr              error
	prunePlan             db.SchemaPrunePlan
	prunePlanErr          error
	root                  string
	databaseURL           string
	planRoot              string
	planDatabaseURL       string
	pruneRoot             string
	pruneDatabaseURL      string
	prunePlanRoot         string
	prunePlanDatabaseURL  string
	patchApplyRoot        string
	patchApplyDatabaseURL string
	patchApplyPhase       string
	patchApplyDygoVersion string
	patchPlanRoot         string
	patchPlanDatabaseURL  string
	patchPlanPhase        string
	calls                 int
	patchApplyCalls       int
	patchPlanCalls        int
	planCalls             int
	pruneCalls            int
	prunePlanCalls        int
}

func (r *fakeSchemaSyncRunner) ApplyPatches(_ context.Context, root string, databaseURL string, phase string, dygoVersion string) (db.PatchApplyResult, error) {
	r.patchApplyCalls++
	r.patchApplyRoot = root
	r.patchApplyDatabaseURL = databaseURL
	r.patchApplyPhase = phase
	r.patchApplyDygoVersion = dygoVersion
	return r.patchApplyResult, r.patchApplyErr
}

func (r *fakeSchemaSyncRunner) PatchPlan(_ context.Context, root string, databaseURL string, phase string) (db.PatchPlan, error) {
	r.patchPlanCalls++
	r.patchPlanRoot = root
	r.patchPlanDatabaseURL = databaseURL
	r.patchPlanPhase = phase
	return r.patchPlan, r.patchPlanErr
}

func (r *fakeSchemaSyncRunner) Plan(_ context.Context, root string, databaseURL string) (db.SchemaPlan, error) {
	r.planCalls++
	r.planRoot = root
	r.planDatabaseURL = databaseURL
	return r.plan, r.planErr
}

func (r *fakeSchemaSyncRunner) Prune(_ context.Context, root string, databaseURL string) (db.SchemaPruneResult, error) {
	r.pruneCalls++
	r.pruneRoot = root
	r.pruneDatabaseURL = databaseURL
	return r.pruneResult, r.pruneErr
}

func (r *fakeSchemaSyncRunner) PrunePlan(_ context.Context, root string, databaseURL string) (db.SchemaPrunePlan, error) {
	r.prunePlanCalls++
	r.prunePlanRoot = root
	r.prunePlanDatabaseURL = databaseURL
	return r.prunePlan, r.prunePlanErr
}

func (r *fakeSchemaSyncRunner) Sync(_ context.Context, root string, databaseURL string) (db.SchemaSyncResult, error) {
	r.calls++
	r.root = root
	r.databaseURL = databaseURL
	return r.result, r.err
}

func writeCLIEntity(t *testing.T, path string, body string) {
	t.Helper()

	body = strings.TrimSpace(body)
	isCollection := strings.Contains(filepath.ToSlash(path), "/_collections/")
	if !isCollection && !strings.Contains(body, "\nname:") && !strings.HasPrefix(body, "name:") {
		if strings.Contains(body, "\nroute:") {
			body = strings.Replace(body, "\nroute:", "\nname:\n  strategy: random\nroute:", 1)
		} else {
			body = strings.Replace(body, "\nfields:", "\nname:\n  strategy: random\nfields:", 1)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func cliEntityPath(root string, app string, entity string) string {
	return filepath.Join(root, "apps", app, "entities", entity, "entity.yml")
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

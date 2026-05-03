package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			name:       "prints default serve address",
			args:       []string{"serve"},
			wantStdout: "127.0.0.1:6790",
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

func TestSecretsCommands(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	run := func(args []string, stdin string) (string, string, error) {
		t.Helper()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		err := Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
		return stdout.String(), stderr.String(), err
	}

	if _, _, err := run([]string{"secrets", "init", "--env", "dev"}, ""); err == nil {
		t.Fatal("init --env dev error = nil, want error")
	}
	if stdout, _, err := run([]string{"secrets", "init", "--env", "development"}, ""); err != nil {
		t.Fatalf("init development error = %v", err)
	} else if !strings.Contains(stdout, "initialized development secrets") {
		t.Fatalf("init stdout = %q, want initialization message", stdout)
	}
	if _, _, err := run([]string{"secrets", "rm", "--env", "development", "DATABASE_URL"}, ""); err == nil {
		t.Fatal("secrets rm error = nil, want unknown command error")
	}

	if stdout, _, err := run([]string{"secrets", "set", "--env", "development", "DATABASE_URL", "--value", "postgres://local"}, ""); err != nil {
		t.Fatalf("set --value error = %v", err)
	} else if !strings.Contains(stdout, "set DATABASE_URL in development") {
		t.Fatalf("set stdout = %q, want set message", stdout)
	}

	if stdout, _, err := run([]string{"secrets", "get", "--env", "development", "DATABASE_URL"}, ""); err != nil {
		t.Fatalf("get error = %v", err)
	} else if stdout != "postgres://local\n" {
		t.Fatalf("get stdout = %q, want raw secret", stdout)
	}

	if stdout, _, err := run([]string{"secrets", "show", "--env", "development", "DATABASE_URL"}, ""); err != nil {
		t.Fatalf("show error = %v", err)
	} else if strings.Contains(stdout, "postgres://local") || !strings.Contains(stdout, "************ocal") {
		t.Fatalf("show stdout = %q, want redacted output", stdout)
	}

	if stdout, _, err := run([]string{"secrets", "show", "--env", "development", "DATABASE_URL", "--reveal"}, ""); err != nil {
		t.Fatalf("show --reveal error = %v", err)
	} else if !strings.Contains(stdout, "postgres://local") {
		t.Fatalf("show --reveal stdout = %q, want raw secret", stdout)
	}

	if stdout, _, err := run([]string{"secrets", "list", "--env", "development"}, ""); err != nil {
		t.Fatalf("list error = %v", err)
	} else if strings.Contains(stdout, "postgres://local") || !strings.Contains(stdout, "DATABASE_URL=************ocal") {
		t.Fatalf("list stdout = %q, want redacted entry", stdout)
	}

	configPath := filepath.Join(root, "configs", "app.yaml")
	if err := os.WriteFile(configPath, []byte("env:\n  DATABASE_URL:\n    secret: DATABASE_URL\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	if stdout, _, err := run([]string{"secrets", "validate", "--env", "development"}, ""); err != nil {
		t.Fatalf("validate error = %v", err)
	} else if !strings.Contains(stdout, "development secrets are valid") {
		t.Fatalf("validate stdout = %q, want success", stdout)
	}

	if stdout, _, err := run([]string{"secrets", "remove", "--env", "development", "DATABASE_URL"}, "no\n"); err != nil {
		t.Fatalf("remove canceled error = %v", err)
	} else if !strings.Contains(stdout, "remove canceled") {
		t.Fatalf("remove canceled stdout = %q, want canceled message", stdout)
	}

	if stdout, _, err := run([]string{"secrets", "remove", "--env", "development", "DATABASE_URL"}, "yes\n"); err != nil {
		t.Fatalf("remove confirmed error = %v", err)
	} else if !strings.Contains(stdout, "removed DATABASE_URL from development") {
		t.Fatalf("remove confirmed stdout = %q, want removed message", stdout)
	}

	if _, _, err := run([]string{"secrets", "get", "--env", "development", "DATABASE_URL"}, ""); err == nil {
		t.Fatal("get after remove error = nil, want error")
	}
}

func TestSecretsSetFromStdinAndFile(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	run := func(args []string, stdin string) (string, string, error) {
		t.Helper()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		err := Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
		return stdout.String(), stderr.String(), err
	}

	if _, _, err := run([]string{"secrets", "init", "--env", "staging"}, ""); err != nil {
		t.Fatalf("init staging error = %v", err)
	}
	if _, _, err := run([]string{"secrets", "set", "--env", "staging", "API_KEY"}, "secret-from-stdin\n"); err != nil {
		t.Fatalf("set from stdin error = %v", err)
	}
	if stdout, _, err := run([]string{"secrets", "get", "--env", "staging", "API_KEY"}, ""); err != nil {
		t.Fatalf("get API_KEY error = %v", err)
	} else if stdout != "secret-from-stdin\n" {
		t.Fatalf("get API_KEY stdout = %q, want stdin secret", stdout)
	}

	valuePath := filepath.Join(root, "value.txt")
	if err := os.WriteFile(valuePath, []byte("secret-from-file\nwith-newline"), 0o600); err != nil {
		t.Fatalf("WriteFile(value) error = %v", err)
	}
	if _, _, err := run([]string{"secrets", "set", "--env", "staging", "FILE_SECRET", "--from-file", valuePath}, ""); err != nil {
		t.Fatalf("set from file error = %v", err)
	}
	if stdout, _, err := run([]string{"secrets", "get", "--env", "staging", "FILE_SECRET"}, ""); err != nil {
		t.Fatalf("get FILE_SECRET error = %v", err)
	} else if stdout != "secret-from-file\nwith-newline\n" {
		t.Fatalf("get FILE_SECRET stdout = %q, want file contents plus command newline", stdout)
	}
}

func TestSecretsProductionWarningAndRotateKey(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	run := func(args []string, stdin string) (string, string, error) {
		t.Helper()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		err := Run(context.Background(), args, strings.NewReader(stdin), &stdout, &stderr)
		return stdout.String(), stderr.String(), err
	}

	if _, stderr, err := run([]string{"secrets", "init", "--env", "production"}, ""); err != nil {
		t.Fatalf("init production error = %v", err)
	} else if !strings.Contains(stderr, "production private keys") {
		t.Fatalf("init production stderr = %q, want production key warning", stderr)
	}
	if _, _, err := run([]string{"secrets", "set", "--env", "production", "DATABASE_URL", "--value", "postgres://prod"}, ""); err != nil {
		t.Fatalf("set production error = %v", err)
	}
	if stdout, stderr, err := run([]string{"secrets", "rotate-key", "--env", "production"}, ""); err != nil {
		t.Fatalf("rotate-key production error = %v", err)
	} else if !strings.Contains(stdout, "rotated production secrets key") || !strings.Contains(stderr, "production private keys") {
		t.Fatalf("rotate-key stdout=%q stderr=%q, want rotate message and warning", stdout, stderr)
	}
	if stdout, _, err := run([]string{"secrets", "get", "--env", "production", "DATABASE_URL"}, ""); err != nil {
		t.Fatalf("get after rotate error = %v", err)
	} else if stdout != "postgres://prod\n" {
		t.Fatalf("get after rotate stdout = %q, want preserved secret", stdout)
	}
}

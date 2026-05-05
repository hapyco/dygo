package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	t.Parallel()

	cfg := Default()

	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("Default().Server.Host = %q, want %q", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 6790 {
		t.Fatalf("Default().Server.Port = %d, want %d", cfg.Server.Port, 6790)
	}
	if cfg.Server.Address() != "127.0.0.1:6790" {
		t.Fatalf("Default().Server.Address() = %q, want %q", cfg.Server.Address(), "127.0.0.1:6790")
	}
}

func TestLoadRepositoryConfig(t *testing.T) {
	t.Parallel()

	cfg, err := Load(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("Load(repo root) error = %v, want nil", err)
	}
	if cfg.Server.Address() != "127.0.0.1:6790" {
		t.Fatalf("Load(repo root).Server.Address() = %q, want 127.0.0.1:6790", cfg.Server.Address())
	}
}

func TestLoadFileMergesDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "dygo.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 7777\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v, want nil", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("LoadFile().Server.Host = %q, want default host", cfg.Server.Host)
	}
	if cfg.Server.Port != 7777 {
		t.Fatalf("LoadFile().Server.Port = %d, want 7777", cfg.Server.Port)
	}
}

func TestLoadFileRequiresFile(t *testing.T) {
	t.Parallel()

	_, err := LoadFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("LoadFile(missing) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "read dygo config") {
		t.Fatalf("LoadFile(missing) error = %q, want read context", err.Error())
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	_, err := Decode([]byte("server:\n  host: 127.0.0.1\nunknown: true\n"))
	if err == nil {
		t.Fatal("Decode(unknown field) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "field unknown not found") {
		t.Fatalf("Decode(unknown field) error = %q, want unknown field", err.Error())
	}
}

func TestDecodeRejectsDuplicateKeys(t *testing.T) {
	t.Parallel()

	_, err := Decode([]byte("server:\n  port: 6790\n  port: 6791\n"))
	if err == nil {
		t.Fatal("Decode(duplicate key) error = nil, want error")
	}
	if !strings.Contains(err.Error(), `duplicate config key "$.server.port"`) {
		t.Fatalf("Decode(duplicate key) error = %q, want duplicate key", err.Error())
	}
}

func TestDecodeValidatesResolvedConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "empty host",
			body: "server:\n  host: \"\"\n",
			want: "server.host is required",
		},
		{
			name: "zero port",
			body: "server:\n  port: 0\n",
			want: "server.port must be between 1 and 65535",
		},
		{
			name: "port too high",
			body: "server:\n  port: 65536\n",
			want: "server.port must be between 1 and 65535",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Decode([]byte(tt.body))
			if err == nil {
				t.Fatal("Decode() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Decode() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

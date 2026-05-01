package config

import "testing"

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

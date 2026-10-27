package queues

import (
	"strings"
	"testing"
)

func TestDecodeQueueConfig(t *testing.T) {
	cfg, err := Decode([]byte(`queues:
  - name: default
    concurrency: 4
  - name: imports
    concurrency: 1
`))
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if !cfg.Has("default") || !cfg.Has("imports") {
		t.Fatalf("queue config = %+v, want default and imports queues", cfg)
	}
	if got := strings.Join(cfg.Names(), ","); got != "default,imports" {
		t.Fatalf("Names() = %q, want stable names", got)
	}
}

func TestDecodeQueueConfigRejectsInvalidRegistry(t *testing.T) {
	_, err := Decode([]byte(`queues:
  - name: default
    concurrency: 0
  - name: bad_queue
    concurrency: 1
`))
	if err == nil {
		t.Fatal("Decode() error = nil, want validation error")
	}
	for _, want := range []string{"concurrency must be greater than 0", `bad_queue" must be kebab-case`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Decode() error = %q, want %q", err.Error(), want)
		}
	}
}

func TestRetentionConfig(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
		valid bool
	}{
		{"", 0, true},
		{"retention-days: 30\n", 30, true},
		{"retention-days: 0\n", 0, true},
		{"retention-days: -1\n", 0, false},
		{"retention-days: 36501\n", 0, false},
	} {
		cfg, err := Decode([]byte(tc.value + "queues:\n  - name: default\n    concurrency: 1\n"))
		if (err == nil) != tc.valid || (err == nil && cfg.RetentionDays != tc.want) {
			t.Errorf("retention config %q: days=%d err=%v", tc.value, cfg.RetentionDays, err)
		}
	}
}

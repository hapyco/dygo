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

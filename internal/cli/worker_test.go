package cli

import (
	"testing"

	"github.com/hapyco/dygo/internal/queues"
)

func TestEffectiveWorkerQueues(t *testing.T) {
	cfg := queues.Config{Queues: []queues.Queue{
		{Name: "default", Concurrency: 4},
		{Name: "email", Concurrency: 2},
	}}

	all, err := effectiveWorkerQueues(cfg, nil, 0, false)
	if err != nil {
		t.Fatalf("effectiveWorkerQueues(all) error = %v, want nil", err)
	}
	if len(all) != 2 || all[0].Name != "default" || all[0].Concurrency != 4 || all[1].Name != "email" || all[1].Concurrency != 2 {
		t.Fatalf("all queues = %+v, want config order with configured concurrency", all)
	}

	selected, err := effectiveWorkerQueues(cfg, []string{"email", "email"}, 8, true)
	if err != nil {
		t.Fatalf("effectiveWorkerQueues(selected) error = %v, want nil", err)
	}
	if len(selected) != 1 || selected[0].Name != "email" || selected[0].Concurrency != 8 {
		t.Fatalf("selected queues = %+v, want deduped email with override concurrency", selected)
	}
}

func TestEffectiveWorkerQueuesRejectsInvalidInput(t *testing.T) {
	cfg := queues.Default()
	if _, err := effectiveWorkerQueues(cfg, []string{"missing"}, 0, false); err == nil {
		t.Fatal("effectiveWorkerQueues(unknown) error = nil, want error")
	}
	if _, err := effectiveWorkerQueues(cfg, nil, 0, true); err == nil {
		t.Fatal("effectiveWorkerQueues(concurrency 0) error = nil, want error")
	}
}

package store

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hapyco/dygo/internal/jobs"
)

func TestRetryJSONUsesRuntimeDecodableKeys(t *testing.T) {
	raw, err := encodeRetry(&jobs.Retry{
		Attempts:     3,
		InitialDelay: "10s",
		MaxDelay:     "5m",
	})
	if err != nil {
		t.Fatalf("encodeRetry() error = %v, want nil", err)
	}
	for _, want := range []string{`"attempts":3`, `"initial-delay":"10s"`, `"max-delay":"5m"`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("encodeRetry() = %s, want %s", raw, want)
		}
	}

	retry, err := decodeRetry(json.RawMessage(`{"attempts":3,"initial-delay":"10s","max-delay":"5m"}`))
	if err != nil {
		t.Fatalf("decodeRetry() error = %v, want nil", err)
	}
	if retry.Attempts != 3 || retry.InitialDelay != "10s" || retry.MaxDelay != "5m" {
		t.Fatalf("decodeRetry() = %+v, want retry values from hyphenated JSON keys", retry)
	}
}

func TestScheduleIdempotencyKeyUsesDueTime(t *testing.T) {
	dueAt := time.Date(2026, 6, 1, 4, 0, 0, 0, time.FixedZone("PKT", 5*60*60))
	got := scheduleIdempotencyKey("sales", "weekly-report", dueAt)
	want := "schedule:sales/weekly-report:2026-05-31T23:00:00Z"
	if got != want {
		t.Fatalf("scheduleIdempotencyKey() = %q, want %q", got, want)
	}
}

func TestScheduleJobBlocker(t *testing.T) {
	if got := scheduleJobBlocker(jobRecord{AppName: "sales", Key: "send-report", Enabled: false}); got != "job sales/send-report is disabled" {
		t.Fatalf("disabled blocker = %q, want disabled message", got)
	}
	if got := scheduleJobBlocker(jobRecord{AppName: "sales", Key: "send-report", Enabled: true, Retired: true}); got != "job sales/send-report is retired" {
		t.Fatalf("retired blocker = %q, want retired message", got)
	}
	if got := scheduleJobBlocker(jobRecord{AppName: "sales", Key: "send-report", Enabled: true}); got != "" {
		t.Fatalf("active blocker = %q, want empty", got)
	}
}

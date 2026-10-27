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

func TestRetryDelayUsesBoundedJitter(t *testing.T) {
	retry := &jobs.Retry{InitialDelay: "10s", MaxDelay: "25s"}
	for _, tc := range []struct {
		attempt int
		upper   time.Duration
	}{
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{3, 25 * time.Second},
		{100, 25 * time.Second},
	} {
		first, err := retryDelay(retry, tc.attempt)
		if err != nil {
			t.Fatal(err)
		}
		varied := false
		for i := 0; i < 32; i++ {
			delay, err := retryDelay(retry, tc.attempt)
			if err != nil {
				t.Fatal(err)
			}
			if delay < tc.upper/2 || delay > tc.upper {
				t.Fatalf("attempt %d delay = %s, want [%s, %s]", tc.attempt, delay, tc.upper/2, tc.upper)
			}
			varied = varied || delay != first
		}
		if !varied {
			t.Fatalf("attempt %d produced the same delay on every sample", tc.attempt)
		}
	}
}

func TestRetryDelayHandlesLimits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		retry *jobs.Retry
		want  time.Duration
		err   bool
	}{
		{"no policy", nil, 0, false},
		{"smallest delay", &jobs.Retry{InitialDelay: "1ns", MaxDelay: "1ns"}, 1, false},
		{"invalid duration", &jobs.Retry{InitialDelay: "bad", MaxDelay: "1s"}, 0, true},
		{"nonpositive delay", &jobs.Retry{InitialDelay: "0s", MaxDelay: "1s"}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := retryDelay(tc.retry, 1)
			if (err != nil) != tc.err || got != tc.want {
				t.Fatalf("retryDelay() = %s, %v; want %s, error=%t", got, err, tc.want, tc.err)
			}
		})
	}
	// Doubling a large delay must not overflow before applying the cap.
	got, err := retryDelay(&jobs.Retry{InitialDelay: "2000000h", MaxDelay: "2500000h"}, 2)
	if err != nil || got < 1250000*time.Hour || got > 2500000*time.Hour {
		t.Fatalf("large capped delay = %s, %v", got, err)
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

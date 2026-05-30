package store

import (
	"encoding/json"
	"strings"
	"testing"

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

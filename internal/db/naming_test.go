package db

import (
	"testing"
	"time"
)

func TestRenderSeriesPattern(t *testing.T) {
	rendered, width, key, err := renderSeriesPattern("sales-invoice", "SINV-{YYYY}-{MM}-{#####}", time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("renderSeriesPattern() error = %v, want nil", err)
	}
	if rendered != "SINV-2026-05-{#}" {
		t.Fatalf("rendered = %q, want placeholder-rendered pattern", rendered)
	}
	if width != 5 {
		t.Fatalf("width = %d, want 5", width)
	}
	if key != "sales-invoice:SINV-2026-05-{#####}" {
		t.Fatalf("key = %q, want entity scoped key", key)
	}
}

func TestRandomRecordName(t *testing.T) {
	name, err := randomRecordName(16)
	if err != nil {
		t.Fatalf("randomRecordName() error = %v, want nil", err)
	}
	if len(name) != 16 {
		t.Fatalf("randomRecordName() length = %d, want 16", len(name))
	}
}

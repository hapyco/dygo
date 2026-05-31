package dygodata

import (
	"encoding/json"
	"testing"

	"github.com/hapyco/dygo/pkg/dygo"
)

func TestLogInputMapsEntryToCoreRecordInput(t *testing.T) {
	input, err := logInput(dygo.LogEntry{
		Type:                dygo.TypeError,
		Source:              dygo.SourceHook,
		App:                 "crm",
		Title:               "Customer import failed",
		Message:             "stripe timeout",
		TraceID:             "trace-1",
		ReferenceEntity:     "crm.customer",
		ReferenceRecordID:   42,
		ReferenceRecordName: "CUST-42",
		Actor:               "admin@example.com",
		Metadata:            map[string]any{"batch": "May"},
	})
	if err != nil {
		t.Fatalf("logInput() error = %v, want nil", err)
	}

	for key, want := range map[string]string{
		"type":                  `"Error"`,
		"source":                `"Hook"`,
		"app":                   `"crm"`,
		"title":                 `"Customer import failed"`,
		"message":               `"stripe timeout"`,
		"trace-id":              `"trace-1"`,
		"reference-entity":      `"crm.customer"`,
		"reference-record-name": `"CUST-42"`,
		"actor":                 `"admin@example.com"`,
		"reference-record-id":   `42`,
	} {
		if got := string(input[key]); got != want {
			t.Fatalf("input[%q] = %s, want %s", key, got, want)
		}
	}

	var metadata map[string]any
	if err := json.Unmarshal(input["metadata"], &metadata); err != nil {
		t.Fatalf("metadata decode error = %v", err)
	}
	if metadata["batch"] != "May" {
		t.Fatalf("metadata = %#v, want batch", metadata)
	}
}

package dygo

import (
	"context"
	"errors"
	"testing"
)

func TestLogMergesContextDefaults(t *testing.T) {
	writer := &captureLogWriter{}
	ctx := WithLogWriter(context.Background(), writer)
	ctx = WithLogContext(ctx, LogContext{
		Source:              SourceHook,
		App:                 "crm",
		TraceID:             "trace-1",
		ReferenceEntity:     "crm.customer",
		ReferenceRecordID:   42,
		ReferenceRecordName: "CUST-42",
		Actor:               "admin@example.com",
		Metadata:            map[string]any{"hook": "after-create"},
	})

	if err := Log(ctx, LogEntry{
		Type:     TypeInfo,
		Title:    "Customer import started",
		Metadata: map[string]any{"batch": "May"},
	}); err != nil {
		t.Fatalf("Log() error = %v, want nil", err)
	}

	entry := writer.entry
	if entry.Type != TypeInfo || entry.Source != SourceHook || entry.App != "crm" || entry.Title != "Customer import started" {
		t.Fatalf("entry core fields = %+v, want context defaults and explicit title/type", entry)
	}
	if entry.TraceID != "trace-1" || entry.ReferenceEntity != "crm.customer" || entry.ReferenceRecordID != 42 || entry.ReferenceRecordName != "CUST-42" || entry.Actor != "admin@example.com" {
		t.Fatalf("entry context fields = %+v, want defaults", entry)
	}
	if entry.Metadata["hook"] != "after-create" || entry.Metadata["batch"] != "May" {
		t.Fatalf("entry metadata = %#v, want merged metadata", entry.Metadata)
	}
}

func TestErrorHelperWritesErrorMessageBestEffort(t *testing.T) {
	writer := &captureLogWriter{}
	ctx := WithLogWriter(context.Background(), writer)

	Error(ctx, "Customer import failed", errors.New("stripe timeout"))

	entry := writer.entry
	if entry.Type != TypeError || entry.Source != SourceSDK || entry.Title != "Customer import failed" || entry.Message != "stripe timeout" {
		t.Fatalf("entry = %+v, want error helper fields", entry)
	}
}

func TestLogWithoutWriterReturnsUnavailable(t *testing.T) {
	err := Log(context.Background(), LogEntry{Type: TypeInfo, Title: "Started"})
	if !errors.Is(err, ErrLogWriterUnavailable) {
		t.Fatalf("Log() error = %v, want ErrLogWriterUnavailable", err)
	}
}

type captureLogWriter struct {
	entry LogEntry
}

func (w *captureLogWriter) WriteLog(_ context.Context, entry LogEntry) error {
	w.entry = entry
	return nil
}

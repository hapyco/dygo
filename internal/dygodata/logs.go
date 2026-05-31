package dygodata

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/pkg/dygo"
)

// LogData persists dygo Log entries through Core Records.
type LogData struct {
	writer db.SystemRecordWriter
}

// NewLogData returns LogData backed by metadata-driven Record storage.
func NewLogData(queryer db.RecordQueryer) LogData {
	return LogData{writer: db.NewSystemRecordWriter(queryer)}
}

// WriteLog persists one LogEntry without running Record hooks or writing Activity.
func (d LogData) WriteLog(ctx context.Context, entry dygo.LogEntry) error {
	input, err := logInput(entry)
	if err != nil {
		return err
	}
	return d.writer.InsertByIdentity(ctx, "core", "log", input, db.SystemMutationSilent)
}

func logInput(entry dygo.LogEntry) (db.RecordInput, error) {
	input := db.RecordInput{
		"type":   logString(string(entry.Type)),
		"source": logString(string(entry.Source)),
		"title":  logString(entry.Title),
	}
	if value := strings.TrimSpace(entry.App); value != "" {
		input["app"] = logString(value)
	}
	if value := strings.TrimSpace(entry.Message); value != "" {
		input["message"] = logString(value)
	}
	if value := strings.TrimSpace(entry.TraceID); value != "" {
		input["trace-id"] = logString(value)
	}
	if value := strings.TrimSpace(entry.ReferenceEntity); value != "" {
		input["reference-entity"] = logString(value)
	}
	if entry.ReferenceRecordID != 0 {
		input["reference-record-id"] = logInt(entry.ReferenceRecordID)
	}
	if value := strings.TrimSpace(entry.ReferenceRecordName); value != "" {
		input["reference-record-name"] = logString(value)
	}
	if value := strings.TrimSpace(entry.Actor); value != "" {
		input["actor"] = logString(value)
	}
	if len(entry.Metadata) > 0 {
		metadata, err := json.Marshal(entry.Metadata)
		if err != nil {
			return nil, err
		}
		input["metadata"] = metadata
	}
	return input, nil
}

func logString(value string) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return encoded
}

func logInt(value int64) json.RawMessage {
	return json.RawMessage(strconv.FormatInt(value, 10))
}

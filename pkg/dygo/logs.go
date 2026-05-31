package dygo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hapyco/dygo/internal/corevalues"
)

// LogType is the stored type of a persisted Log.
type LogType string

const (
	TypeDebug   LogType = corevalues.LogTypeDebug
	TypeInfo    LogType = corevalues.LogTypeInfo
	TypeWarning LogType = corevalues.LogTypeWarning
	TypeError   LogType = corevalues.LogTypeError
	TypePanic   LogType = corevalues.LogTypePanic
)

// LogSource is the stored runtime surface that wrote a Log.
type LogSource string

const (
	SourceFramework LogSource = corevalues.LogSourceFramework
	SourceSDK       LogSource = corevalues.LogSourceSDK
	SourceHTTP      LogSource = corevalues.LogSourceHTTP
	SourceJob       LogSource = corevalues.LogSourceJob
	SourceHook      LogSource = corevalues.LogSourceHook
	SourceCLI       LogSource = corevalues.LogSourceCLI
	SourceStudio    LogSource = corevalues.LogSourceStudio
)

// LogEntry is one persisted diagnostic event.
type LogEntry struct {
	Type                LogType
	Source              LogSource
	App                 string
	Title               string
	Message             string
	TraceID             string
	ReferenceEntity     string
	ReferenceRecordID   int64
	ReferenceRecordName string
	Actor               string
	Metadata            map[string]any
}

// LogWriter persists Log entries for dygo helper functions.
type LogWriter interface {
	WriteLog(context.Context, LogEntry) error
}

// ErrLogWriterUnavailable means ctx does not contain a LogWriter.
var ErrLogWriterUnavailable = errors.New("dygo log writer is unavailable")

type logWriterContextKey struct{}
type logContextKey struct{}

// LogContext carries runtime defaults that dygo applies to LogEntry values.
type LogContext struct {
	Source              LogSource
	App                 string
	TraceID             string
	ReferenceEntity     string
	ReferenceRecordID   int64
	ReferenceRecordName string
	Actor               string
	Metadata            map[string]any
}

// WithLogWriter attaches a LogWriter to ctx.
func WithLogWriter(ctx context.Context, writer LogWriter) context.Context {
	if ctx == nil || writer == nil {
		return ctx
	}
	return context.WithValue(ctx, logWriterContextKey{}, writer)
}

// LogWriterFromContext returns the LogWriter attached to ctx.
func LogWriterFromContext(ctx context.Context) (LogWriter, bool) {
	if ctx == nil {
		return nil, false
	}
	writer, ok := ctx.Value(logWriterContextKey{}).(LogWriter)
	return writer, ok && writer != nil
}

// WithLogContext attaches runtime Log defaults to ctx.
func WithLogContext(ctx context.Context, defaults LogContext) context.Context {
	if ctx == nil {
		return ctx
	}
	merged := LogContextFromContext(ctx)
	merged = merged.with(defaults)
	return context.WithValue(ctx, logContextKey{}, merged)
}

// LogContextFromContext returns the Log defaults attached to ctx.
func LogContextFromContext(ctx context.Context) LogContext {
	if ctx == nil {
		return LogContext{}
	}
	defaults, _ := ctx.Value(logContextKey{}).(LogContext)
	return defaults
}

// Debug writes a best-effort Debug Log.
func Debug(ctx context.Context, title string, options ...LogOption) {
	writeBestEffort(ctx, TypeDebug, title, "", options...)
}

// Info writes a best-effort Info Log.
func Info(ctx context.Context, title string, options ...LogOption) {
	writeBestEffort(ctx, TypeInfo, title, "", options...)
}

// Warning writes a best-effort Warning Log.
func Warning(ctx context.Context, title string, options ...LogOption) {
	writeBestEffort(ctx, TypeWarning, title, "", options...)
}

// Error writes a best-effort Error Log.
func Error(ctx context.Context, title string, err error, options ...LogOption) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	writeBestEffort(ctx, TypeError, title, message, options...)
}

// Panic writes a best-effort Panic Log.
func Panic(ctx context.Context, title string, recovered any, options ...LogOption) {
	writeBestEffort(ctx, TypePanic, title, recoveredMessage(recovered), options...)
}

func writeBestEffort(ctx context.Context, logType LogType, title string, message string, options ...LogOption) {
	entry := LogEntry{Type: logType, Title: title, Message: message}
	applyLogOptions(&entry, options)
	_ = Log(ctx, entry)
}

// Log writes one LogEntry and returns persistence errors to strict callers.
func Log(ctx context.Context, entry LogEntry) error {
	writer, ok := LogWriterFromContext(ctx)
	if !ok {
		return ErrLogWriterUnavailable
	}
	entry = entry.withContext(LogContextFromContext(ctx))
	entry.normalize()
	if entry.Type == "" {
		return errors.New("dygo log type is required")
	}
	if entry.Title == "" {
		return errors.New("dygo log title is required")
	}
	return writer.WriteLog(ctx, entry)
}

// LogOption customizes a helper-written LogEntry.
type LogOption func(*LogEntry)

// WithTraceID sets the Log trace ID.
func WithTraceID(traceID string) LogOption {
	return func(entry *LogEntry) {
		entry.TraceID = strings.TrimSpace(traceID)
	}
}

// WithReference sets a polymorphic Record reference by app, Entity, and Record ID.
func WithReference(appName string, entity string, recordID int64) LogOption {
	return func(entry *LogEntry) {
		appName = strings.TrimSpace(appName)
		entity = strings.TrimSpace(entity)
		if appName != "" && entity != "" {
			entry.ReferenceEntity = appName + "." + entity
		}
		entry.ReferenceRecordID = recordID
	}
}

// WithReferenceName stores the related Record's name snapshot.
func WithReferenceName(name string) LogOption {
	return func(entry *LogEntry) {
		entry.ReferenceRecordName = strings.TrimSpace(name)
	}
}

// WithApp sets the producing app.
func WithApp(appName string) LogOption {
	return func(entry *LogEntry) {
		entry.App = strings.TrimSpace(appName)
	}
}

// WithSource sets the Log source.
func WithSource(source LogSource) LogOption {
	return func(entry *LogEntry) {
		entry.Source = LogSource(strings.TrimSpace(string(source)))
	}
}

// WithActor sets the actor user Record name.
func WithActor(actor string) LogOption {
	return func(entry *LogEntry) {
		entry.Actor = strings.TrimSpace(actor)
	}
}

// WithMessage sets the longer Log detail.
func WithMessage(message string) LogOption {
	return func(entry *LogEntry) {
		entry.Message = strings.TrimSpace(message)
	}
}

// WithMetadata adds one structured metadata value.
func WithMetadata(key string, value any) LogOption {
	return func(entry *LogEntry) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		if entry.Metadata == nil {
			entry.Metadata = map[string]any{}
		}
		entry.Metadata[key] = value
	}
}

func applyLogOptions(entry *LogEntry, options []LogOption) {
	for _, option := range options {
		if option != nil {
			option(entry)
		}
	}
}

func (entry LogEntry) withContext(defaults LogContext) LogEntry {
	if entry.Source == "" {
		entry.Source = defaults.Source
	}
	if entry.App == "" {
		entry.App = defaults.App
	}
	if entry.TraceID == "" {
		entry.TraceID = defaults.TraceID
	}
	if entry.ReferenceEntity == "" {
		entry.ReferenceEntity = defaults.ReferenceEntity
	}
	if entry.ReferenceRecordID == 0 {
		entry.ReferenceRecordID = defaults.ReferenceRecordID
	}
	if entry.ReferenceRecordName == "" {
		entry.ReferenceRecordName = defaults.ReferenceRecordName
	}
	if entry.Actor == "" {
		entry.Actor = defaults.Actor
	}
	entry.Metadata = mergedMetadata(defaults.Metadata, entry.Metadata)
	return entry
}

func (defaults LogContext) with(next LogContext) LogContext {
	if next.Source != "" {
		defaults.Source = next.Source
	}
	if next.App != "" {
		defaults.App = next.App
	}
	if next.TraceID != "" {
		defaults.TraceID = next.TraceID
	}
	if next.ReferenceEntity != "" {
		defaults.ReferenceEntity = next.ReferenceEntity
	}
	if next.ReferenceRecordID != 0 {
		defaults.ReferenceRecordID = next.ReferenceRecordID
	}
	if next.ReferenceRecordName != "" {
		defaults.ReferenceRecordName = next.ReferenceRecordName
	}
	if next.Actor != "" {
		defaults.Actor = next.Actor
	}
	defaults.Metadata = mergedMetadata(defaults.Metadata, next.Metadata)
	return defaults
}

func (entry *LogEntry) normalize() {
	entry.Source = LogSource(strings.TrimSpace(string(entry.Source)))
	if entry.Source == "" {
		entry.Source = SourceSDK
	}
	entry.Type = LogType(strings.TrimSpace(string(entry.Type)))
	entry.App = strings.TrimSpace(entry.App)
	entry.Title = strings.TrimSpace(entry.Title)
	entry.Message = strings.TrimSpace(entry.Message)
	entry.TraceID = strings.TrimSpace(entry.TraceID)
	entry.ReferenceEntity = strings.TrimSpace(entry.ReferenceEntity)
	entry.ReferenceRecordName = strings.TrimSpace(entry.ReferenceRecordName)
	entry.Actor = strings.TrimSpace(entry.Actor)
}

func mergedMetadata(left map[string]any, right map[string]any) map[string]any {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	merged := make(map[string]any, len(left)+len(right))
	for key, value := range left {
		merged[key] = value
	}
	for key, value := range right {
		merged[key] = value
	}
	return merged
}

func recoveredMessage(recovered any) string {
	if recovered == nil {
		return ""
	}
	if err, ok := recovered.(error); ok {
		return err.Error()
	}
	return strings.TrimSpace(fmt.Sprint(recovered))
}

package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dygo-dev/dygo/internal/auth"
)

// AuthSessionWriter persists login sessions through the framework system writer.
type AuthSessionWriter struct {
	writer SystemRecordWriter
}

// NewAuthSessionWriter returns an auth session writer backed by queryer.
func NewAuthSessionWriter(queryer RecordQueryer) AuthSessionWriter {
	return AuthSessionWriter{writer: NewSystemRecordWriter(queryer)}
}

// CreateSession creates one Core session Record.
func (w AuthSessionWriter) CreateSession(ctx context.Context, input auth.SessionInput) error {
	recordInput := RecordInput{
		"user":         systemRecordInt(input.UserID),
		"token-digest": systemRecordString(input.TokenDigest),
		"status":       systemRecordString(input.Status),
		"started-at":   systemRecordTime(input.StartedAt),
		"expires-at":   systemRecordTime(input.ExpiresAt),
		"last-seen-at": systemRecordTime(input.LastSeenAt),
	}
	_, err := w.writer.InsertByIdentity(ctx, "core", "session", recordInput, SystemMutationOptions{})
	return err
}

// AuthAdminWriter persists the first administrator through the framework system writer.
type AuthAdminWriter struct {
	writer SystemRecordWriter
}

// NewAuthAdminWriter returns an auth admin writer backed by queryer.
func NewAuthAdminWriter(queryer RecordQueryer) AuthAdminWriter {
	return AuthAdminWriter{writer: NewSystemRecordWriter(queryer)}
}

// SaveAdmin creates or promotes the first Core user administrator Record.
func (w AuthAdminWriter) SaveAdmin(ctx context.Context, input auth.AdminInput) (auth.User, error) {
	match := RecordInput{
		"email": systemRecordString(input.Email),
	}
	recordInput := RecordInput{
		"email":         systemRecordString(input.Email),
		"full-name":     systemRecordString(input.FullName),
		"password":      systemRecordString(input.Password),
		"enabled":       systemRecordBool(true),
		"administrator": systemRecordBool(true),
	}
	record, err := w.writer.UpsertByIdentity(ctx, "core", "user", match, recordInput, SystemMutationOptions{Bootstrap: true, ReturnRecord: true})
	if err != nil {
		return auth.User{}, err
	}
	return authUserFromRecord(record)
}

func systemRecordTime(value time.Time) json.RawMessage {
	if value.IsZero() {
		return json.RawMessage("null")
	}
	return systemRecordString(value.UTC().Format(time.RFC3339))
}

func authUserFromRecord(record Record) (auth.User, error) {
	id, err := recordInt64(record, "id")
	if err != nil {
		return auth.User{}, err
	}
	email, err := recordString(record, "email")
	if err != nil {
		return auth.User{}, err
	}
	fullName, err := recordString(record, "full-name")
	if err != nil {
		return auth.User{}, err
	}
	enabled, err := recordBool(record, "enabled")
	if err != nil {
		return auth.User{}, err
	}
	administrator, err := recordBool(record, "administrator")
	if err != nil {
		return auth.User{}, err
	}
	return auth.User{ID: id, Email: email, FullName: fullName, Enabled: enabled, Administrator: administrator}, nil
}

func recordInt64(record Record, key string) (int64, error) {
	value, ok := record[key]
	if !ok {
		return 0, fmt.Errorf("auth record missing %s", key)
	}
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case float64:
		return int64(typed), nil
	default:
		return 0, fmt.Errorf("auth record field %s has type %T", key, value)
	}
}

func recordString(record Record, key string) (string, error) {
	value, ok := record[key]
	if !ok {
		return "", fmt.Errorf("auth record missing %s", key)
	}
	typed, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("auth record field %s has type %T", key, value)
	}
	return typed, nil
}

func recordBool(record Record, key string) (bool, error) {
	value, ok := record[key]
	if !ok {
		return false, fmt.Errorf("auth record missing %s", key)
	}
	typed, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("auth record field %s has type %T", key, value)
	}
	return typed, nil
}

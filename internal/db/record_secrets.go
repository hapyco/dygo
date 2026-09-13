package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hapyco/dygo/internal/recordsecret"
	"github.com/hapyco/dygo/pkg/dygo"
	"github.com/jackc/pgx/v5"
)

func secretBinding(layout recordLayout, field recordField) string {
	return layout.AppName + "/" + layout.Entity + "/" + field.Name
}
func secretStorageValue(ctx context.Context, layout recordLayout, field recordField, raw json.RawMessage) (any, error) {
	if rawIsNull(raw) {
		if field.Required {
			return nil, recordError(RecordErrorValidation, "required field cannot be null", map[string]any{"field": field.Name}, nil)
		}
		return nil, nil
	}
	var value string
	if json.Unmarshal(raw, &value) != nil || value == "" {
		return nil, recordError(RecordErrorValidation, "secret must be a non-empty string", map[string]any{"field": field.Name}, nil)
	}
	ring, err := recordsecret.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if ring.Rotating {
		return nil, errors.New("Record key rotation is in progress")
	}
	return ring.Encrypt(secretBinding(layout, field), value)
}

// DecryptSecret is an internal storage primitive. The SDK enforces system access.
func (s RecordStore) DecryptSecret(ctx context.Context, appName, entity string, id int64, name string) (string, error) {
	layout, err := s.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return "", err
	}
	field, ok := layout.FieldByName[name]
	if !ok || field.Type != "secret" {
		return "", recordError(RecordErrorInvalidRequest, "field is not a secret", nil, nil)
	}
	var ciphertext *string
	// Collection rows are accessible here only through the explicit system SDK.
	where, args := s.scopedWhere(quoteIdent(recordSelectSourceAlias)+"."+quoteIdent(systemColumnID)+" = $1", []any{id})
	err = s.queryer.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM %s AS %s WHERE %s", quoteIdent(field.Column), quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias), where), args...).Scan(&ciphertext)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", recordError(RecordErrorNotFound, "record not found", nil, nil)
	}
	if err != nil {
		return "", fmt.Errorf("read Record secret failed: %w", err)
	}
	if ciphertext == nil {
		return "", dygo.ErrSecretUnset
	}
	ring, err := recordsecret.FromContext(ctx)
	if err != nil {
		return "", err
	}
	return ring.Decrypt(secretBinding(layout, field), *ciphertext)
}

func (s RecordStore) SecretStatus(ctx context.Context, entity string, id int64) (dygo.SecretStatus, error) {
	layout, err := s.recordLayout(ctx, entity)
	if err != nil {
		return dygo.SecretStatus{}, err
	}
	return s.secretStatus(ctx, layout, id)
}
func (s RecordStore) SecretStatusByIdentity(ctx context.Context, appName, entity string, id int64) (dygo.SecretStatus, error) {
	layout, err := s.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return dygo.SecretStatus{}, err
	}
	return s.secretStatus(ctx, layout, id)
}
func (s RecordStore) secretStatus(ctx context.Context, layout recordLayout, id int64) (dygo.SecretStatus, error) {
	record, err := s.getRecordWithLayout(ctx, layout, id)
	if err != nil {
		return dygo.SecretStatus{}, err
	}
	// TODO: batch presence projections if Entities gain many secret fields.
	result := dygo.SecretStatus{Fields: map[string]bool{}, Collections: map[string]map[int64]map[string]bool{}}
	for _, field := range layout.Fields {
		if field.Type != "secret" {
			continue
		}
		if err := s.AuthorizeField(ctx, layout.AppName, layout.Entity, id, field.Name, false); err != nil {
			var denied RecordError
			if errors.As(err, &denied) && denied.Code == RecordErrorPermissionDenied {
				continue
			}
			return result, err
		}
		where, args := s.scopedReadWhere("id = $1", []any{id}, []string{field.Name})
		var present bool
		err = s.queryer.QueryRow(ctx, fmt.Sprintf("SELECT %s IS NOT NULL FROM %s AS %s WHERE %s", quoteIdent(field.Column), quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias), where), args...).Scan(&present)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return result, fmt.Errorf("read secret status failed: %w", err)
		}
		result.Fields[field.Name] = present
	}
	for name, collection := range layout.Collections {
		if _, visible := record[name]; !visible {
			continue
		}
		fields := []recordField{}
		for _, field := range collection.Layout.Fields {
			if field.Type == "secret" {
				fields = append(fields, field)
			}
		}
		if len(fields) == 0 {
			continue
		}
		if err := s.AuthorizeField(ctx, layout.AppName, layout.Entity, id, name, false); err != nil {
			var denied RecordError
			if errors.As(err, &denied) && denied.Code == RecordErrorPermissionDenied {
				continue
			}
			return result, err
		}
		selectSQL := "id"
		for _, field := range fields {
			selectSQL += ", " + quoteIdent(field.Column) + " IS NOT NULL"
		}
		rows, err := s.queryer.Query(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE parent_entity_id=$1 AND parent_record_id=$2 AND parent_field_id=$3 ORDER BY ordinal", selectSQL, quoteIdent(collection.Layout.Table)), layout.EntityID, id, collection.Field.ID)
		if err != nil {
			return result, fmt.Errorf("read collection secret status failed: %w", err)
		}
		statuses := map[int64]map[string]bool{}
		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				rows.Close()
				return result, fmt.Errorf("read secret status failed: %w", err)
			}
			rowID := values[0].(int64)
			statuses[rowID] = map[string]bool{}
			for i, f := range fields {
				statuses[rowID][f.Name] = values[i+1].(bool)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return result, fmt.Errorf("read secret status failed: %w", err)
		}
		result.Collections[name] = statuses
	}
	return result, nil
}

// hiddenHookInput keeps submitted secrets out of ordinary Hook maps. Collections
// with submitted secrets retain their row positions while Hooks edit other values.
func hiddenHookInput(layout recordLayout, original RecordInput) (RecordInput, func() error, error) {
	if !layoutHasSecrets(layout) {
		return original, func() error { return nil }, nil
	}
	clean := cloneRecordInput(original)
	restorers := []func() error{}
	for _, field := range layout.Fields {
		if field.Type == "secret" {
			delete(clean, field.Name)
		}
	}
	for name, collection := range layout.Collections {
		if !layoutHasSecrets(*collection.Layout) {
			continue
		}
		raw, exists := original[name]
		if !exists {
			continue
		}
		var rows []RecordInput
		if json.Unmarshal(raw, &rows) != nil {
			return nil, nil, recordError(RecordErrorValidation, "collection field must be an array", map[string]any{"field": name}, nil)
		}
		cleaned := make([]RecordInput, len(rows))
		restoreRows := make([]func() error, len(rows))
		for i, row := range rows {
			var err error
			cleaned[i], restoreRows[i], err = hiddenHookInput(*collection.Layout, row)
			if err != nil {
				return nil, nil, err
			}
		}
		clean[name], _ = json.Marshal(cleaned)
		restorers = append(restorers, func() error {
			var next []RecordInput
			if json.Unmarshal(clean[name], &next) != nil || len(next) != len(rows) {
				return recordError(RecordErrorValidation, "Hooks cannot change submitted collection row structure", map[string]any{"field": name}, nil)
			}
			for i, row := range next {
				if string(row["id"]) != string(rows[i]["id"]) {
					return recordError(RecordErrorValidation, "Hooks cannot reorder submitted collection rows", map[string]any{"field": name}, nil)
				}
				for key := range cleaned[i] {
					delete(cleaned[i], key)
				}
				for key, value := range row {
					cleaned[i][key] = value
				}
				if err := restoreRows[i](); err != nil {
					return err
				}
			}
			clean[name], _ = json.Marshal(rows)
			return nil
		})
	}
	return clean, func() error {
		if err := rejectHookSecrets(layout, clean); err != nil {
			return err
		}
		for _, restore := range restorers {
			if err := restore(); err != nil {
				return err
			}
		}
		for name := range original {
			if layout.FieldByName[name].Type != "secret" {
				delete(original, name)
			}
		}
		for name, value := range clean {
			original[name] = value
		}
		return nil
	}, nil
}

func layoutHasSecrets(layout recordLayout) bool {
	for _, field := range layout.Fields {
		if field.Type == "secret" {
			return true
		}
	}
	for _, collection := range layout.Collections {
		if layoutHasSecrets(*collection.Layout) {
			return true
		}
	}
	return false
}

func rejectHookSecrets(layout recordLayout, input RecordInput) error {
	for _, field := range layout.Fields {
		if field.Type == "secret" {
			if _, exists := input[field.Name]; exists {
				return recordError(RecordErrorValidation, "Hooks cannot submit secret fields", map[string]any{"field": field.Name}, nil)
			}
		}
	}
	for name, collection := range layout.Collections {
		if !layoutHasSecrets(*collection.Layout) {
			continue
		}
		raw, exists := input[name]
		if !exists {
			continue
		}
		var rows []RecordInput
		if json.Unmarshal(raw, &rows) != nil {
			return recordError(RecordErrorValidation, "collection field must be an array", map[string]any{"field": name}, nil)
		}
		for _, row := range rows {
			if err := rejectHookSecrets(*collection.Layout, row); err != nil {
				return err
			}
		}
	}
	return nil
}

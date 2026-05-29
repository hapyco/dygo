package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hapyco/dygo/internal/entity/fieldtype"
)

func (s RecordStore) applyFetchedFields(ctx context.Context, layout recordLayout, input RecordInput, base Record) error {
	// TODO(fetch): cache traversed link records when multiple fetched fields share a path prefix.
	for _, field := range layout.Fields {
		if strings.TrimSpace(field.Fetch.From) == "" {
			continue
		}
		if !field.Storage || field.WriteOnly || field.Type == "collection" {
			continue
		}
		value, err := s.resolveFetchedField(ctx, layout, input, base, field)
		if err != nil {
			return err
		}
		input[field.Name] = value
	}
	return nil
}

func (s RecordStore) resolveFetchedField(ctx context.Context, layout recordLayout, input RecordInput, base Record, destination recordField) (json.RawMessage, error) {
	segments := strings.Split(strings.TrimSpace(destination.Fetch.From), ".")
	if len(segments) < 2 {
		return json.RawMessage("null"), recordError(RecordErrorInternal, "field fetch metadata is invalid", map[string]any{"entity": layout.Entity, "field": destination.Name}, nil)
	}

	currentLayout := layout
	var currentRecord Record
	for index, segment := range segments {
		field, ok := currentLayout.fetchPathField(segment)
		if !ok {
			return json.RawMessage("null"), recordError(RecordErrorInternal, "field fetch path is invalid", map[string]any{"entity": layout.Entity, "field": destination.Name, "segment": segment}, nil)
		}

		var raw json.RawMessage
		if index == 0 {
			raw = fetchInputValue(field, input, base)
		} else {
			raw = fetchRecordValue(currentRecord, field)
		}

		if index == len(segments)-1 {
			if rawIsNull(raw) {
				return json.RawMessage("null"), nil
			}
			return raw, nil
		}

		if rawIsNull(raw) {
			return json.RawMessage("null"), nil
		}
		if field.Type != "link" {
			return json.RawMessage("null"), recordError(RecordErrorInternal, "field fetch path segment is not a link", map[string]any{"entity": currentLayout.Entity, "field": field.Name}, nil)
		}
		id, err := s.fetchLinkID(ctx, currentLayout, field, raw)
		if err != nil {
			return nil, err
		}
		if id == 0 {
			return json.RawMessage("null"), nil
		}
		targetLayout, err := s.fetchTargetLayout(ctx, currentLayout, field)
		if err != nil {
			return nil, err
		}
		record, err := s.getRecordWithLayout(ctx, targetLayout, id)
		if err != nil {
			return nil, err
		}
		currentLayout = targetLayout
		currentRecord = record
	}
	return json.RawMessage("null"), nil
}

func (l recordLayout) fetchPathField(name string) (recordField, bool) {
	if field, ok := l.FieldByName[name]; ok {
		return field, true
	}
	if name == systemFieldName {
		return recordField{Name: systemFieldName, Type: "text", Column: systemColumnName, Storage: true, Nameable: true, ValueKind: fieldtype.ValueString, SystemName: true}, true
	}
	return recordField{}, false
}

func fetchInputValue(field recordField, input RecordInput, base Record) json.RawMessage {
	if raw, ok := input[field.Name]; ok {
		return cloneRawMessage(raw)
	}
	return fetchRecordValue(base, field)
}

func fetchRecordValue(record Record, field recordField) json.RawMessage {
	if record == nil {
		return json.RawMessage("null")
	}
	value, ok := record[field.Name]
	if !ok || value == nil {
		return json.RawMessage("null")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("null")
	}
	return raw
}

func (s RecordStore) fetchLinkID(ctx context.Context, layout recordLayout, field recordField, raw json.RawMessage) (int64, error) {
	value, err := s.linkValueCodec().storageValue(ctx, layout, field, raw)
	if err != nil {
		return 0, err
	}
	if value == nil {
		return 0, nil
	}
	id, ok := value.(int64)
	if !ok {
		return 0, recordError(RecordErrorInternal, "link field storage value is invalid", map[string]any{"entity": layout.Entity, "field": field.Name, "type": fmt.Sprintf("%T", value)}, nil)
	}
	return id, nil
}

func (s RecordStore) fetchTargetLayout(ctx context.Context, layout recordLayout, field recordField) (recordLayout, error) {
	targetApp := strings.TrimSpace(field.Options.App)
	if targetApp == "" {
		targetApp = layout.AppName
	}
	targetEntity := strings.TrimSpace(field.Options.Entity)
	if targetEntity == "" {
		return recordLayout{}, recordError(RecordErrorInternal, "link field target metadata is invalid", map[string]any{"entity": layout.Entity, "field": field.Name}, nil)
	}
	return s.recordLayoutByIdentity(ctx, targetApp, targetEntity)
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	cloned := make([]byte, len(raw))
	copy(cloned, raw)
	return cloned
}

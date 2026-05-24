package db

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/dygo-dev/dygo/internal/entity/schema"
	namegen "github.com/dygo-dev/dygo/internal/naming"
)

type recordNaming struct {
	Strategy string `json:"strategy"`
	Label    string `json:"label,omitempty"`
	Length   int    `json:"length,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	Format   string `json:"format,omitempty"`
}

func defaultRecordNaming() schema.Naming {
	return schema.Naming{Strategy: schema.NamingStrategyRandom, Length: schema.DefaultRandomNameLength}
}

func parseRecordNaming(raw json.RawMessage) (schema.Naming, error) {
	if len(raw) == 0 {
		return defaultRecordNaming(), nil
	}
	var naming schema.Naming
	if err := json.Unmarshal(raw, &naming); err != nil {
		return schema.Naming{}, err
	}
	if strings.TrimSpace(naming.Strategy) == "" {
		naming.Strategy = schema.NamingStrategyRandom
	}
	if naming.Strategy == schema.NamingStrategyRandom && naming.Length == 0 {
		naming.Length = schema.DefaultRandomNameLength
	}
	return naming, nil
}

// SingleRecordName returns the framework-owned system name for a single Entity row.
func SingleRecordName(entityKey string) string {
	return entityKey
}

func (s RecordStore) generateRecordName(ctx context.Context, layout recordLayout, input RecordInput) (string, error) {
	name, err := namegen.Generate(ctx, layout.Naming, recordNameResolver{store: s, layout: layout, input: input}, namegen.Options{
		Entity: layout.Entity,
		Series: recordSeriesCounter{
			store:  s,
			layout: layout,
		},
	})
	if err != nil {
		if IsRecordError(err) {
			return "", err
		}
		return "", recordError(RecordErrorInternal, "record naming metadata is invalid", map[string]any{"entity": layout.Entity, "strategy": layout.Naming.Strategy}, err)
	}
	return name, nil
}

type recordNameResolver struct {
	store  RecordStore
	layout recordLayout
	input  RecordInput
}

func (r recordNameResolver) Value(ctx context.Context, token string) (string, error) {
	field, ok := r.layout.FieldByName[token]
	if !ok {
		return "", recordError(RecordErrorInternal, "naming field metadata is missing", map[string]any{"entity": r.layout.Entity, "field": token}, nil)
	}
	raw, ok := r.input[token]
	if !ok {
		return "", recordError(RecordErrorValidation, "naming field is required", map[string]any{"entity": r.layout.Entity, "field": token}, nil)
	}
	return r.value(ctx, field, raw)
}

func (r recordNameResolver) value(ctx context.Context, field recordField, raw json.RawMessage) (string, error) {
	if field.Type != "link" {
		return recordNameValue(field, raw)
	}
	return r.store.linkValueCodec().nameFromRaw(ctx, r.layout, field, raw)
}

type recordSeriesCounter struct {
	store  RecordStore
	layout recordLayout
}

func (c recordSeriesCounter) Next(ctx context.Context, key string, pattern string) (int64, error) {
	var current int64
	err := c.store.queryer.QueryRow(ctx, `
INSERT INTO "naming_series" ("name", "entity_id", "key", "pattern", "current")
VALUES ($1, $2, $3, $4, 1)
ON CONFLICT ("key") DO UPDATE
SET "current" = "naming_series"."current" + 1,
	updated_at = now()
RETURNING "current"`, key, c.layout.EntityID, key, pattern).Scan(&current)
	if err != nil {
		return 0, classifyRecordDBError(err, "naming-series")
	}
	return current, nil
}

func recordNameValue(field recordField, raw json.RawMessage) (string, error) {
	if rawIsNull(raw) {
		return "", recordError(RecordErrorValidation, "naming field cannot be null", map[string]any{"field": field.Name}, nil)
	}
	if !field.Nameable {
		return "", recordError(RecordErrorValidation, "field type cannot be used for naming", map[string]any{"field": field.Name, "type": field.Type}, nil)
	}
	switch field.ValueKind {
	case fieldtype.ValueString:
		return jsonStringValue(field, raw)
	case fieldtype.ValueInteger:
		value, err := jsonIntValue(field, raw)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(value, 10), nil
	case fieldtype.ValueNumber:
		return jsonNumberStringValue(field, raw)
	case fieldtype.ValueBoolean:
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", recordError(RecordErrorValidation, "field must be a boolean", map[string]any{"field": field.Name}, err)
		}
		if value {
			return "true", nil
		}
		return "false", nil
	case fieldtype.ValueDate, fieldtype.ValueDatetime, fieldtype.ValueTime:
		return jsonStringValue(field, raw)
	default:
		return "", recordError(RecordErrorValidation, "field type cannot be used for naming", map[string]any{"field": field.Name, "type": field.Type}, nil)
	}
}

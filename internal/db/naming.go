package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dygo-dev/dygo/internal/entity/schema"
	"github.com/dygo-dev/dygo/internal/naming"
	"github.com/jackc/pgx/v5"
)

var seriesTokenPattern = regexp.MustCompile(`\{(YY|YYYY|MM|#+)\}`)

type recordNaming struct {
	Strategy string `json:"strategy"`
	Length   int    `json:"length,omitempty"`
	Field    string `json:"field,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
	Template string `json:"template,omitempty"`
}

func defaultRecordNaming() recordNaming {
	return recordNaming{Strategy: schema.NamingStrategyRandom, Length: schema.DefaultRandomNameLength}
}

func parseRecordNaming(raw json.RawMessage) (recordNaming, error) {
	if len(raw) == 0 {
		return defaultRecordNaming(), nil
	}
	var naming recordNaming
	if err := json.Unmarshal(raw, &naming); err != nil {
		return recordNaming{}, err
	}
	if strings.TrimSpace(naming.Strategy) == "" {
		naming.Strategy = schema.NamingStrategyRandom
	}
	if naming.Strategy == schema.NamingStrategyRandom && naming.Length == 0 {
		naming.Length = schema.DefaultRandomNameLength
	}
	return naming, nil
}

func randomRecordName(length int) (string, error) {
	if length <= 0 {
		length = schema.DefaultRandomNameLength
	}
	return naming.Random(length)
}

func (s RecordStore) seriesRecordName(ctx context.Context, layout recordLayout, pattern string, now time.Time) (string, error) {
	rendered, counterWidth, key, err := renderSeriesPattern(layout.Entity, pattern, now)
	if err != nil {
		return "", err
	}
	var current int64
	err = s.queryer.QueryRow(ctx, `
INSERT INTO "naming_series" ("name", "entity_id", "key", "pattern", "current")
VALUES ($1, $2, $3, $4, 1)
ON CONFLICT ("key") DO UPDATE
SET "current" = "naming_series"."current" + 1,
	updated_at = now()
RETURNING "current"`, key, layout.EntityID, key, pattern).Scan(&current)
	if err != nil {
		return "", classifyRecordDBError(err, "naming-series")
	}
	return strings.Replace(rendered, "{#}", fmt.Sprintf("%0*d", counterWidth, current), 1), nil
}

func (s RecordStore) templateRecordName(ctx context.Context, layout recordLayout, template string, input RecordInput) (string, error) {
	name, err := renderNameTemplate(template, func(token string) (string, error) {
		field, ok := layout.FieldByName[token]
		if !ok {
			return "", recordError(RecordErrorInternal, "naming template field metadata is missing", map[string]any{"entity": layout.Entity, "field": token}, nil)
		}
		raw, ok := input[token]
		if !ok {
			return "", recordError(RecordErrorValidation, "naming template field is required", map[string]any{"entity": layout.Entity, "field": token}, nil)
		}
		return s.templateRecordNameValue(ctx, layout, field, raw)
	})
	if err != nil {
		if IsRecordError(err) {
			return "", err
		}
		return "", recordError(RecordErrorInternal, "naming template metadata is invalid", map[string]any{"entity": layout.Entity}, err)
	}
	return name, nil
}

func (s RecordStore) templateRecordNameValue(ctx context.Context, layout recordLayout, field recordField, raw json.RawMessage) (string, error) {
	if field.Type != "link" {
		return recordNameValue(field, raw)
	}
	id, err := jsonIntValue(field, raw)
	if err != nil {
		return "", err
	}
	appName := strings.TrimSpace(field.Options.App)
	if appName == "" {
		appName = layout.AppName
	}
	table := entityTableName(appName, field.Options.Entity)
	var name string
	err = s.queryer.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1", quoteIdent("name"), quoteIdent(table), quoteIdent("id")), id).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", recordError(RecordErrorValidation, "naming template link target not found", map[string]any{"entity": layout.Entity, "field": field.Name, "id": id}, err)
	}
	if err != nil {
		return "", classifyRecordDBError(err, field.Options.Entity)
	}
	return name, nil
}

func renderNameTemplate(template string, resolve func(token string) (string, error)) (string, error) {
	var rendered strings.Builder
	for i := 0; i < len(template); {
		switch template[i] {
		case '{':
			end := strings.IndexByte(template[i+1:], '}')
			if end < 0 {
				return "", fmt.Errorf("template has an unclosed token")
			}
			token := template[i+1 : i+1+end]
			value, err := resolve(token)
			if err != nil {
				return "", err
			}
			rendered.WriteString(value)
			i += end + 2
		case '}':
			return "", fmt.Errorf("template has an unopened token")
		default:
			rendered.WriteByte(template[i])
			i++
		}
	}
	return rendered.String(), nil
}

func renderSeriesPattern(entity string, pattern string, now time.Time) (rendered string, counterWidth int, key string, err error) {
	counterTokens := 0
	rendered = seriesTokenPattern.ReplaceAllStringFunc(pattern, func(token string) string {
		name := strings.Trim(token, "{}")
		switch name {
		case "YY":
			return now.Format("06")
		case "YYYY":
			return now.Format("2006")
		case "MM":
			return now.Format("01")
		default:
			counterTokens++
			counterWidth = len(name)
			return "{#}"
		}
	})
	if counterTokens != 1 {
		return "", 0, "", fmt.Errorf("series pattern must include exactly one counter token")
	}
	key = entity + ":" + strings.Replace(rendered, "{#}", "{"+strings.Repeat("#", counterWidth)+"}", 1)
	return rendered, counterWidth, key, nil
}

func recordNameValue(field recordField, raw json.RawMessage) (string, error) {
	if rawIsNull(raw) {
		return "", recordError(RecordErrorValidation, "naming field cannot be null", map[string]any{"field": field.Name}, nil)
	}
	switch field.Type {
	case "text", "email", "phone", "long-text", "select", "attachment":
		return jsonStringValue(field, raw)
	case "int", "bigint", "link":
		value, err := jsonIntValue(field, raw)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(value, 10), nil
	case "decimal", "currency":
		return jsonNumberStringValue(field, raw)
	case "boolean":
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", recordError(RecordErrorValidation, "field must be a boolean", map[string]any{"field": field.Name}, err)
		}
		if value {
			return "true", nil
		}
		return "false", nil
	case "date", "datetime", "time":
		return jsonStringValue(field, raw)
	default:
		return "", recordError(RecordErrorValidation, "field type cannot be used for naming", map[string]any{"field": field.Name, "type": field.Type}, nil)
	}
}

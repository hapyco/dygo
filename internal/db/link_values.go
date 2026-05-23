package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type linkValueCodec struct {
	queryer RecordQueryer
}

func newLinkValueCodec(queryer RecordQueryer) linkValueCodec {
	return linkValueCodec{queryer: queryer}
}

func (s RecordStore) linkValueCodec() linkValueCodec {
	return newLinkValueCodec(s.queryer)
}

func (c linkValueCodec) storageValue(ctx context.Context, layout recordLayout, field recordField, raw json.RawMessage) (any, error) {
	if field.Type != "link" {
		return recordDBValue(field, raw)
	}
	if rawIsNull(raw) {
		return nil, nil
	}
	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		return c.idByName(ctx, layout, field, name)
	}
	return jsonIntValue(field, raw)
}

func (c linkValueCodec) displaySQL(layout recordLayout, field recordField) string {
	if field.Type != "link" {
		return recordSourceColumn(field.Column)
	}
	targetTable, ok := linkTargetTable(layout, field)
	if !ok {
		return recordSourceColumn(field.Column)
	}
	return fmt.Sprintf("(SELECT %s FROM %s WHERE %s = %s)", quoteIdent("name"), quoteIdent(targetTable), quoteIdent("id"), recordSourceColumn(field.Column))
}

func (c linkValueCodec) nameFromRaw(ctx context.Context, layout recordLayout, field recordField, raw json.RawMessage) (string, error) {
	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		return name, nil
	}
	id, err := jsonIntValue(field, raw)
	if err != nil {
		return "", err
	}
	return c.nameByID(ctx, layout, field, id)
}

func (c linkValueCodec) idByName(ctx context.Context, layout recordLayout, field recordField, name string) (int64, error) {
	targetTable, ok := linkTargetTable(layout, field)
	if !ok {
		return 0, recordError(RecordErrorInternal, "link field target metadata is invalid", map[string]any{"entity": layout.Entity, "field": field.Name}, nil)
	}
	var id int64
	err := c.queryer.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1", quoteIdent("id"), quoteIdent(targetTable), quoteIdent("name")), name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, recordError(RecordErrorValidation, "link target not found", map[string]any{"entity": layout.Entity, "field": field.Name, "target": field.Options.Entity, "name": name}, err)
	}
	if err != nil {
		return 0, classifyRecordDBError(err, field.Options.Entity)
	}
	return id, nil
}

func (c linkValueCodec) nameByID(ctx context.Context, layout recordLayout, field recordField, id int64) (string, error) {
	targetTable, ok := linkTargetTable(layout, field)
	if !ok {
		return "", recordError(RecordErrorInternal, "link field target metadata is invalid", map[string]any{"entity": layout.Entity, "field": field.Name}, nil)
	}
	var name string
	err := c.queryer.QueryRow(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s = $1", quoteIdent("name"), quoteIdent(targetTable), quoteIdent("id")), id).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", recordError(RecordErrorValidation, "link target not found", map[string]any{"entity": layout.Entity, "field": field.Name, "target": field.Options.Entity, "id": id}, err)
	}
	if err != nil {
		return "", classifyRecordDBError(err, field.Options.Entity)
	}
	return name, nil
}

func linkTargetTable(layout recordLayout, field recordField) (string, bool) {
	targetEntity := strings.TrimSpace(field.Options.Entity)
	if targetEntity == "" {
		return "", false
	}
	targetApp := strings.TrimSpace(field.Options.App)
	if targetApp == "" {
		targetApp = layout.AppName
	}
	return entityTableName(targetApp, targetEntity), true
}

package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/dygo-dev/dygo/internal/entity/schema"
)

func (s RecordStore) deleteRecordCollections(ctx context.Context, layout recordLayout, parentID int64) error {
	for _, fieldName := range layout.collectionNames() {
		collection := layout.Collections[fieldName]
		sql := fmt.Sprintf(
			"DELETE FROM %s WHERE %s = $1 AND %s = $2 AND %s = $3",
			quoteIdent(collection.Layout.Table),
			quoteIdent(systemColumnParentEntityID),
			quoteIdent(systemColumnParentRecordID),
			quoteIdent(systemColumnParentFieldID),
		)
		if _, err := s.queryer.Exec(ctx, sql, layout.EntityID, parentID, collection.Field.ID); err != nil {
			return classifyRecordDBError(err, collection.Layout.Entity)
		}
	}
	return nil
}

func (s RecordStore) loadRecordCollections(ctx context.Context, layout recordLayout, record Record) (Record, error) {
	if len(layout.Collections) == 0 || record == nil {
		return record, nil
	}
	parentID, err := activityRecordID(record)
	if err != nil {
		return nil, err
	}
	for _, fieldName := range layout.collectionNames() {
		collection := layout.Collections[fieldName]
		sql := fmt.Sprintf(
			"SELECT %s FROM %s AS %s WHERE %s = $1 AND %s = $2 AND %s = $3 ORDER BY %s ASC, %s ASC",
			collection.Layout.selectList(),
			quoteIdent(collection.Layout.Table),
			quoteIdent(recordSelectSourceAlias),
			quoteIdent(systemColumnParentEntityID),
			quoteIdent(systemColumnParentRecordID),
			quoteIdent(systemColumnParentFieldID),
			quoteIdent(systemColumnOrdinal),
			quoteIdent(systemColumnID),
		)
		rows, err := s.queryer.Query(ctx, sql, layout.EntityID, parentID, collection.Field.ID)
		if err != nil {
			return nil, classifyRecordDBError(err, collection.Layout.Entity)
		}
		children := []Record{}
		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				rows.Close()
				return nil, recordError(RecordErrorInternal, "read collection row failed", map[string]any{"entity": collection.Layout.Entity, "field": fieldName}, err)
			}
			child, err := collection.Layout.recordFromValues(values)
			if err != nil {
				rows.Close()
				return nil, err
			}
			children = append(children, child)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, classifyRecordDBError(err, collection.Layout.Entity)
		}
		rows.Close()
		record[fieldName] = children
	}
	return record, nil
}

func (s RecordStore) saveSubmittedCollections(ctx context.Context, layout recordLayout, parentID int64, input RecordInput, create bool) error {
	if len(layout.Collections) == 0 {
		return nil
	}
	for _, fieldName := range layout.collectionNames() {
		raw, ok := input[fieldName]
		if !ok {
			continue
		}
		collection := layout.Collections[fieldName]
		rows, err := layout.collectionRowInputs(fieldName, raw, !create)
		if err != nil {
			return err
		}
		if err := s.saveCollectionRows(ctx, collection, parentID, rows, create); err != nil {
			return err
		}
	}
	return nil
}

func (s RecordStore) saveCollectionRows(ctx context.Context, collection recordCollection, parentID int64, rows []recordCollectionRowInput, create bool) error {
	existing := map[int64]struct{}{}
	var err error
	if !create {
		existing, err = s.collectionExistingRowIDs(ctx, collection, parentID)
		if err != nil {
			return err
		}
	}
	submitted := map[int64]struct{}{}
	for _, row := range rows {
		if row.ID == 0 {
			continue
		}
		if create {
			return recordError(RecordErrorValidation, "collection row id cannot be written on create", map[string]any{"field": collection.Field.Name, "row-id": row.ID}, nil)
		}
		if _, ok := existing[row.ID]; !ok {
			return recordError(RecordErrorValidation, "collection row does not belong to parent record", map[string]any{"field": collection.Field.Name, "row-id": row.ID}, nil)
		}
		submitted[row.ID] = struct{}{}
	}
	if !create {
		for id := range existing {
			if _, ok := submitted[id]; ok {
				continue
			}
			if err := s.deleteCollectionRow(ctx, collection, parentID, id); err != nil {
				return err
			}
		}
	}
	for _, row := range rows {
		if row.ID == 0 {
			if err := s.insertCollectionRow(ctx, collection, parentID, row); err != nil {
				return err
			}
			continue
		}
		if err := s.updateCollectionRow(ctx, collection, parentID, row); err != nil {
			return err
		}
	}
	return nil
}

func (s RecordStore) collectionExistingRowIDs(ctx context.Context, collection recordCollection, parentID int64) (map[int64]struct{}, error) {
	sql := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = $1 AND %s = $2 AND %s = $3",
		quoteIdent(systemColumnID),
		quoteIdent(collection.Layout.Table),
		quoteIdent(systemColumnParentEntityID),
		quoteIdent(systemColumnParentRecordID),
		quoteIdent(systemColumnParentFieldID),
	)
	rows, err := s.queryer.Query(ctx, sql, collection.ParentEntityID, parentID, collection.Field.ID)
	if err != nil {
		return nil, classifyRecordDBError(err, collection.Layout.Entity)
	}
	defer rows.Close()

	ids := map[int64]struct{}{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, recordError(RecordErrorInternal, "read collection row ids failed", map[string]any{"entity": collection.Layout.Entity}, err)
		}
		if len(values) != 1 {
			return nil, recordError(RecordErrorInternal, "collection row id query returned an invalid column count", map[string]any{"entity": collection.Layout.Entity, "actual": len(values)}, nil)
		}
		id, err := recordIDFromDBValue(values[0], collection.Layout.Entity)
		if err != nil {
			return nil, err
		}
		ids[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, classifyRecordDBError(err, collection.Layout.Entity)
	}
	return ids, nil
}

func (s RecordStore) insertCollectionRow(ctx context.Context, collection recordCollection, parentID int64, row recordCollectionRowInput) error {
	parentEntityField := collectionSystemMutationField("parent-entity-id", systemColumnParentEntityID)
	parentRecordField := collectionSystemMutationField("parent-record-id", systemColumnParentRecordID)
	parentFieldField := collectionSystemMutationField("parent-field-id", systemColumnParentFieldID)
	ordinalField := collectionSystemMutationField("ordinal", systemColumnOrdinal)
	for attempt := 0; attempt <= randomNameRetries; attempt++ {
		mutation, err := s.createMutation(ctx, *collection.Layout, row.Input)
		if err != nil {
			return err
		}
		mutation.Columns = append(mutation.Columns, systemColumnParentEntityID)
		mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, parentEntityField))
		mutation.Values = append(mutation.Values, collection.ParentEntityID)
		mutation.Columns = append(mutation.Columns, systemColumnParentRecordID)
		mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, parentRecordField))
		mutation.Values = append(mutation.Values, parentID)
		mutation.Columns = append(mutation.Columns, systemColumnParentFieldID)
		mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, parentFieldField))
		mutation.Values = append(mutation.Values, collection.Field.ID)
		mutation.Columns = append(mutation.Columns, systemColumnOrdinal)
		mutation.Placeholders = append(mutation.Placeholders, recordPlaceholder(len(mutation.Values)+1, ordinalField))
		mutation.Values = append(mutation.Values, row.Ordinal)

		sql := insertRecordSQL(*collection.Layout, mutation, false)
		if _, err := s.queryer.Exec(ctx, sql, mutation.Values...); err == nil {
			return nil
		} else {
			err = classifyRecordDBError(err, collection.Layout.Entity)
			if collection.Layout.Naming.Strategy != schema.NamingStrategyRandom || !isRecordNameCollision(err, *collection.Layout) || attempt == randomNameRetries {
				return err
			}
		}
	}
	return recordError(RecordErrorInternal, "collection row insert failed", map[string]any{"entity": collection.Layout.Entity, "field": collection.Field.Name}, nil)
}

func (s RecordStore) updateCollectionRow(ctx context.Context, collection recordCollection, parentID int64, row recordCollectionRowInput) error {
	if err := collection.Layout.validateUpdateFields(row.Input); err != nil {
		return err
	}
	mutation, err := s.writeMutation(ctx, *collection.Layout, row.Input)
	if err != nil {
		return err
	}
	setClauses := make([]string, 0, len(mutation.Columns)+2)
	for i, column := range mutation.Columns {
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", quoteIdent(column), mutation.Placeholders[i]))
	}
	ordinalField := collectionSystemMutationField("ordinal", systemColumnOrdinal)
	args := append([]any(nil), mutation.Values...)
	args = append(args, row.Ordinal)
	setClauses = append(setClauses, fmt.Sprintf("%s = %s", quoteIdent(systemColumnOrdinal), recordPlaceholder(len(args), ordinalField)))
	setClauses = append(setClauses, fmt.Sprintf("%s = now()", quoteIdent(systemColumnUpdatedAt)))
	args = append(args, row.ID, collection.ParentEntityID, parentID, collection.Field.ID)
	sql := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d AND %s = $%d AND %s = $%d AND %s = $%d",
		quoteIdent(collection.Layout.Table),
		strings.Join(setClauses, ", "),
		quoteIdent(systemColumnID),
		len(args)-3,
		quoteIdent(systemColumnParentEntityID),
		len(args)-2,
		quoteIdent(systemColumnParentRecordID),
		len(args)-1,
		quoteIdent(systemColumnParentFieldID),
		len(args),
	)
	tag, err := s.queryer.Exec(ctx, sql, args...)
	if err != nil {
		return classifyRecordDBError(err, collection.Layout.Entity)
	}
	if tag.RowsAffected() == 0 {
		return recordError(RecordErrorValidation, "collection row does not belong to parent record", map[string]any{"field": collection.Field.Name, "row-id": row.ID}, nil)
	}
	return nil
}

func (s RecordStore) deleteCollectionRow(ctx context.Context, collection recordCollection, parentID int64, id int64) error {
	sql := fmt.Sprintf(
		"DELETE FROM %s WHERE %s = $1 AND %s = $2 AND %s = $3 AND %s = $4",
		quoteIdent(collection.Layout.Table),
		quoteIdent(systemColumnParentEntityID),
		quoteIdent(systemColumnParentRecordID),
		quoteIdent(systemColumnParentFieldID),
		quoteIdent(systemColumnID),
	)
	tag, err := s.queryer.Exec(ctx, sql, collection.ParentEntityID, parentID, collection.Field.ID, id)
	if err != nil {
		return classifyRecordDBError(err, collection.Layout.Entity)
	}
	if tag.RowsAffected() == 0 {
		return recordError(RecordErrorValidation, "collection row does not belong to parent record", map[string]any{"field": collection.Field.Name, "row-id": id}, nil)
	}
	return nil
}

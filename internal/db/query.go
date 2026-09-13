package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/pkg/dygo"
)

// LockRecordsByIdentity returns matching Records ordered by the requested sort and locks them with FOR UPDATE.
// Callers should invoke this on a transaction and keep the transaction open while using the returned Records.
func (s RecordStore) LockRecordsByIdentity(ctx context.Context, appName string, entity string, params RecordListParams) (RecordListResult, error) {
	if err := s.requireQueryer(); err != nil {
		return RecordListResult{}, err
	}
	params, err := normalizeRecordListParams(params)
	if err != nil {
		return RecordListResult{}, err
	}
	layout, err := s.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return RecordListResult{}, err
	}
	return s.listRecordsWithLock(ctx, layout, recordIdentityName(appName, entity), params, true)
}

// CountRecordsByIdentity counts matching non-collection Records without applying pagination.
func (s RecordStore) CountRecordsByIdentity(ctx context.Context, appName string, entity string, params RecordListParams) (int64, error) {
	layout, identity, err := s.queryLayoutByIdentity(ctx, appName, entity, "count")
	if err != nil {
		return 0, err
	}
	where, args, err := s.listWhere(ctx, layout, params.Filters)
	if err != nil {
		return 0, err
	}
	where, args = s.scopedReadWhere(where, args, queryFilterFields(params.Filters))
	sql := fmt.Sprintf("SELECT COUNT(*) FROM %s AS %s", quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias))
	if where != "" {
		sql += " WHERE " + where
	}
	var count int64
	if err := s.queryer.QueryRow(ctx, sql, args...).Scan(&count); err != nil {
		return 0, classifyRecordDBError(err, identity)
	}
	return count, nil
}

// ExistsRecordsByIdentity reports whether at least one Record matches without applying pagination.
func (s RecordStore) ExistsRecordsByIdentity(ctx context.Context, appName string, entity string, params RecordListParams) (bool, error) {
	layout, identity, err := s.queryLayoutByIdentity(ctx, appName, entity, "exists")
	if err != nil {
		return false, err
	}
	where, args, err := s.listWhere(ctx, layout, params.Filters)
	if err != nil {
		return false, err
	}
	where, args = s.scopedReadWhere(where, args, queryFilterFields(params.Filters))
	sql := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s AS %s", quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias))
	if where != "" {
		sql += " WHERE " + where
	}
	sql += ")"
	var exists bool
	if err := s.queryer.QueryRow(ctx, sql, args...).Scan(&exists); err != nil {
		return false, classifyRecordDBError(err, identity)
	}
	return exists, nil
}

// AggregateRecordsByIdentity evaluates supported aggregate expressions over matching Records.
func (s RecordStore) AggregateRecordsByIdentity(ctx context.Context, appName string, entity string, params dygo.AggregateParams) ([]dygo.AggregateResult, error) {
	layout, identity, err := s.queryLayoutByIdentity(ctx, appName, entity, "aggregate")
	if err != nil {
		return nil, err
	}
	if len(params.Aggregates) == 0 {
		return nil, queryInvalid(identity, "at least one aggregate is required")
	}
	where, args, err := s.listWhere(ctx, layout, recordFilters(params.Filters))
	if err != nil {
		return nil, err
	}
	readFields := queryFilterFields(recordFilters(params.Filters))
	for _, aggregate := range params.Aggregates {
		if aggregate.Field != "" {
			readFields = append(readFields, aggregate.Field)
		}
	}
	where, args = s.scopedReadWhere(where, args, readFields)
	expressions := make([]string, 0, len(params.Aggregates))
	results := make([]dygo.AggregateResult, 0, len(params.Aggregates))
	for index, spec := range params.Aggregates {
		expression, err := s.aggregateExpression(ctx, layout, spec.Function, spec.Field)
		if err != nil {
			return nil, err
		}
		alias := strings.TrimSpace(spec.Alias)
		if alias == "" {
			alias = aggregateAlias(spec.Function, spec.Field, index)
		}
		expressions = append(expressions, expression+" AS "+quoteIdent(alias))
		results = append(results, dygo.AggregateResult{Function: spec.Function, Field: spec.Field, Alias: alias})
	}
	sql := fmt.Sprintf("SELECT %s FROM %s AS %s", strings.Join(expressions, ", "), quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias))
	if where != "" {
		sql += " WHERE " + where
	}
	rows, err := s.queryer.Query(ctx, sql, args...)
	if err != nil {
		return nil, classifyRecordDBError(err, identity)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, classifyRecordDBError(err, identity)
		}
		return nil, recordError(RecordErrorInternal, "aggregate query returned no rows", map[string]any{"entity": identity}, nil)
	}
	values, err := rows.Values()
	if err != nil {
		return nil, recordError(RecordErrorInternal, "read aggregate record row failed", map[string]any{"entity": identity}, err)
	}
	if len(values) != len(results) {
		return nil, recordError(RecordErrorInternal, "aggregate column count did not match query", map[string]any{"entity": identity, "expected": len(results), "actual": len(values)}, nil)
	}
	if rows.Next() {
		return nil, recordError(RecordErrorInternal, "aggregate query returned multiple rows", map[string]any{"entity": identity}, nil)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyRecordDBError(err, identity)
	}
	for i := range results {
		results[i].Value = values[i]
	}
	return results, nil
}

// GroupRecordsByIdentity evaluates grouped aggregate expressions over matching Records.
func (s RecordStore) GroupRecordsByIdentity(ctx context.Context, appName string, entity string, params dygo.GroupByParams) ([]dygo.GroupByResult, error) {
	layout, identity, err := s.queryLayoutByIdentity(ctx, appName, entity, "group")
	if err != nil {
		return nil, err
	}
	if len(params.GroupBy) == 0 {
		return nil, queryInvalid(identity, "at least one group field is required")
	}
	if len(params.Aggregates) == 0 {
		return nil, queryInvalid(identity, "at least one aggregate is required")
	}
	groupExpressions := make([]string, 0, len(params.GroupBy))
	groupNames := make([]string, 0, len(params.GroupBy))
	for _, name := range params.GroupBy {
		path, err := s.recordFieldPath(ctx, layout, name, "group")
		if err != nil {
			return nil, err
		}
		groupExpressions = append(groupExpressions, path.Expression)
		groupNames = append(groupNames, strings.TrimSpace(name))
	}
	where, args, err := s.listWhere(ctx, layout, recordFilters(params.Filters))
	if err != nil {
		return nil, err
	}
	readFields := append(queryFilterFields(recordFilters(params.Filters)), params.GroupBy...)
	for _, aggregate := range params.Aggregates {
		if aggregate.Field != "" {
			readFields = append(readFields, aggregate.Field)
		}
	}
	where, args = s.scopedReadWhere(where, args, readFields)
	selectParts := make([]string, 0, len(params.GroupBy)+len(params.Aggregates))
	for i, expression := range groupExpressions {
		selectParts = append(selectParts, expression+" AS "+quoteIdent(groupNames[i]))
	}
	aggregateAliases := make([]string, 0, len(params.Aggregates))
	for index, spec := range params.Aggregates {
		expression, err := s.aggregateExpression(ctx, layout, spec.Function, spec.Field)
		if err != nil {
			return nil, err
		}
		alias := strings.TrimSpace(spec.Alias)
		if alias == "" {
			alias = aggregateAlias(spec.Function, spec.Field, index)
		}
		aggregateAliases = append(aggregateAliases, alias)
		selectParts = append(selectParts, expression+" AS "+quoteIdent(alias))
	}
	sql := fmt.Sprintf("SELECT %s FROM %s AS %s", strings.Join(selectParts, ", "), quoteIdent(layout.Table), quoteIdent(recordSelectSourceAlias))
	if where != "" {
		sql += " WHERE " + where
	}
	sql += " GROUP BY " + strings.Join(groupExpressions, ", ")
	orderParts := make([]string, len(groupExpressions))
	for i := range orderParts {
		orderParts[i] = fmt.Sprintf("%d ASC", i+1)
	}
	sql += " ORDER BY " + strings.Join(orderParts, ", ")
	normalized, err := normalizeRecordListParams(RecordListParams{Limit: params.Limit, Offset: params.Offset})
	if err != nil {
		return nil, err
	}
	args = append(args, normalized.Limit, normalized.Offset)
	sql += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	rows, err := s.queryer.Query(ctx, sql, args...)
	if err != nil {
		return nil, classifyRecordDBError(err, identity)
	}
	defer rows.Close()
	groups := []dygo.GroupByResult{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, recordError(RecordErrorInternal, "read grouped record row failed", map[string]any{"entity": identity}, err)
		}
		if len(values) != len(selectParts) {
			return nil, recordError(RecordErrorInternal, "grouped record column count did not match query", map[string]any{"entity": identity, "expected": len(selectParts), "actual": len(values)}, nil)
		}
		group := make(map[string]any, len(groupNames))
		for i, name := range groupNames {
			group[name] = values[i]
		}
		aggregates := make(map[string]any, len(aggregateAliases))
		for i, alias := range aggregateAliases {
			aggregates[alias] = values[len(groupNames)+i]
		}
		groups = append(groups, dygo.GroupByResult{Group: group, Aggregates: aggregates})
	}
	if err := rows.Err(); err != nil {
		return nil, classifyRecordDBError(err, identity)
	}
	return groups, nil
}

func (s RecordStore) aggregateExpression(ctx context.Context, layout recordLayout, function dygo.AggregateFunction, fieldName string) (string, error) {
	function = dygo.AggregateFunction(strings.ToLower(strings.TrimSpace(string(function))))
	fieldName = strings.TrimSpace(fieldName)
	if function != dygo.AggregateCount && function != dygo.AggregateSum && function != dygo.AggregateMin && function != dygo.AggregateMax {
		return "", queryInvalid(layout.Entity, "aggregate function is not supported")
	}
	if function == dygo.AggregateCount && fieldName == "" {
		return "COUNT(*)", nil
	}
	if fieldName == "" {
		return "", queryInvalid(layout.Entity, "aggregate field is required")
	}
	path, err := s.recordFieldPath(ctx, layout, fieldName, "aggregate")
	if err != nil {
		return "", err
	}
	if function == dygo.AggregateSum && path.Field.ValueKind != fieldtype.ValueInteger && path.Field.ValueKind != fieldtype.ValueNumber {
		return "", queryInvalid(layout.Entity, "sum requires a numeric field")
	}
	if (function == dygo.AggregateMin || function == dygo.AggregateMax) && !aggregateOrderable(path.Field) {
		return "", queryInvalid(layout.Entity, "min and max require an orderable field")
	}
	return fmt.Sprintf("%s(%s)", strings.ToUpper(string(function)), path.Expression), nil
}

func aggregateOrderable(field recordField) bool {
	switch field.ValueKind {
	case fieldtype.ValueString, fieldtype.ValueInteger, fieldtype.ValueNumber, fieldtype.ValueDate, fieldtype.ValueDatetime, fieldtype.ValueTime:
		return field.Type != "link"
	default:
		return false
	}
}

func aggregateAlias(function dygo.AggregateFunction, field string, index int) string {
	field = strings.TrimSpace(field)
	if field == "" {
		return string(function)
	}
	return fmt.Sprintf("%s_%s_%d", function, strings.ReplaceAll(field, ".", "_"), index+1)
}

func (s RecordStore) queryLayoutByIdentity(ctx context.Context, appName string, entity string, operation string) (recordLayout, string, error) {
	if err := s.requireQueryer(); err != nil {
		return recordLayout{}, "", err
	}
	layout, err := s.recordLayoutByIdentity(ctx, appName, entity)
	if err != nil {
		return recordLayout{}, "", err
	}
	identity := recordIdentityName(appName, entity)
	if layout.IsSingle || layout.IsCollection {
		return recordLayout{}, "", recordError(RecordErrorInvalidRequest, fmt.Sprintf("%s is not supported for this Entity", operation), map[string]any{"entity": identity}, nil)
	}
	return layout, identity, nil
}

func queryInvalid(entity string, message string) error {
	return recordError(RecordErrorInvalidRequest, message, map[string]any{"entity": entity}, nil)
}

func recordFilters(filters []dygo.RecordFilter) []RecordFilter {
	converted := make([]RecordFilter, len(filters))
	for i, filter := range filters {
		converted[i] = RecordFilter{Field: filter.Field, Operator: filter.Operator, Value: filter.Value}
	}
	return converted
}

func queryFilterFields(filters []RecordFilter) []string {
	fields := make([]string, 0, len(filters))
	for _, filter := range filters {
		fields = append(fields, filter.Field)
	}
	return fields
}

type recordFieldPath struct {
	Field      recordField
	Expression string
}

// recordFieldPath validates metadata-backed dot-separated link paths and builds a safe SQL expression.
func (s RecordStore) recordFieldPath(ctx context.Context, layout recordLayout, name string, operation string) (recordFieldPath, error) {
	parts := strings.Split(strings.TrimSpace(name), ".")
	if len(parts) == 0 {
		return recordFieldPath{}, queryInvalid(layout.Entity, "field is required")
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return recordFieldPath{}, queryInvalid(layout.Entity, "field path contains an empty segment")
		}
	}
	return s.recordFieldPathParts(ctx, layout, quoteIdent(recordSelectSourceAlias), parts, operation, false)
}

func (s RecordStore) recordFieldPathParts(ctx context.Context, layout recordLayout, source string, parts []string, operation string, nested bool) (recordFieldPath, error) {
	if len(parts) == 0 {
		return recordFieldPath{}, queryInvalid(layout.Entity, "field is required")
	}
	field, err := layout.listField(parts[0], operation)
	if err != nil {
		return recordFieldPath{}, err
	}
	column := source + "." + quoteIdent(field.Column)
	if len(parts) == 1 {
		if field.Type != "link" || !nested {
			if !nested {
				return recordFieldPath{Field: field, Expression: quoteIdent(field.Column)}, nil
			}
			return recordFieldPath{Field: field, Expression: column}, nil
		}
		targetTable, ok := linkTargetTable(layout, field)
		if !ok {
			return recordFieldPath{}, queryInvalid(layout.Entity, "link field target metadata is invalid")
		}
		alias := quoteIdent("_dygo_path_" + strconv.Itoa(pathAliasNumber(source)))
		return recordFieldPath{
			Field:      recordField{Name: strings.Join(parts, "."), Type: "text", Storage: true, Listable: true, ValueKind: fieldtype.ValueString},
			Expression: fmt.Sprintf("(SELECT %s.%s FROM %s AS %s WHERE %s.%s = %s)", alias, quoteIdent("name"), quoteIdent(targetTable), alias, alias, quoteIdent("id"), column),
		}, nil
	}
	if field.Type != "link" {
		return recordFieldPath{}, queryInvalid(layout.Entity, fmt.Sprintf("field %q is not a link", parts[0]))
	}
	targetApp := strings.TrimSpace(field.Options.App)
	if targetApp == "" {
		targetApp = layout.AppName
	}
	targetEntity := strings.TrimSpace(field.Options.Entity)
	if targetEntity == "" {
		return recordFieldPath{}, queryInvalid(layout.Entity, "link field target metadata is invalid")
	}
	targetLayout, err := s.recordLayoutByIdentity(ctx, targetApp, targetEntity)
	if err != nil {
		return recordFieldPath{}, err
	}
	alias := quoteIdent("_dygo_path_" + strconv.Itoa(pathAliasNumber(source)+1))
	child, err := s.recordFieldPathParts(ctx, targetLayout, alias, parts[1:], operation, true)
	if err != nil {
		return recordFieldPath{}, err
	}
	return recordFieldPath{Field: child.Field, Expression: fmt.Sprintf("(SELECT %s FROM %s AS %s WHERE %s.%s = %s)", child.Expression, quoteIdent(targetLayout.Table), alias, alias, quoteIdent("id"), column)}, nil
}

func pathAliasNumber(source string) int {
	count := strings.Count(source, "_dygo_path_")
	return count
}

// Package studiostate stores private Studio state through the Record SDK.
package studiostate

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/dygodata"
	"github.com/hapyco/dygo/internal/recordquery"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/pkg/dygo"
)

type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value,omitempty"`
}

type SavedFilter struct {
	ID              int64    `json:"id"`
	Entity          string   `json:"entity"`
	Label           string   `json:"label"`
	Filters         []Filter `json:"filters"`
	ValidationError string   `json:"validationError,omitempty"`
}

type Store struct{ queryer db.RecordQueryer }

const privateStateMaxRecords = 10000

func New(queryer db.RecordQueryer) Store { return Store{queryer: queryer} }

func fmtID(id int64) string { return strconv.FormatInt(id, 10) }

func invalid(message string) error {
	return db.RecordError{Code: db.RecordErrorInvalidRequest, Message: message}
}

func (s Store) records(actor dygo.Actor) dygo.RecordData {
	return dygodata.NewRecordData(s.queryer, nil).AsPrivate(actor, "manage authenticated user's private Studio state")
}

func input(values map[string]any) dygo.RecordInput {
	result := dygo.RecordInput{}
	for key, value := range values {
		result[key], _ = json.Marshal(value)
	}
	return result
}

func owner(actor dygo.Actor, ownerField string) dygo.RecordFilter {
	return dygo.RecordFilter{Field: ownerField + ".id", Operator: "eq", Value: fmtID(actor.UserID)}
}

func (s Store) privateOwnerField(ctx context.Context, entity string) (string, error) {
	meta, err := db.NewMetadataReader(s.queryer).GetEntityMetaByIdentity(ctx, "studio", entity)
	if err != nil {
		return "", err
	}
	if !meta.IsPrivate || strings.TrimSpace(meta.PrivateOwnerField) == "" {
		return "", db.RecordError{Code: db.RecordErrorPermissionDenied, Message: "Entity is not configured for private Studio state"}
	}
	return meta.PrivateOwnerField, nil
}

func (s Store) ensureCapacity(ctx context.Context, actor dygo.Actor, entity string) error {
	ownerField, err := s.privateOwnerField(ctx, entity)
	if err != nil {
		return err
	}
	count, err := s.records(actor).Count(ctx, "studio", entity, dygo.RecordListParams{Filters: []dygo.RecordFilter{owner(actor, ownerField)}})
	if err != nil {
		return err
	}
	if count >= privateStateMaxRecords {
		return db.RecordError{Code: db.RecordErrorValidation, Message: "private Studio state exceeds the per-user record limit"}
	}
	return nil
}

func (s Store) ownerName(ctx context.Context, actor dygo.Actor) (string, error) {
	record, err := dygodata.NewRecordData(s.queryer, nil).WithActionActor(actor).AsSystem("resolve authenticated private-state owner").Get(ctx, "core", "user", actor.UserID)
	if err != nil {
		return "", err
	}
	return recordStringValue(record, "name")
}

func requireActor(actor dygo.Actor) error {
	if actor.UserID <= 0 || actor.Email == "" {
		return db.RecordError{Code: db.RecordErrorPermissionDenied, Message: "authentication required"}
	}
	return nil
}

func recordIDValue(record dygo.Record) (int64, error) {
	id, ok := record["id"].(int64)
	if !ok {
		return 0, invalid("record id is missing")
	}
	return id, nil
}

func recordStringValue(record dygo.Record, field string) (string, error) {
	value, ok := record[field].(string)
	if !ok {
		return "", invalid(field + " is missing")
	}
	return value, nil
}

func validateKey(key string) error {
	if len(key) > 240 || !strings.Contains(key, ".") {
		return invalid("preference key must be namespaced")
	}
	for _, part := range strings.Split(key, ".") {
		if err := shape.ValidateMetadataName("preference key", part); err != nil {
			return invalid("invalid preference key")
		}
	}
	return nil
}

func (s Store) list(ctx context.Context, actor dygo.Actor, entity string, filters ...dygo.RecordFilter) ([]dygo.Record, error) {
	if err := requireActor(actor); err != nil {
		return nil, err
	}
	ownerField, err := s.privateOwnerField(ctx, entity)
	if err != nil {
		return nil, err
	}
	params := dygo.RecordListParams{Limit: recordquery.MaxLimit, Filters: append([]dygo.RecordFilter{owner(actor, ownerField)}, filters...)}
	records := []dygo.Record{}
	for {
		if len(records) >= privateStateMaxRecords {
			probeParams := params
			probeParams.Limit = 1
			probeParams.Offset = len(records)
			probe, err := s.records(actor).List(ctx, "studio", entity, probeParams)
			if err != nil {
				return nil, err
			}
			if len(probe.Records) > 0 {
				return nil, db.RecordError{Code: db.RecordErrorValidation, Message: "private Studio state exceeds the per-user record limit"}
			}
			return records, nil
		}
		remaining := privateStateMaxRecords - len(records)
		if params.Limit > remaining {
			params.Limit = remaining
		}
		page, err := s.records(actor).List(ctx, "studio", entity, params)
		if err != nil {
			return nil, err
		}
		records = append(records, page.Records...)
		if len(page.Records) < params.Limit {
			return records, nil
		}
		params.Offset += len(page.Records)
	}
}

func (s Store) Preferences(ctx context.Context, actor dygo.Actor) (map[string]any, error) {
	records, err := s.list(ctx, actor, "preference")
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	for _, record := range records {
		key, err := recordStringValue(record, "key")
		if err != nil {
			return nil, err
		}
		values[key] = record["value"]
	}
	return values, nil
}

func (s Store) PutPreference(ctx context.Context, actor dygo.Actor, key string, value json.RawMessage) error {
	if err := requireActor(actor); err != nil {
		return err
	}
	if err := validateKey(key); err != nil {
		return err
	}
	if !json.Valid(value) {
		return invalid("value must be JSON")
	}
	records := s.records(actor)
	name, err := s.ownerName(ctx, actor)
	if err != nil {
		return err
	}
	match := input(map[string]any{"user": name, "key": key})
	for attempt := 0; attempt < 2; attempt++ {
		record, err := records.Find(ctx, "studio", "preference", match)
		if err == nil {
			id, idErr := recordIDValue(record)
			if idErr != nil {
				return idErr
			}
			_, err = records.Update(ctx, "studio", "preference", id, dygo.RecordInput{"value": value})
			return err
		}
		var problem db.RecordError
		if !errors.As(err, &problem) || problem.Code != db.RecordErrorNotFound {
			return err
		}
		create := input(map[string]any{"user": name, "key": key})
		create["value"] = value
		if err := s.ensureCapacity(ctx, actor, "preference"); err != nil {
			return err
		}
		_, err = records.Create(ctx, "studio", "preference", create)
		if err == nil {
			return nil
		}
		if !errors.As(err, &problem) || problem.Code != db.RecordErrorConstraintViolation || attempt == 1 {
			return err
		}
	}
	return nil
}

func (s Store) DeletePreference(ctx context.Context, actor dygo.Actor, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	rows, err := s.list(ctx, actor, "preference", dygo.RecordFilter{Field: "key", Operator: "eq", Value: key})
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	id, err := recordIDValue(rows[0])
	if err != nil {
		return err
	}
	return s.records(actor).Delete(ctx, "studio", "preference", id)
}

func (s Store) SavedFilters(ctx context.Context, actor dygo.Actor, entity string) ([]SavedFilter, error) {
	if err := s.validate(ctx, actor, entity, []Filter{}); err != nil {
		return nil, err
	}
	rows, err := s.list(ctx, actor, "saved-filter", dygo.RecordFilter{Field: "entity", Operator: "eq", Value: entity})
	if err != nil {
		return nil, err
	}
	result := []SavedFilter{}
	for _, row := range rows {
		item, err := savedFilter(row)
		if err != nil {
			return nil, err
		}
		if err := s.validate(ctx, actor, item.Entity, item.Filters); err != nil {
			var problem db.RecordError
			if errors.As(err, &problem) && (problem.Code == db.RecordErrorInvalidRequest || problem.Code == db.RecordErrorValidation || problem.Code == db.RecordErrorPermissionDenied || problem.Code == db.RecordErrorNotFound) {
				item.ValidationError = problem.Message
			} else {
				return nil, err
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func savedFilter(record dygo.Record) (SavedFilter, error) {
	id, err := recordIDValue(record)
	if err != nil {
		return SavedFilter{}, err
	}
	entity, err := recordStringValue(record, "entity")
	if err != nil {
		return SavedFilter{}, err
	}
	label, err := recordStringValue(record, "label")
	if err != nil {
		return SavedFilter{}, err
	}
	item := SavedFilter{ID: id, Entity: entity, Label: label}
	data, err := json.Marshal(record["filters"])
	if err == nil {
		err = json.Unmarshal(data, &item.Filters)
	}
	return item, err
}

func validateLabel(label string) error {
	if strings.TrimSpace(label) == "" || len(label) > 160 {
		return invalid("label must contain 1 to 160 bytes")
	}
	return nil
}

func (s Store) CreateSavedFilter(ctx context.Context, actor dygo.Actor, entity, label string, filters []Filter) (SavedFilter, error) {
	label = strings.TrimSpace(label)
	if err := validateLabel(label); err != nil {
		return SavedFilter{}, err
	}
	if err := s.validate(ctx, actor, entity, filters); err != nil {
		return SavedFilter{}, err
	}
	name, err := s.ownerName(ctx, actor)
	if err != nil {
		return SavedFilter{}, err
	}
	if err := s.ensureCapacity(ctx, actor, "saved-filter"); err != nil {
		return SavedFilter{}, err
	}
	record, err := s.records(actor).Create(ctx, "studio", "saved-filter", input(map[string]any{"user": name, "entity": entity, "label": label, "filters": filters}))
	if err != nil {
		return SavedFilter{}, err
	}
	return savedFilter(record)
}

func (s Store) mutateSavedFilter(ctx context.Context, actor dygo.Actor, id int64, mutate func(context.Context, dygo.RecordData, dygo.Record) error) error {
	if err := requireActor(actor); err != nil {
		return err
	}
	ownerField, err := s.privateOwnerField(ctx, "saved-filter")
	if err != nil {
		return err
	}
	return s.records(actor).Transaction(ctx, func(ctx context.Context, records dygo.RecordData) error {
		locked, err := records.Lock(ctx, "studio", "saved-filter", dygo.RecordListParams{Limit: 1, Filters: []dygo.RecordFilter{owner(actor, ownerField), {Field: "id", Operator: "eq", Value: fmtID(id)}}})
		if err != nil {
			return err
		}
		if len(locked.Records) == 0 {
			return db.RecordError{Code: db.RecordErrorNotFound, Message: "saved filter not found"}
		}
		return mutate(ctx, records, locked.Records[0])
	})
}

func (s Store) UpdateSavedFilter(ctx context.Context, actor dygo.Actor, id int64, label *string, filters *[]Filter) (SavedFilter, error) {
	var result SavedFilter
	err := s.mutateSavedFilter(ctx, actor, id, func(ctx context.Context, records dygo.RecordData, record dygo.Record) error {
		item, err := savedFilter(record)
		if err != nil {
			return err
		}
		changes := map[string]any{}
		if label != nil {
			item.Label = strings.TrimSpace(*label)
			if err := validateLabel(item.Label); err != nil {
				return err
			}
			changes["label"] = item.Label
		}
		if filters != nil {
			item.Filters = *filters
			changes["filters"] = item.Filters
		}
		if len(changes) == 0 {
			return invalid("label or filters is required")
		}
		if err := s.validate(ctx, actor, item.Entity, item.Filters); err != nil {
			return err
		}
		record, err = records.Update(ctx, "studio", "saved-filter", id, input(changes))
		if err != nil {
			return err
		}
		result, err = savedFilter(record)
		return err
	})
	return result, err
}

func (s Store) DeleteSavedFilter(ctx context.Context, actor dygo.Actor, id int64) error {
	return s.mutateSavedFilter(ctx, actor, id, func(ctx context.Context, records dygo.RecordData, _ dygo.Record) error {
		return records.Delete(ctx, "studio", "saved-filter", id)
	})
}

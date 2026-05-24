package db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ActivityReader reads scoped Record Activity from Core activity records.
type ActivityReader struct {
	queryer ActivityQueryer
}

// ActivityQueryer is the database behavior needed by the Activity reader.
type ActivityQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// ActivityActor is the optional user that caused an Activity entry.
type ActivityActor struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full-name"`
}

// ActivityEntry is one append-only Record history entry.
type ActivityEntry struct {
	ID        int64          `json:"id"`
	CreatedAt string         `json:"created-at"`
	Entity    string         `json:"entity"`
	RecordID  int64          `json:"record-id"`
	Kind      string         `json:"kind"`
	Operation string         `json:"operation"`
	Status    string         `json:"status"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Actor     *ActivityActor `json:"actor"`
	Changes   any            `json:"changes"`
	Snapshot  any            `json:"snapshot"`
	Details   any            `json:"details"`
}

// ActivityListResult is a page of Activity entries.
type ActivityListResult struct {
	Activities []ActivityEntry
	Limit      int
	Offset     int
	Count      int
}

// NewActivityReader returns an Activity reader backed by queryer.
func NewActivityReader(queryer ActivityQueryer) ActivityReader {
	return ActivityReader{queryer: queryer}
}

// ListRecordActivity returns Activity entries for one Entity Record ID.
func (r ActivityReader) ListRecordActivity(ctx context.Context, entity string, recordID int64, params RecordListParams) (ActivityListResult, error) {
	if err := r.requireQueryer(); err != nil {
		return ActivityListResult{}, err
	}
	if recordID <= 0 {
		return ActivityListResult{}, invalidRecordIDError(entity)
	}
	params, err := normalizeRecordListParams(params)
	if err != nil {
		return ActivityListResult{}, err
	}
	entityID, err := r.entityID(ctx, entity)
	if err != nil {
		return ActivityListResult{}, err
	}
	rows, err := r.queryer.Query(ctx, `
SELECT
	a.id,
	a.created_at,
	e.slug,
	a.record_id,
	a.kind,
	a.operation,
	a.status,
	a.title,
	COALESCE(a.message, ''),
	a.changes,
	a.snapshot,
	a.details,
	COALESCE(u.id, 0),
	COALESCE(u.email, ''),
	COALESCE(u.full_name, '')
FROM "activity" a
JOIN "entity" e ON e.id = a.entity_id
LEFT JOIN "user" u ON u.id = a.actor_id
WHERE a.entity_id = $1 AND a.record_id = $2
ORDER BY a.created_at DESC, a.id DESC
LIMIT $3 OFFSET $4`, entityID, recordID, params.Limit, params.Offset)
	if err != nil {
		return ActivityListResult{}, classifyRecordDBError(err, "activity")
	}
	defer rows.Close()

	activities := []ActivityEntry{}
	for rows.Next() {
		var entry ActivityEntry
		var changes []byte
		var snapshot []byte
		var details []byte
		var actorID int64
		var actorEmail string
		var actorFullName string
		var createdAt time.Time
		if err := rows.Scan(
			&entry.ID,
			&createdAt,
			&entry.Entity,
			&entry.RecordID,
			&entry.Kind,
			&entry.Operation,
			&entry.Status,
			&entry.Title,
			&entry.Message,
			&changes,
			&snapshot,
			&details,
			&actorID,
			&actorEmail,
			&actorFullName,
		); err != nil {
			return ActivityListResult{}, recordError(RecordErrorInternal, "scan activity row failed", map[string]any{"entity": entity, "id": recordID}, err)
		}
		entry.CreatedAt = normalizeDatetimeValue(createdAt)
		entry.Changes, err = decodeActivityJSON(changes)
		if err != nil {
			return ActivityListResult{}, err
		}
		entry.Snapshot, err = decodeActivityJSON(snapshot)
		if err != nil {
			return ActivityListResult{}, err
		}
		entry.Details, err = decodeActivityJSON(details)
		if err != nil {
			return ActivityListResult{}, err
		}
		if actorID > 0 {
			entry.Actor = &ActivityActor{ID: actorID, Email: actorEmail, FullName: actorFullName}
		}
		activities = append(activities, entry)
	}
	if err := rows.Err(); err != nil {
		return ActivityListResult{}, classifyRecordDBError(err, "activity")
	}
	return ActivityListResult{Activities: activities, Limit: params.Limit, Offset: params.Offset, Count: len(activities)}, nil
}

func (r ActivityReader) entityID(ctx context.Context, entity string) (int64, error) {
	var id int64
	err := r.queryer.QueryRow(ctx, `SELECT id FROM "entity" WHERE slug = $1`, entity).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, recordError(RecordErrorNotFound, "entity not found", map[string]any{"entity": entity}, err)
	}
	if err != nil {
		return 0, classifyRecordDBError(err, entity)
	}
	return id, nil
}

func (r ActivityReader) requireQueryer() error {
	if r.queryer == nil {
		return recordError(RecordErrorInternal, "activity queryer is required", nil, nil)
	}
	return nil
}

func decodeActivityJSON(raw []byte) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, recordError(RecordErrorInternal, "decode activity JSON failed", nil, err)
	}
	return value, nil
}

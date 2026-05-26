package db

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hapyco/dygo/internal/auth"
	"github.com/hapyco/dygo/internal/corevalues"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRecordStoreListRecords(t *testing.T) {
	pkt := time.FixedZone("PKT", 5*60*60)
	createdAt := time.Date(2026, 5, 7, 12, 0, 0, 637443000, pkt)
	updatedAt := time.Date(2026, 5, 7, 12, 30, 0, 0, pkt)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(1), "a@example.com", createdAt, updatedAt, "a@example.com", "A User", true},
		{int64(2), "b@example.com", createdAt, updatedAt, "b@example.com", "B User", false},
	}))

	result, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{})
	if err != nil {
		t.Fatalf("ListRecords() error = %v, want nil", err)
	}
	if result.Limit != 20 || result.Offset != 0 || result.Count != 2 {
		t.Fatalf("ListRecords() result = %+v, want default pagination and two records", result)
	}
	if result.Records[0]["email"] != "a@example.com" || result.Records[0]["full-name"] != "A User" {
		t.Fatalf("ListRecords() first record = %+v, want metadata field names", result.Records[0])
	}
	if result.Records[0]["created-at"] != "2026-05-07T07:00:00.637443Z" || result.Records[0]["updated-at"] != "2026-05-07T07:30:00Z" {
		t.Fatalf("ListRecords() timestamps = created:%#v updated:%#v, want UTC RFC3339Nano", result.Records[0]["created-at"], result.Records[0]["updated-at"])
	}
	if _, ok := result.Records[0]["password"]; ok {
		t.Fatalf("ListRecords() returned password field: %+v", result.Records[0])
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	if !strings.Contains(lastQuery, `FROM "user"`) || !strings.Contains(lastQuery, `ORDER BY "id" ASC LIMIT $1 OFFSET $2`) {
		t.Fatalf("list query = %q, want deterministic paginated query", lastQuery)
	}
	if got := queryer.args[len(queryer.args)-1]; len(got) != 2 || got[0] != 20 || got[1] != 0 {
		t.Fatalf("list args = %#v, want default limit/offset", got)
	}
}

func TestRecordStoreListRecordsNormalizesAuthoredDatetime(t *testing.T) {
	pkt := time.FixedZone("PKT", 5*60*60)
	createdAt := time.Date(2026, 5, 7, 12, 0, 0, 0, pkt)
	updatedAt := time.Date(2026, 5, 7, 12, 30, 0, 0, pkt)
	startsAt := time.Date(2026, 5, 7, 14, 29, 14, 123456000, pkt)
	queryer := newEventRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(1), "launch", createdAt, updatedAt, startsAt},
	}))

	result, err := NewRecordStore(queryer).ListRecords(context.Background(), "event", RecordListParams{})
	if err != nil {
		t.Fatalf("ListRecords(event) error = %v, want nil", err)
	}
	if result.Records[0]["starts-at"] != "2026-05-07T09:29:14.123456Z" {
		t.Fatalf("ListRecords(event) starts-at = %#v, want UTC RFC3339Nano", result.Records[0]["starts-at"])
	}
}

func TestRecordStoreListRecordsResolvesLinkFieldsToTargetNames(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newActivityRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(1), "activity-1", now, now, "record", "create", "success", "core.user", int64(7), "admin@example.com", "Created User", nil, nil, nil, nil},
	}))

	result, err := NewRecordStore(queryer).ListRecords(context.Background(), "activity", RecordListParams{})
	if err != nil {
		t.Fatalf("ListRecords(activity) error = %v, want nil", err)
	}
	if result.Count != 1 {
		t.Fatalf("ListRecords(activity) count = %d, want 1", result.Count)
	}
	record := result.Records[0]
	if record["entity"] != "core.user" || record["actor"] != "admin@example.com" {
		t.Fatalf("ListRecords(activity) link values = entity:%#v actor:%#v, want target names", record["entity"], record["actor"])
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{
		`(SELECT "name" FROM "entity" WHERE "id" = "_dygo_record"."entity_id")`,
		`(SELECT "name" FROM "user" WHERE "id" = "_dygo_record"."actor_id")`,
	} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("list query = %q, want link name projection %q", lastQuery, want)
		}
	}
}

func TestRecordStoreListRecordsFiltersResolveLinkNamesToStoredIDs(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newActivityRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(1), "activity-1", now, now, "record", "create", "success", "core.user", int64(7), "admin@example.com", "Created User", nil, nil, nil, nil},
	}))

	_, err := NewRecordStore(queryer).ListRecords(context.Background(), "activity", RecordListParams{
		Filters: []RecordFilter{{Field: "entity", Value: "core.user"}},
	})
	if err != nil {
		t.Fatalf("ListRecords(activity link filter) error = %v, want nil", err)
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	if !strings.Contains(lastQuery, `WHERE "entity_id" = $1::bigint`) {
		t.Fatalf("list query = %q, want link storage filter", lastQuery)
	}
	if got := queryer.args[len(queryer.args)-1]; !reflect.DeepEqual(got, []any{int64(10), 20, 0}) {
		t.Fatalf("list args = %#v, want resolved link id plus pagination", got)
	}
}

func TestRecordStoreListRecordsFiltersAndSorts(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(1), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	result, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{
		Filters: []RecordFilter{
			{Field: "enabled", Value: "true"},
			{Field: "email", Value: "a@example.com"},
		},
		Sort: []RecordSort{
			{Field: "full-name", Desc: true},
			{Field: "created-at"},
		},
	})
	if err != nil {
		t.Fatalf("ListRecords(filters/sort) error = %v, want nil", err)
	}
	if result.Count != 1 {
		t.Fatalf("ListRecords(filters/sort) count = %d, want 1", result.Count)
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{
		`WHERE "email" = $1 AND "enabled" = $2::boolean`,
		`ORDER BY "full_name" DESC, "created_at" ASC, "id" ASC`,
		`LIMIT $3 OFFSET $4`,
	} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("list query = %q, want %q", lastQuery, want)
		}
	}
	if got := queryer.args[len(queryer.args)-1]; !reflect.DeepEqual(got, []any{"a@example.com", true, 20, 0}) {
		t.Fatalf("list args = %#v, want filter args then pagination", got)
	}
}

func TestRecordStoreListRecordsSupportsSystemFiltersAndSortTieBreaker(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	_, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{
		Filters: []RecordFilter{
			{Field: "id", Value: "7"},
			{Field: "created-at", Value: "2026-05-07T17:00:00+05:00"},
		},
		Sort: []RecordSort{{Field: "updated-at", Desc: true}},
	})
	if err != nil {
		t.Fatalf("ListRecords(system filters) error = %v, want nil", err)
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{
		`WHERE "created_at" = $1::timestamptz AND "id" = $2::bigint`,
		`ORDER BY "updated_at" DESC, "id" ASC`,
	} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("list query = %q, want %q", lastQuery, want)
		}
	}
	if got := queryer.args[len(queryer.args)-1]; !reflect.DeepEqual(got, []any{"2026-05-07T17:00:00+05:00", int64(7), 20, 0}) {
		t.Fatalf("list args = %#v, want system filter args", got)
	}
}

func TestRecordStoreListRecordsSortByIDSkipsTieBreaker(t *testing.T) {
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows(nil))

	_, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{Sort: []RecordSort{{Field: "id", Desc: true}}})
	if err != nil {
		t.Fatalf("ListRecords(sort id) error = %v, want nil", err)
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	if !strings.Contains(lastQuery, `ORDER BY "id" DESC LIMIT $1 OFFSET $2`) {
		t.Fatalf("list query = %q, want id sort without tie-breaker", lastQuery)
	}
}

func TestRecordStoreListRecordsByIdentityHonorsFiltersAndSort(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newLeadRecordQueryer()
	queryer.row = newFakeRow(int64(20), "crm.lead", "lead", "crm-lead", "Lead", "Sales lead", "contact", false, false, false, []byte(`{"strategy":"random","length":16}`), "crm", "CRM")
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "New"},
	}))

	_, err := NewRecordStore(queryer).ListRecordsByIdentity(context.Background(), "crm", "lead", RecordListParams{
		Filters: []RecordFilter{{Field: "status", Value: "New"}},
		Sort:    []RecordSort{{Field: "status"}},
	})
	if err != nil {
		t.Fatalf("ListRecordsByIdentity() error = %v, want nil", err)
	}
	if !strings.Contains(queryer.rowSQL[0], `WHERE a.name = $1 AND e.key = $2`) {
		t.Fatalf("metadata query = %q, want app/entity lookup", queryer.rowSQL[0])
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{`FROM "crm_lead"`, `WHERE "status" = $1`, `ORDER BY "status" ASC, "id" ASC`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("list query = %q, want %q", lastQuery, want)
		}
	}
}

func TestRecordStoreGetRecord(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	record, err := NewRecordStore(queryer).GetRecord(context.Background(), "user", 7)
	if err != nil {
		t.Fatalf("GetRecord() error = %v, want nil", err)
	}
	if record["id"] != int64(7) || record["email"] != "a@example.com" {
		t.Fatalf("GetRecord() = %+v, want record by id", record)
	}
	if _, ok := record["password"]; ok {
		t.Fatalf("GetRecord() returned password field: %+v", record)
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	if !strings.Contains(lastQuery, `WHERE "id" = $1`) {
		t.Fatalf("get query = %q, want id predicate", lastQuery)
	}
}

func TestRecordStoreGetRecordByIdentityUsesAppEntityLookup(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newLeadRecordQueryer()
	queryer.row = newFakeRow(int64(20), "crm.lead", "lead", "crm-lead", "Lead", "Sales lead", "contact", false, false, false, []byte(`{"strategy":"random","length":16}`), "crm", "CRM")
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "New"},
	}))

	record, err := NewRecordStore(queryer).GetRecordByIdentity(context.Background(), "crm", "lead", 7)
	if err != nil {
		t.Fatalf("GetRecordByIdentity() error = %v, want nil", err)
	}
	if record["id"] != int64(7) || record["status"] != "New" {
		t.Fatalf("GetRecordByIdentity() = %+v, want crm lead record", record)
	}
	if !strings.Contains(queryer.rowSQL[0], `WHERE a.name = $1 AND e.key = $2`) {
		t.Fatalf("metadata query = %q, want app/entity lookup", queryer.rowSQL[0])
	}
	if !reflect.DeepEqual(queryer.rowArgs[0], []any{"crm", "lead"}) {
		t.Fatalf("metadata args = %#v, want crm/lead", queryer.rowArgs[0])
	}
	lastQuery, _ := lastQueryContaining(t, queryer, `FROM "crm_lead"`)
	if !strings.Contains(lastQuery, `FROM "crm_lead"`) {
		t.Fatalf("get query = %q, want app-scoped storage table", lastQuery)
	}
}

func TestRecordStoreRouteSlugMethodsKeepUsingRouteSlugLookup(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newLeadRecordQueryer()
	queryer.row = newFakeRow(int64(20), "crm.lead", "lead", "crm-lead", "Lead", "Sales lead", "contact", false, false, false, []byte(`{"strategy":"random","length":16}`), "crm", "CRM")
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "New"},
	}))

	_, err := NewRecordStore(queryer).GetRecord(context.Background(), "crm-lead", 7)
	if err != nil {
		t.Fatalf("GetRecord() error = %v, want nil", err)
	}
	if !strings.Contains(queryer.rowSQL[0], `WHERE e.slug = $1`) {
		t.Fatalf("metadata query = %q, want route slug lookup", queryer.rowSQL[0])
	}
	if !reflect.DeepEqual(queryer.rowArgs[0], []any{"crm-lead"}) {
		t.Fatalf("metadata args = %#v, want crm-lead route slug", queryer.rowArgs[0])
	}
}

func TestRecordStoreFindRecord(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	record, err := NewRecordStore(queryer).FindRecord(context.Background(), "user", recordInput(map[string]string{
		"email": `"a@example.com"`,
	}))
	if err != nil {
		t.Fatalf("FindRecord() error = %v, want nil", err)
	}
	if record["id"] != int64(7) || record["email"] != "a@example.com" {
		t.Fatalf("FindRecord() = %+v, want matched record", record)
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{`FROM "user"`, `WHERE "email" = $1`, `ORDER BY "id" ASC LIMIT 2`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("find query = %q, want %q", lastQuery, want)
		}
	}
}

func TestRecordStoreFindRecordAmbiguous(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
		{int64(8), "a@example.com", now, now, "a@example.com", "Another User", true},
	}))

	_, err := NewRecordStore(queryer).FindRecord(context.Background(), "user", recordInput(map[string]string{
		"email": `"a@example.com"`,
	}))
	assertRecordError(t, err, RecordErrorValidation, "")
}

func TestRecordStoreGetSingleRecord(t *testing.T) {
	queryer := newSingleSettingsRecordQueryer()

	record, err := NewRecordStore(queryer).GetSingleRecord(context.Background(), "invoice-settings")
	if err != nil {
		t.Fatalf("GetSingleRecord() error = %v, want nil", err)
	}
	if record["name"] != "invoice-settings" || record["default-due-days"] != int64(30) {
		t.Fatalf("GetSingleRecord() = %+v, want singleton settings", record)
	}
	if !strings.Contains(queryer.queries[3], `WHERE "name" = $1`) {
		t.Fatalf("GetSingleRecord() query = %q, want fixed name lookup", queryer.queries[3])
	}
}

func TestRecordStoreGetSingleRecordRejectsNormalEntity(t *testing.T) {
	_, err := NewRecordStore(newUserRecordQueryer()).GetSingleRecord(context.Background(), "user")
	assertRecordError(t, err, RecordErrorInvalidRequest, "")
}

func TestRecordStoreUpdateSingleRecordWritesActivity(t *testing.T) {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	queryer := newSingleSettingsRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "invoice-settings", now, now, int64(30)},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "invoice-settings", now, now, int64(45)},
	}))

	record, err := NewRecordStore(queryer).UpdateSingleRecord(context.Background(), "invoice-settings", recordInput(map[string]string{
		"default-due-days": "45",
	}))
	if err != nil {
		t.Fatalf("UpdateSingleRecord() error = %v, want nil", err)
	}
	if record["name"] != "invoice-settings" || record["default-due-days"] != int64(45) {
		t.Fatalf("UpdateSingleRecord() = %+v, want updated singleton settings", record)
	}
	args := activityArgs(t, queryer)
	if args[1] != corevalues.ActivityOperationUpdate || args[4] != int64(7) {
		t.Fatalf("activity args = %#v, want update for singleton record 7", args)
	}
	changes := decodeActivityList(t, args[8])
	if len(changes) != 1 || changes[0]["field"] != "default-due-days" || changes[0]["old"] != float64(30) || changes[0]["new"] != float64(45) {
		t.Fatalf("activity changes = %#v, want default-due-days diff", changes)
	}
}

func TestRecordStoreRejectsSingleEntityCollectionMutations(t *testing.T) {
	tests := []struct {
		name string
		run  func(RecordStore) error
	}{
		{
			name: "list",
			run: func(store RecordStore) error {
				_, err := store.ListRecords(context.Background(), "invoice-settings", RecordListParams{})
				return err
			},
		},
		{
			name: "create",
			run: func(store RecordStore) error {
				_, err := store.CreateRecord(context.Background(), "invoice-settings", recordInput(map[string]string{"default-due-days": "30"}))
				return err
			},
		},
		{
			name: "delete",
			run: func(store RecordStore) error {
				return store.DeleteRecord(context.Background(), "invoice-settings", 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(NewRecordStore(newSingleSettingsRecordQueryer()))
			assertRecordError(t, err, RecordErrorInvalidRequest, "")
		})
	}
}

func TestRecordStoreRejectsCollectionEntityNormalEndpoints(t *testing.T) {
	tests := []struct {
		name string
		run  func(RecordStore) error
	}{
		{
			name: "list",
			run: func(store RecordStore) error {
				_, err := store.ListRecords(context.Background(), "invoice-item", RecordListParams{})
				return err
			},
		},
		{
			name: "get",
			run: func(store RecordStore) error {
				_, err := store.GetRecord(context.Background(), "invoice-item", 1)
				return err
			},
		},
		{
			name: "find",
			run: func(store RecordStore) error {
				_, err := store.FindRecord(context.Background(), "invoice-item", recordInput(map[string]string{"item-code": `"SKU-1"`}))
				return err
			},
		},
		{
			name: "create",
			run: func(store RecordStore) error {
				_, err := store.CreateRecord(context.Background(), "invoice-item", recordInput(map[string]string{"item-code": `"SKU-1"`}))
				return err
			},
		},
		{
			name: "delete",
			run: func(store RecordStore) error {
				return store.DeleteRecord(context.Background(), "invoice-item", 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(NewRecordStore(newCollectionRecordQueryer()))
			assertRecordError(t, err, RecordErrorInvalidRequest, "")
		})
	}
}

func TestRecordStoreRejectsSystemEntityMutations(t *testing.T) {
	tests := []struct {
		name    string
		queryer *fakeRecordQueryer
		run     func(RecordStore) error
	}{
		{
			name:    "create",
			queryer: newSystemUserRecordQueryer(),
			run: func(store RecordStore) error {
				_, err := store.CreateRecord(context.Background(), "user", recordInput(map[string]string{"email": `"a@example.com"`}))
				return err
			},
		},
		{
			name:    "create by identity",
			queryer: newSystemUserRecordQueryer(),
			run: func(store RecordStore) error {
				_, err := store.CreateRecordByIdentity(context.Background(), "core", "user", recordInput(map[string]string{"email": `"a@example.com"`}))
				return err
			},
		},
		{
			name:    "update",
			queryer: newSystemUserRecordQueryer(),
			run: func(store RecordStore) error {
				_, err := store.UpdateRecord(context.Background(), "user", 7, recordInput(map[string]string{"full-name": `"A User"`}))
				return err
			},
		},
		{
			name:    "update by identity",
			queryer: newSystemUserRecordQueryer(),
			run: func(store RecordStore) error {
				_, err := store.UpdateRecordByIdentity(context.Background(), "core", "user", 7, recordInput(map[string]string{"full-name": `"A User"`}))
				return err
			},
		},
		{
			name:    "update single",
			queryer: newSystemSingleSettingsRecordQueryer(),
			run: func(store RecordStore) error {
				_, err := store.UpdateSingleRecord(context.Background(), "invoice-settings", recordInput(map[string]string{"default-due-days": "45"}))
				return err
			},
		},
		{
			name:    "delete",
			queryer: newSystemUserRecordQueryer(),
			run: func(store RecordStore) error {
				return store.DeleteRecord(context.Background(), "user", 7)
			},
		},
		{
			name:    "delete by identity",
			queryer: newSystemUserRecordQueryer(),
			run: func(store RecordStore) error {
				return store.DeleteRecordByIdentity(context.Background(), "core", "user", 7)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(NewRecordStore(tt.queryer))
			assertRecordError(t, err, RecordErrorInvalidRequest, "")
		})
	}
}

func TestRecordStoreReadsSystemEntityRecords(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)

	t.Run("list", func(t *testing.T) {
		queryer := newSystemUserRecordQueryer()
		queryer.rows = append(queryer.rows, newFakeRows([][]any{
			{int64(7), "a@example.com", now, now, "a@example.com", "A User", true, int64(1)},
		}))

		result, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{})
		if err != nil {
			t.Fatalf("ListRecords(system) error = %v, want nil", err)
		}
		if result.Count != 1 || result.Records[0]["email"] != "a@example.com" {
			t.Fatalf("ListRecords(system) = %+v, want user record", result)
		}
	})

	t.Run("get", func(t *testing.T) {
		queryer := newSystemUserRecordQueryer()
		queryer.rows = append(queryer.rows, newFakeRows([][]any{
			{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
		}))

		record, err := NewRecordStore(queryer).GetRecord(context.Background(), "user", 7)
		if err != nil {
			t.Fatalf("GetRecord(system) error = %v, want nil", err)
		}
		if record["email"] != "a@example.com" {
			t.Fatalf("GetRecord(system) = %+v, want user record", record)
		}
	})

	t.Run("find", func(t *testing.T) {
		queryer := newSystemUserRecordQueryer()
		queryer.rows = append(queryer.rows, newFakeRows([][]any{
			{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
		}))

		record, err := NewRecordStore(queryer).FindRecord(context.Background(), "user", recordInput(map[string]string{
			"email": `"a@example.com"`,
		}))
		if err != nil {
			t.Fatalf("FindRecord(system) error = %v, want nil", err)
		}
		if record["email"] != "a@example.com" {
			t.Fatalf("FindRecord(system) = %+v, want user record", record)
		}
	})
}

func TestSystemRecordWriterCanMutateSystemEntity(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newSystemUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	record, err := NewSystemRecordWriter(queryer).InsertReturningByIdentity(context.Background(), "core", "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"A User"`,
	}), SystemMutationSilent)
	if err != nil {
		t.Fatalf("InsertReturningByIdentity(system) error = %v, want nil", err)
	}
	if record["email"] != "a@example.com" {
		t.Fatalf("InsertReturningByIdentity(system) = %+v, want user record", record)
	}
	if len(queryer.execSQL) != 0 {
		t.Fatalf("exec SQL = %#v, want no activity insert for silent system mutation", queryer.execSQL)
	}
}

func TestRecordStoreCreateRecord(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	record, err := NewRecordStore(queryer).CreateRecord(context.Background(), "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"A User"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	if record["id"] != int64(7) || record["enabled"] != true {
		t.Fatalf("CreateRecord() = %+v, want returned record", record)
	}
	if record["name"] != "a@example.com" {
		t.Fatalf("CreateRecord() name = %v, want email naming source", record["name"])
	}
	lastQuery, args := lastQueryContaining(t, queryer, `INSERT INTO "user"`)
	for _, want := range []string{`INSERT INTO "user"`, `"email", "full_name"`, `"name"`, `RETURNING "_dygo_record"."id", "_dygo_record"."name", "_dygo_record"."created_at", "_dygo_record"."updated_at"`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("create query = %q, want %q", lastQuery, want)
		}
	}
	if args[len(args)-1] != "a@example.com" {
		t.Fatalf("CreateRecord() name arg = %#v, want source email", args[len(args)-1])
	}
}

func TestRecordStoreCreateRecordGeneratesRandomName(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newLeadRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "generated-name", now, now, "New"},
	}))

	_, err := NewRecordStore(queryer).CreateRecord(context.Background(), "lead", recordInput(map[string]string{
		"status": `"New"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	_, args := lastQueryContaining(t, queryer, `INSERT INTO "crm_lead"`)
	name, ok := args[len(args)-1].(string)
	if !ok {
		t.Fatalf("random name arg type = %T, want string", args[len(args)-1])
	}
	if len(name) != 16 {
		t.Fatalf("random name length = %d, want 16", len(name))
	}
}

func TestRecordStoreCreateRecordWithCollectionRows(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newLeadRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "New"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(10), "contact-10", now, now, "a@example.com", "A Contact"},
	}))

	record, err := NewRecordStore(queryer).CreateRecord(context.Background(), "lead", recordInput(map[string]string{
		"status":   `"New"`,
		"contacts": `[{"email":"a@example.com","full-name":"A Contact"}]`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord(collection) error = %v, want nil", err)
	}
	contacts, ok := record["contacts"].([]Record)
	if !ok || len(contacts) != 1 {
		t.Fatalf("CreateRecord(collection) contacts = %#v, want one child row", record["contacts"])
	}
	if contacts[0]["id"] != int64(10) || contacts[0]["email"] != "a@example.com" {
		t.Fatalf("CreateRecord(collection) child = %#v, want saved child row", contacts[0])
	}
	childSQL, childArgs := lastExecContaining(t, queryer, `INSERT INTO "crm_lead_contact"`)
	for _, want := range []string{`"email"`, `"full_name"`, `"name"`, `"parent_entity_id"`, `"parent_record_id"`, `"parent_field_id"`, `"ordinal"`} {
		if !strings.Contains(childSQL, want) {
			t.Fatalf("child insert SQL = %q, want %q", childSQL, want)
		}
	}
	if tail := childArgs[len(childArgs)-4:]; !reflect.DeepEqual(tail, []any{int64(20), int64(7), int64(2), int64(1)}) {
		t.Fatalf("child insert args = %#v, want parent entity 20, parent record 7, parent field 2, ordinal 1", childArgs)
	}
}

func TestRecordStoreCreateRecordGeneratesTemplateName(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newTemplateRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "T-New-A1", now, now, "A1", "New"},
	}))

	record, err := NewRecordStore(queryer).CreateRecord(context.Background(), "ticket", recordInput(map[string]string{
		"code":   `"A1"`,
		"status": `"New"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	if record["name"] != "T-New-A1" {
		t.Fatalf("CreateRecord() name = %v, want template name", record["name"])
	}
	_, args := lastQueryContaining(t, queryer, `INSERT INTO "support_ticket"`)
	if args[len(args)-1] != "T-New-A1" {
		t.Fatalf("CreateRecord() name arg = %#v, want template name", args[len(args)-1])
	}
}

func TestRecordStoreCreateRecordResolvesLinkNamesToStoredIDs(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newActivityRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(1), "activity-1", now, now, "record", "create", "success", "core.user", nil, "admin@example.com", "Created User", nil, nil, nil, nil},
	}))

	record, err := NewRecordStore(queryer).CreateRecord(context.Background(), "activity", recordInput(map[string]string{
		"actor":     `"admin@example.com"`,
		"entity":    `"core.user"`,
		"kind":      `"record"`,
		"operation": `"create"`,
		"status":    `"success"`,
		"title":     `"Created User"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord(activity links) error = %v, want nil", err)
	}
	if record["entity"] != "core.user" || record["actor"] != "admin@example.com" {
		t.Fatalf("CreateRecord(activity links) = %+v, want public link names", record)
	}
	lastQuery, args := lastQueryContaining(t, queryer, `INSERT INTO "activity"`)
	for _, want := range []string{`"actor_id", "entity_id"`, `(SELECT "name" FROM "entity" WHERE "id" = "_dygo_record"."entity_id")`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("create query = %q, want %q", lastQuery, want)
		}
	}
	if len(args) < 2 || args[0] != int64(99) || args[1] != int64(10) {
		t.Fatalf("create args = %#v, want actor/entity link names resolved to ids", args)
	}
}

func TestRecordStoreCreateRecordRejectsNumericLinkInput(t *testing.T) {
	_, err := NewRecordStore(newActivityRecordQueryer()).CreateRecord(context.Background(), "activity", recordInput(map[string]string{
		"actor":     `99`,
		"entity":    `"core.user"`,
		"kind":      `"record"`,
		"operation": `"create"`,
		"status":    `"success"`,
		"title":     `"Created User"`,
	}))
	assertRecordError(t, err, RecordErrorValidation, "actor")
	if err == nil || !strings.Contains(err.Error(), "link field must be a record name") {
		t.Fatalf("CreateRecord(numeric link) error = %v, want link name validation", err)
	}
}

func TestRecordStoreUpdateRecord(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "Renamed User", true},
	}))

	record, err := NewRecordStore(queryer).UpdateRecord(context.Background(), "user", 7, recordInput(map[string]string{
		"full-name": `"Renamed User"`,
	}))
	if err != nil {
		t.Fatalf("UpdateRecord() error = %v, want nil", err)
	}
	if record["full-name"] != "Renamed User" {
		t.Fatalf("UpdateRecord() = %+v, want patched record", record)
	}
	lastQuery, _ := lastQueryContaining(t, queryer, `UPDATE "user"`)
	for _, want := range []string{`UPDATE "user" AS "_dygo_record" SET "full_name" = $1`, `"updated_at" = now()`, `WHERE "id" = $2`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("update query = %q, want %q", lastQuery, want)
		}
	}
}

func TestRecordStoreUpdateRecordWithCollectionRows(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newLeadRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "New"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(10), "contact-10", now, now, "old@example.com", "Old Contact"},
		{int64(11), "contact-11", now, now, "remove@example.com", "Remove Contact"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "Qualified"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{{int64(10)}, {int64(11)}}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(10), "contact-10", now, now, "updated@example.com", "Updated Contact"},
		{int64(12), "contact-12", now, now, "new@example.com", "New Contact"},
	}))
	queryer.execTags = []pgconn.CommandTag{
		pgconn.NewCommandTag("DELETE 1"),
		pgconn.NewCommandTag("UPDATE 1"),
		pgconn.NewCommandTag("INSERT 1"),
		pgconn.NewCommandTag("INSERT 1"),
	}

	record, err := NewRecordStore(queryer).UpdateRecord(context.Background(), "lead", 7, recordInput(map[string]string{
		"status":   `"Qualified"`,
		"contacts": `[{"id":10,"email":"updated@example.com","full-name":"Updated Contact"},{"email":"new@example.com","full-name":"New Contact"}]`,
	}))
	if err != nil {
		t.Fatalf("UpdateRecord(collection) error = %v, want nil", err)
	}
	contacts, ok := record["contacts"].([]Record)
	if !ok || len(contacts) != 2 {
		t.Fatalf("UpdateRecord(collection) contacts = %#v, want two child rows", record["contacts"])
	}
	if contacts[0]["id"] != int64(10) || contacts[1]["id"] != int64(12) {
		t.Fatalf("UpdateRecord(collection) child ids = %#v, want preserved then inserted ids", contacts)
	}
	deleteSQL, deleteArgs := lastExecContaining(t, queryer, `DELETE FROM "crm_lead_contact"`)
	if !strings.Contains(deleteSQL, `WHERE "parent_entity_id" = $1 AND "parent_record_id" = $2 AND "parent_field_id" = $3 AND "id" = $4`) || !reflect.DeepEqual(deleteArgs, []any{int64(20), int64(7), int64(2), int64(11)}) {
		t.Fatalf("child delete = %q %#v, want omitted row 11 deleted for parent entity 20 record 7 field 2", deleteSQL, deleteArgs)
	}
	updateSQL, updateArgs := lastExecContaining(t, queryer, `UPDATE "crm_lead_contact"`)
	for _, want := range []string{`"email" = $1`, `"full_name" = $2`, `"ordinal" = $3::bigint`, `WHERE "id" = $4 AND "parent_entity_id" = $5 AND "parent_record_id" = $6 AND "parent_field_id" = $7`} {
		if !strings.Contains(updateSQL, want) {
			t.Fatalf("child update SQL = %q, want %q", updateSQL, want)
		}
	}
	if !reflect.DeepEqual(updateArgs, []any{"updated@example.com", "Updated Contact", int64(1), int64(10), int64(20), int64(7), int64(2)}) {
		t.Fatalf("child update args = %#v, want update row 10 at ordinal 1 for parent entity 20 record 7 field 2", updateArgs)
	}
	insertSQL, insertArgs := lastExecContaining(t, queryer, `INSERT INTO "crm_lead_contact"`)
	if !strings.Contains(insertSQL, `"parent_entity_id", "parent_record_id", "parent_field_id", "ordinal"`) {
		t.Fatalf("child insert SQL = %q, want collection ownership columns", insertSQL)
	}
	if tail := insertArgs[len(insertArgs)-4:]; !reflect.DeepEqual(tail, []any{int64(20), int64(7), int64(2), int64(2)}) {
		t.Fatalf("child insert = %q %#v, want new row at ordinal 2 for parent entity 20 record 7 field 2", insertSQL, insertArgs)
	}
}

func TestRecordStoreUpdateCollectionRowAllowsRequiredWriteOnlyOmission(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newLeadRecordQueryer()
	queryer.collectionFieldRows = [][]any{
		metadataFieldRow("email", "Email", "email", true, false, false, nil, nil, 1, nil),
		metadataFieldRow("access-token", "Access Token", "password", true, false, false, nil, nil, 2, nil),
	}
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "New"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(10), "contact-10", now, now, "old@example.com"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "Qualified"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{{int64(10)}}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(10), "contact-10", now, now, "old@example.com"},
	}))
	queryer.execTags = []pgconn.CommandTag{
		pgconn.NewCommandTag("UPDATE 1"),
		pgconn.NewCommandTag("INSERT 1"),
	}

	_, err := NewRecordStore(queryer).UpdateRecord(context.Background(), "lead", 7, recordInput(map[string]string{
		"status":   `"Qualified"`,
		"contacts": `[{"id":10}]`,
	}))
	if err != nil {
		t.Fatalf("UpdateRecord(collection existing patch) error = %v, want nil", err)
	}
	updateSQL, updateArgs := lastExecContaining(t, queryer, `UPDATE "crm_lead_contact"`)
	for _, want := range []string{`"ordinal" = $1::bigint`, `"updated_at" = now()`, `WHERE "id" = $2 AND "parent_entity_id" = $3 AND "parent_record_id" = $4 AND "parent_field_id" = $5`} {
		if !strings.Contains(updateSQL, want) {
			t.Fatalf("child update SQL = %q, want %q", updateSQL, want)
		}
	}
	if !reflect.DeepEqual(updateArgs, []any{int64(1), int64(10), int64(20), int64(7), int64(2)}) {
		t.Fatalf("child update args = %#v, want reorder-only update row 10 at ordinal 1", updateArgs)
	}
}

func TestRecordStoreRejectsDuplicateCollectionRowIDs(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newLeadRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "New"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(10), "contact-10", now, now, "old@example.com", "Old Contact"},
	}))

	_, err := NewRecordStore(queryer).UpdateRecord(context.Background(), "lead", 7, recordInput(map[string]string{
		"contacts": `[{"id":10,"email":"one@example.com"},{"id":10,"email":"two@example.com"}]`,
	}))
	assertRecordError(t, err, RecordErrorValidation, "contacts")
	if err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("UpdateRecord(duplicate collection ids) error = %v, want duplicate id validation", err)
	}
}

func TestRecordStoreRejectsForeignCollectionRowIDs(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newLeadRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "New"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(10), "contact-10", now, now, "old@example.com", "Old Contact"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "New"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{{int64(10)}}))

	_, err := NewRecordStore(queryer).UpdateRecord(context.Background(), "lead", 7, recordInput(map[string]string{
		"contacts": `[{"id":99,"email":"foreign@example.com"}]`,
	}))
	assertRecordError(t, err, RecordErrorValidation, "contacts")
	if err == nil || !strings.Contains(err.Error(), "does not belong") {
		t.Fatalf("UpdateRecord(foreign collection id) error = %v, want ownership validation", err)
	}
}

func TestRecordStoreRejectsCollectionPayloadOverMaxRows(t *testing.T) {
	queryer := newLeadRecordQueryer()

	_, err := NewRecordStore(queryer).CreateRecord(context.Background(), "lead", recordInput(map[string]string{
		"status":   `"New"`,
		"contacts": collectionRowsJSON(recordCollectionMaxRows + 1),
	}))
	assertRecordError(t, err, RecordErrorValidation, "contacts")
	if err == nil || !strings.Contains(err.Error(), "too many rows") {
		t.Fatalf("CreateRecord(collection max rows) error = %v, want max row validation", err)
	}
}

func TestRecordStoreBeforeUpdateHookCanMutateInput(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "Renamed User", false},
	}))
	registry := NewRecordHookRegistry()
	mustRegisterRecordHook(registry.RegisterEntity("core", "user", RecordBeforeUpdate, "disable-user", func(_ context.Context, hookCtx RecordHookContext) error {
		hookCtx.Input["enabled"] = json.RawMessage(`false`)
		return nil
	}))

	record, err := NewRecordStoreWithHooks(queryer, registry).UpdateRecord(context.Background(), "user", 7, recordInput(map[string]string{
		"full-name": `"Renamed User"`,
	}))
	if err != nil {
		t.Fatalf("UpdateRecord() error = %v, want nil", err)
	}
	if record["enabled"] != false {
		t.Fatalf("UpdateRecord() = %+v, want before-update input mutation applied", record)
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{`"enabled" = $1`, `"full_name" = $2`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("update query = %q, want %q", lastQuery, want)
		}
	}
	args := queryer.args[len(queryer.args)-1]
	if len(args) < 3 || args[0] != false || args[1] != "Renamed User" || args[2] != int64(7) {
		t.Fatalf("update args = %#v, want hook-mutated enabled plus original full-name", args)
	}
}

func TestRecordStoreHashesPasswordOnCreate(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	record, err := NewRecordStore(queryer).CreateRecord(context.Background(), "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"A User"`,
		"password":  `"super-secret"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	if _, ok := record["password"]; ok {
		t.Fatalf("CreateRecord() returned password field: %+v", record)
	}
	lastQuery, args := lastQueryContaining(t, queryer, `INSERT INTO "user"`)
	for _, want := range []string{`"password_hash"`, `RETURNING "_dygo_record"."id", "_dygo_record"."name", "_dygo_record"."created_at", "_dygo_record"."updated_at", "_dygo_record"."email", "_dygo_record"."full_name", "_dygo_record"."enabled"`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("create query = %q, want %q", lastQuery, want)
		}
	}
	if strings.Contains(lastQuery, `RETURNING "_dygo_record"."id", "_dygo_record"."name", "_dygo_record"."created_at", "_dygo_record"."updated_at", "_dygo_record"."email", "_dygo_record"."full_name", "_dygo_record"."password_hash"`) {
		t.Fatalf("create query = %q, returned password_hash", lastQuery)
	}
	hash, ok := args[len(args)-2].(string)
	if !ok {
		t.Fatalf("password arg type = %T, want string", args[len(args)-2])
	}
	if hash == "super-secret" {
		t.Fatal("password arg is plaintext, want bcrypt hash")
	}
	if err := auth.ComparePassword(hash, "super-secret"); err != nil {
		t.Fatalf("stored password hash did not verify: %v", err)
	}
}

func TestRecordStoreHashesPasswordOnUpdate(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	_, err := NewRecordStore(queryer).UpdateRecord(context.Background(), "user", 7, recordInput(map[string]string{
		"password": `"changed-secret"`,
	}))
	if err != nil {
		t.Fatalf("UpdateRecord() error = %v, want nil", err)
	}
	lastQuery, args := lastQueryContaining(t, queryer, `UPDATE "user"`)
	if !strings.Contains(lastQuery, `UPDATE "user" AS "_dygo_record" SET "password_hash" = $1`) {
		t.Fatalf("update query = %q, want password_hash update", lastQuery)
	}
	hash, ok := args[0].(string)
	if !ok {
		t.Fatalf("password arg type = %T, want string", args[0])
	}
	if hash == "changed-secret" {
		t.Fatal("password arg is plaintext, want bcrypt hash")
	}
	if err := auth.ComparePassword(hash, "changed-secret"); err != nil {
		t.Fatalf("stored password hash did not verify: %v", err)
	}
}

func TestRecordStoreDeleteRecord(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	queryer.execTags = []pgconn.CommandTag{pgconn.NewCommandTag("DELETE 1")}

	err := NewRecordStore(queryer).DeleteRecord(context.Background(), "user", 7)
	if err != nil {
		t.Fatalf("DeleteRecord() error = %v, want nil", err)
	}
	if !strings.Contains(queryer.execSQL[0], `DELETE FROM "user" WHERE "id" = $1`) {
		t.Fatalf("delete SQL = %q, want hard delete by id", queryer.execSQL[0])
	}
}

func TestRecordStoreDeleteRecordDeletesCollectionRows(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newLeadRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "lead-7", now, now, "New"},
	}))
	queryer.rows = append(queryer.rows, newFakeRows(nil))
	queryer.execTags = []pgconn.CommandTag{
		pgconn.NewCommandTag("DELETE 2"),
		pgconn.NewCommandTag("DELETE 1"),
	}

	err := NewRecordStore(queryer).DeleteRecord(context.Background(), "lead", 7)
	if err != nil {
		t.Fatalf("DeleteRecord(collection owner) error = %v, want nil", err)
	}
	if len(queryer.execSQL) < 2 {
		t.Fatalf("exec SQL count = %d, want child then parent deletes", len(queryer.execSQL))
	}
	if !strings.Contains(queryer.execSQL[0], `DELETE FROM "crm_lead_contact" WHERE "parent_entity_id" = $1 AND "parent_record_id" = $2 AND "parent_field_id" = $3`) ||
		!reflect.DeepEqual(queryer.execArg[0], []any{int64(20), int64(7), int64(2)}) {
		t.Fatalf("child delete = %q %#v, want parent entity 20 record 7 field 2", queryer.execSQL[0], queryer.execArg[0])
	}
	if !strings.Contains(queryer.execSQL[1], `DELETE FROM "crm_lead" WHERE "id" = $1`) ||
		!reflect.DeepEqual(queryer.execArg[1], []any{int64(7)}) {
		t.Fatalf("parent delete = %q %#v, want hard delete by id", queryer.execSQL[1], queryer.execArg[1])
	}
}

func TestRecordStoreCreateRecordWritesActivity(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	ctx := WithActivityActorName(WithActivitySource(context.Background(), ActivitySourceAPI), "admin@example.com")
	record, err := NewRecordStore(queryer).CreateRecord(ctx, "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"A User"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	if record["id"] != int64(7) {
		t.Fatalf("CreateRecord() id = %v, want 7", record["id"])
	}
	args := activityArgs(t, queryer)
	if args[1] != corevalues.ActivityOperationCreate || args[3] != int64(10) || args[4] != int64(7) || args[5] != int64(99) {
		t.Fatalf("activity args = %#v, want create for user entity and actor name", args)
	}
	if name, ok := args[11].(string); !ok || len(name) != 16 {
		t.Fatalf("activity name arg = %#v, want generated length-16 string", args[11])
	}
	snapshot := decodeActivityObject(t, args[9])
	if snapshot["email"] != "a@example.com" || snapshot["password"] != nil {
		t.Fatalf("activity snapshot = %#v, want visible record without password", snapshot)
	}
	details := decodeActivityObject(t, args[10])
	if details["source"] != ActivitySourceAPI {
		t.Fatalf("activity details = %#v, want api source", details)
	}
}

func TestRecordStoreHookPolicyNoneSuppressesFrameworkActivity(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	_, err := NewRecordStoreWithHookPolicy(queryer, RecordMutationHooksNone).CreateRecord(context.Background(), "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"A User"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	if executedSQLContaining(queryer, `INSERT INTO "activity"`) {
		t.Fatalf("exec SQL = %#v, want hook policy none to suppress Activity", queryer.execSQL)
	}
}

func TestRecordStoreUpdateRecordWritesActivityDiffsAndRedactsPassword(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "Renamed User", true},
	}))

	_, err := NewRecordStore(queryer).UpdateRecord(context.Background(), "user", 7, recordInput(map[string]string{
		"full-name": `"Renamed User"`,
		"password":  `"changed-secret"`,
	}))
	if err != nil {
		t.Fatalf("UpdateRecord() error = %v, want nil", err)
	}
	args := activityArgs(t, queryer)
	if args[1] != corevalues.ActivityOperationUpdate || args[4] != int64(7) {
		t.Fatalf("activity args = %#v, want update for record 7", args)
	}
	encodedChanges, ok := args[8].(string)
	if !ok {
		t.Fatalf("activity changes arg type = %T, want string", args[8])
	}
	if strings.Contains(encodedChanges, "changed-secret") {
		t.Fatalf("activity changes leaked plaintext password: %s", encodedChanges)
	}
	changes := decodeActivityList(t, args[8])
	if len(changes) != 2 {
		t.Fatalf("activity changes = %#v, want full-name and password changes", changes)
	}
	if changes[0]["field"] != "full-name" || changes[0]["old"] != "A User" || changes[0]["new"] != "Renamed User" {
		t.Fatalf("first activity change = %#v, want full-name diff", changes[0])
	}
	if changes[1]["field"] != "password" || changes[1]["redacted"] != true {
		t.Fatalf("second activity change = %#v, want redacted password", changes[1])
	}
}

func TestRecordStoreUpdateRecordSkipsActivityWithoutChanges(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	_, err := NewRecordStore(queryer).UpdateRecord(context.Background(), "user", 7, recordInput(map[string]string{
		"full-name": `"A User"`,
	}))
	if err != nil {
		t.Fatalf("UpdateRecord() error = %v, want nil", err)
	}
	if len(queryer.execSQL) != 0 {
		t.Fatalf("exec SQL = %#v, want no activity insert for unchanged update", queryer.execSQL)
	}
}

func TestRecordStoreAfterUpdateHookRunsWithoutChanges(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	var afterCalls int
	var observed RecordHookContext
	registry := DefaultRecordHookRegistry()
	mustRegisterRecordHook(registry.RegisterEntity("core", "user", RecordAfterUpdate, "observe-noop-update", func(_ context.Context, hookCtx RecordHookContext) error {
		afterCalls++
		observed = hookCtx
		return nil
	}))

	_, err := NewRecordStoreWithHooks(queryer, registry).UpdateRecord(context.Background(), "user", 7, recordInput(map[string]string{
		"full-name": `"A User"`,
	}))
	if err != nil {
		t.Fatalf("UpdateRecord() error = %v, want nil", err)
	}
	if afterCalls != 1 {
		t.Fatalf("after-update hook calls = %d, want 1", afterCalls)
	}
	if observed.RecordID != int64(7) || observed.Operation != corevalues.ActivityOperationUpdate {
		t.Fatalf("after-update context = %+v, want record 7 update", observed)
	}
	if observed.OldRecord["full-name"] != "A User" || observed.NewRecord["full-name"] != "A User" {
		t.Fatalf("after-update records = old %+v new %+v, want unchanged snapshots", observed.OldRecord, observed.NewRecord)
	}
	if len(observed.Changes) != 0 {
		t.Fatalf("after-update changes = %#v, want empty", observed.Changes)
	}
	if len(queryer.execSQL) != 0 {
		t.Fatalf("exec SQL = %#v, want no activity insert for unchanged update", queryer.execSQL)
	}
}

func TestRecordStoreDeleteRecordWritesActivitySnapshot(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	queryer.execTags = []pgconn.CommandTag{pgconn.NewCommandTag("DELETE 1")}

	err := NewRecordStore(queryer).DeleteRecord(context.Background(), "user", 7)
	if err != nil {
		t.Fatalf("DeleteRecord() error = %v, want nil", err)
	}
	args := activityArgs(t, queryer)
	if args[1] != corevalues.ActivityOperationDelete || args[4] != int64(7) {
		t.Fatalf("activity args = %#v, want delete for record 7", args)
	}
	snapshot := decodeActivityObject(t, args[9])
	if snapshot["email"] != "a@example.com" {
		t.Fatalf("activity snapshot = %#v, want deleted record snapshot", snapshot)
	}
}

func TestRecordStoreSkipsActivityForActivityEntity(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newActivityRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(1), "activity-1", now, now, "record", "create", "success", nil, nil, nil, "Created User", nil, nil, nil, nil},
	}))

	_, err := NewRecordStore(queryer).CreateRecord(context.Background(), "activity", recordInput(map[string]string{
		"kind":      `"record"`,
		"operation": `"create"`,
		"status":    `"success"`,
		"title":     `"Created User"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord(activity) error = %v, want nil", err)
	}
	if len(queryer.execSQL) != 0 {
		t.Fatalf("exec SQL = %#v, want no recursive activity insert", queryer.execSQL)
	}
}

func TestRecordStoreActivityFailureRollsBackTransactionalMutation(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	queryer.execErrs = []error{errors.New("activity insert failed")}
	transactional := &fakeTransactionalRecordQueryer{fakeRecordQueryer: queryer}

	_, err := NewRecordStore(transactional).CreateRecord(context.Background(), "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"A User"`,
	}))
	assertRecordError(t, err, RecordErrorInternal, "")
	if transactional.tx == nil || !transactional.tx.rolledBack || transactional.tx.committed {
		t.Fatalf("transaction state = %+v, want rollback without commit", transactional.tx)
	}
}

func TestRecordStoreRunsGlobalHooksBeforeEntityHooks(t *testing.T) {
	queryer := newUserRecordQueryer()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	var order []string
	registry := NewRecordHookRegistry()
	mustRegisterRecordHook(registry.RegisterGlobal(RecordBeforeCreate, "global-before-create", func(context.Context, RecordHookContext) error {
		order = append(order, "global-before-create")
		return nil
	}))
	mustRegisterRecordHook(registry.RegisterEntity("core", "user", RecordBeforeCreate, "entity-before-create", func(context.Context, RecordHookContext) error {
		order = append(order, "entity-before-create")
		return nil
	}))
	mustRegisterRecordHook(registry.RegisterGlobal(RecordAfterCreate, "global-after-create", func(context.Context, RecordHookContext) error {
		order = append(order, "global-after-create")
		return nil
	}))
	mustRegisterRecordHook(registry.RegisterEntity("core", "user", RecordAfterCreate, "entity-after-create", func(context.Context, RecordHookContext) error {
		order = append(order, "entity-after-create")
		return nil
	}))

	_, err := NewRecordStoreWithHooks(queryer, registry).CreateRecord(context.Background(), "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"A User"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	want := []string{"global-before-create", "entity-before-create", "global-after-create", "entity-after-create"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("hook order = %#v, want %#v", order, want)
	}
}

func TestRecordStoreBeforeValidateHookCanMutateInput(t *testing.T) {
	queryer := newUserRecordQueryer()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "Filled User", true},
	}))
	registry := NewRecordHookRegistry()
	mustRegisterRecordHook(registry.RegisterEntity("core", "user", RecordBeforeValidate, "fill-full-name", func(_ context.Context, hookCtx RecordHookContext) error {
		hookCtx.Input["full-name"] = json.RawMessage(`"Filled User"`)
		return nil
	}))

	_, err := NewRecordStoreWithHooks(queryer, registry).CreateRecord(context.Background(), "user", recordInput(map[string]string{
		"email": `"a@example.com"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	args := queryer.args[len(queryer.args)-1]
	if args[1] != "Filled User" {
		t.Fatalf("create args = %#v, want hook-mutated full-name", args)
	}
}

func TestRecordStoreValidateHookCannotMutateTargetInput(t *testing.T) {
	queryer := newUserRecordQueryer()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "Original User", true},
	}))
	registry := NewRecordHookRegistry()
	mustRegisterRecordHook(registry.RegisterEntity("core", "user", RecordValidate, "attempt-mutate", func(_ context.Context, hookCtx RecordHookContext) error {
		hookCtx.Input["full-name"] = json.RawMessage(`"Changed By Validate"`)
		return nil
	}))

	_, err := NewRecordStoreWithHooks(queryer, registry).CreateRecord(context.Background(), "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"Original User"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	args := queryer.args[len(queryer.args)-1]
	if args[1] != "Original User" {
		t.Fatalf("create args = %#v, want validate hook context mutation ignored for target input", args)
	}
}

func TestRecordStoreObservationHookContextMutationDoesNotLeak(t *testing.T) {
	queryer := newUserRecordQueryer()
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))
	registry := NewRecordHookRegistry()
	mustRegisterRecordHook(registry.RegisterGlobal(RecordAfterCreate, "global-mutator", func(_ context.Context, hookCtx RecordHookContext) error {
		hookCtx.NewRecord["email"] = "changed@example.com"
		return nil
	}))
	var observed any
	mustRegisterRecordHook(registry.RegisterEntity("core", "user", RecordAfterCreate, "entity-observer", func(_ context.Context, hookCtx RecordHookContext) error {
		observed = hookCtx.NewRecord["email"]
		return nil
	}))

	_, err := NewRecordStoreWithHooks(queryer, registry).CreateRecord(context.Background(), "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"A User"`,
	}))
	if err != nil {
		t.Fatalf("CreateRecord() error = %v, want nil", err)
	}
	if observed != "a@example.com" {
		t.Fatalf("entity after-create observed email = %#v, want unchanged snapshot", observed)
	}
}

func TestRecordStoreHookErrorPreventsMutation(t *testing.T) {
	queryer := newUserRecordQueryer()
	registry := NewRecordHookRegistry()
	mustRegisterRecordHook(registry.RegisterEntity("core", "user", RecordBeforeCreate, "reject-create", func(context.Context, RecordHookContext) error {
		return errors.New("blocked by test hook")
	}))

	_, err := NewRecordStoreWithHooks(queryer, registry).CreateRecord(context.Background(), "user", recordInput(map[string]string{
		"email":     `"a@example.com"`,
		"full-name": `"A User"`,
	}))
	assertRecordError(t, err, RecordErrorValidation, "")
	for _, query := range queryer.queries {
		if strings.Contains(query, `INSERT INTO "user"`) {
			t.Fatalf("query %q was executed after hook rejection", query)
		}
	}
}

func TestRecordStoreValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		entity    string
		queryer   *fakeRecordQueryer
		input     RecordInput
		wantCode  string
		wantField string
	}{
		{
			name:      "missing required field",
			queryer:   newUserRecordQueryer(),
			input:     recordInput(map[string]string{"email": `"a@example.com"`}),
			wantCode:  RecordErrorValidation,
			wantField: "full-name",
		},
		{
			name:      "unknown field",
			queryer:   newUserRecordQueryer(),
			input:     recordInput(map[string]string{"email": `"a@example.com"`, "full-name": `"A User"`, "legacy": `"x"`}),
			wantCode:  RecordErrorValidation,
			wantField: "legacy",
		},
		{
			name:      "system field",
			queryer:   newUserRecordQueryer(),
			input:     recordInput(map[string]string{"email": `"a@example.com"`, "full-name": `"A User"`, "id": `1`}),
			wantCode:  RecordErrorValidation,
			wantField: "id",
		},
		{
			name:      "invalid select value",
			entity:    "lead",
			queryer:   newLeadRecordQueryer(),
			input:     recordInput(map[string]string{"status": `"Archived"`}),
			wantCode:  RecordErrorValidation,
			wantField: "status",
		},
		{
			name:      "collection non-array",
			entity:    "lead",
			queryer:   newLeadRecordQueryer(),
			input:     recordInput(map[string]string{"status": `"New"`, "contacts": `{}`}),
			wantCode:  RecordErrorValidation,
			wantField: "contacts",
		},
		{
			name:      "empty password",
			queryer:   newUserRecordQueryer(),
			input:     recordInput(map[string]string{"email": `"a@example.com"`, "full-name": `"A User"`, "password": `""`}),
			wantCode:  RecordErrorValidation,
			wantField: "password",
		},
		{
			name:      "too long password",
			queryer:   newUserRecordQueryer(),
			input:     RecordInput{"email": json.RawMessage(`"a@example.com"`), "full-name": json.RawMessage(`"A User"`), "password": json.RawMessage(`"secret-` + strings.Repeat("x", 80) + `"`)},
			wantCode:  RecordErrorValidation,
			wantField: "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := tt.entity
			if entity == "" {
				entity = "user"
			}
			_, err := NewRecordStore(tt.queryer).CreateRecord(context.Background(), entity, tt.input)
			if tt.queryer == nil {
				t.Fatal("test queryer is nil")
			}
			assertRecordError(t, err, tt.wantCode, tt.wantField)
		})
	}
}

func TestRecordStoreMapsDatabaseErrors(t *testing.T) {
	tests := []struct {
		name     string
		pgCode   string
		wantCode string
	}{
		{name: "schema not ready", pgCode: "42P01", wantCode: RecordErrorSchemaNotReady},
		{name: "constraint violation", pgCode: "23505", wantCode: RecordErrorConstraintViolation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryer := newUserRecordQueryer()
			queryer.queryErrs = append(queryer.queryErrs, nil, nil, nil, &pgconn.PgError{Code: tt.pgCode, ConstraintName: "user_email_key"})

			_, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{})
			assertRecordError(t, err, tt.wantCode, "")
		})
	}
}

func TestRecordStoreMapsMetadataSchemaErrors(t *testing.T) {
	queryer := newUserRecordQueryer()
	queryer.row = fakeRow{err: &pgconn.PgError{Code: "42P01"}}

	_, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{})
	assertRecordError(t, err, RecordErrorSchemaNotReady, "")
}

func TestRecordStoreInvalidPaginationAndIDs(t *testing.T) {
	_, err := NewRecordStore(newUserRecordQueryer()).ListRecords(context.Background(), "user", RecordListParams{Limit: 2501})
	assertRecordError(t, err, RecordErrorInvalidRequest, "")

	_, err = NewRecordStore(newUserRecordQueryer()).GetRecord(context.Background(), "user", 0)
	assertRecordError(t, err, RecordErrorInvalidRequest, "")
}

func TestRecordStoreInvalidListFiltersAndSorts(t *testing.T) {
	tests := []struct {
		name    string
		queryer *fakeRecordQueryer
		params  RecordListParams
		code    string
	}{
		{name: "unknown filter", queryer: newUserRecordQueryer(), params: RecordListParams{Filters: []RecordFilter{{Field: "missing", Value: "x"}}}, code: RecordErrorInvalidRequest},
		{name: "write-only filter", queryer: newUserRecordQueryer(), params: RecordListParams{Filters: []RecordFilter{{Field: "password", Value: "secret"}}}, code: RecordErrorInvalidRequest},
		{name: "non-storage filter", queryer: newLeadRecordQueryer(), params: RecordListParams{Filters: []RecordFilter{{Field: "contacts", Value: "1"}}}, code: RecordErrorInvalidRequest},
		{name: "duplicate filter", queryer: newUserRecordQueryer(), params: RecordListParams{Filters: []RecordFilter{{Field: "email", Value: "a"}, {Field: "email", Value: "b"}}}, code: RecordErrorInvalidRequest},
		{name: "invalid boolean filter", queryer: newUserRecordQueryer(), params: RecordListParams{Filters: []RecordFilter{{Field: "enabled", Value: "yes"}}}, code: RecordErrorValidation},
		{name: "invalid datetime filter", queryer: newUserRecordQueryer(), params: RecordListParams{Filters: []RecordFilter{{Field: "created-at", Value: "May 7"}}}, code: RecordErrorValidation},
		{name: "unknown sort", queryer: newUserRecordQueryer(), params: RecordListParams{Sort: []RecordSort{{Field: "missing"}}}, code: RecordErrorInvalidRequest},
		{name: "write-only sort", queryer: newUserRecordQueryer(), params: RecordListParams{Sort: []RecordSort{{Field: "password"}}}, code: RecordErrorInvalidRequest},
		{name: "non-storage sort", queryer: newLeadRecordQueryer(), params: RecordListParams{Sort: []RecordSort{{Field: "contacts"}}}, code: RecordErrorInvalidRequest},
		{name: "duplicate sort", queryer: newUserRecordQueryer(), params: RecordListParams{Sort: []RecordSort{{Field: "email"}, {Field: "email", Desc: true}}}, code: RecordErrorInvalidRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRecordStore(tt.queryer).ListRecords(context.Background(), "user", tt.params)
			assertRecordError(t, err, tt.code, "")
		})
	}
}

type testEntityMeta struct {
	id           int64
	name         string
	key          string
	slug         string
	label        string
	description  string
	icon         string
	isSingle     bool
	isSystem     bool
	isCollection bool
	naming       []byte
	app          string
	appLabel     string
}

func (meta testEntityMeta) row() pgx.Row {
	return newFakeRow(
		meta.id,
		meta.name,
		meta.key,
		meta.slug,
		meta.label,
		meta.description,
		meta.icon,
		meta.isSingle,
		meta.isSystem,
		meta.isCollection,
		meta.naming,
		meta.app,
		meta.appLabel,
	)
}

func userEntityMeta() testEntityMeta {
	return testEntityMeta{
		id:          10,
		name:        "core.user",
		key:         "user",
		slug:        "user",
		label:       "User",
		description: "User identity",
		icon:        "user",
		naming:      []byte(`{"strategy":"format","format":"{email}"}`),
		app:         "core",
		appLabel:    "Core",
	}
}

func leadEntityMeta() testEntityMeta {
	return testEntityMeta{
		id:          20,
		name:        "crm.lead",
		key:         "lead",
		slug:        "lead",
		label:       "Lead",
		description: "Sales lead",
		icon:        "contact",
		naming:      []byte(`{"strategy":"random","length":16}`),
		app:         "crm",
		appLabel:    "CRM",
	}
}

func leadContactEntityMeta() testEntityMeta {
	return testEntityMeta{
		id:           21,
		name:         "crm.lead-contact",
		key:          "lead-contact",
		label:        "Lead Contact",
		description:  "Lead contact row",
		icon:         "contact",
		isCollection: true,
		app:          "crm",
		appLabel:     "CRM",
	}
}

func templateEntityMeta() testEntityMeta {
	return testEntityMeta{
		id:          50,
		name:        "support.ticket",
		key:         "ticket",
		slug:        "ticket",
		label:       "Ticket",
		description: "Support ticket",
		icon:        "ticket",
		naming:      []byte(`{"strategy":"format","format":"T-{status}-{code}"}`),
		app:         "support",
		appLabel:    "Support",
	}
}

func eventEntityMeta() testEntityMeta {
	return testEntityMeta{
		id:          60,
		name:        "core.event",
		key:         "event",
		slug:        "event",
		label:       "Event",
		description: "Calendar event",
		icon:        "calendar",
		naming:      []byte(`{"strategy":"manual","label":"Name"}`),
		app:         "core",
		appLabel:    "Core",
	}
}

func singleSettingsEntityMeta() testEntityMeta {
	return testEntityMeta{
		id:          30,
		name:        "sales.invoice-settings",
		key:         "invoice-settings",
		slug:        "invoice-settings",
		label:       "Invoice Settings",
		description: "Invoice defaults",
		icon:        "settings",
		isSingle:    true,
		naming:      []byte(`{"strategy":"random","length":16}`),
		app:         "sales",
		appLabel:    "Sales",
	}
}

func collectionEntityMeta() testEntityMeta {
	return testEntityMeta{
		id:           40,
		name:         "sales.invoice-item",
		key:          "invoice-item",
		slug:         "invoice-item",
		label:        "Invoice Item",
		description:  "Invoice line item",
		icon:         "list",
		isCollection: true,
		app:          "sales",
		appLabel:     "Sales",
	}
}

func activityEntityMeta() testEntityMeta {
	return testEntityMeta{
		id:          1,
		name:        "core.activity",
		key:         "activity",
		slug:        "activity",
		label:       "Activity",
		description: "Timeline entry",
		icon:        "activity",
		naming:      []byte(`{"strategy":"random","length":16}`),
		app:         "core",
		appLabel:    "Core",
	}
}

func metadataFieldRow(name string, label string, fieldType string, required bool, unique bool, indexed bool, defaultValue []byte, check []byte, position int, options []byte) []any {
	return []any{int64(position), name, label, fieldType, required, unique, indexed, defaultValue, check, position, options}
}

func userFieldRows() [][]any {
	return [][]any{
		metadataFieldRow("email", "Email", "email", true, true, false, nil, nil, 1, nil),
		metadataFieldRow("full-name", "Full Name", "text", true, false, false, nil, nil, 2, nil),
		metadataFieldRow("password", "Password", "password", false, false, false, nil, nil, 3, nil),
		metadataFieldRow("enabled", "Enabled", "boolean", false, false, true, []byte("true"), nil, 4, nil),
	}
}

func leadFieldRows() [][]any {
	return [][]any{
		metadataFieldRow("status", "Status", "select", true, false, false, nil, nil, 1, []byte(`{"values":["New","Qualified"]}`)),
		metadataFieldRow("contacts", "Contacts", "collection", false, false, false, nil, nil, 2, []byte(`{"entity":"lead-contact"}`)),
	}
}

func leadContactFieldRows() [][]any {
	return [][]any{
		metadataFieldRow("email", "Email", "email", true, false, false, nil, nil, 1, nil),
		metadataFieldRow("full-name", "Full Name", "text", false, false, false, nil, nil, 2, nil),
	}
}

func templateFieldRows() [][]any {
	return [][]any{
		metadataFieldRow("code", "Code", "text", true, false, false, nil, nil, 1, nil),
		metadataFieldRow("status", "Status", "select", true, false, false, nil, nil, 2, []byte(`{"values":["New","Closed"]}`)),
	}
}

func eventFieldRows() [][]any {
	return [][]any{
		metadataFieldRow("starts-at", "Starts At", "datetime", true, false, false, nil, nil, 1, nil),
	}
}

func singleSettingsFieldRows() [][]any {
	return [][]any{
		metadataFieldRow("default-due-days", "Default Due Days", "int", true, false, false, []byte("30"), nil, 1, nil),
	}
}

func collectionFieldRows() [][]any {
	return [][]any{
		metadataFieldRow("item-code", "Item Code", "text", true, false, false, nil, nil, 1, nil),
	}
}

func activityFieldRows() [][]any {
	return [][]any{
		metadataFieldRow("kind", "Kind", "select", true, false, false, nil, nil, 1, []byte(`{"values":["record","comment","workflow","job","email","attachment","auth","system"]}`)),
		metadataFieldRow("operation", "Operation", "select", true, false, false, nil, nil, 2, []byte(`{"values":["create","update","delete","restore","comment","workflow-transition","job-completed","email-sent","attachment-added","login","logout","system"]}`)),
		metadataFieldRow("status", "Status", "select", true, false, false, nil, nil, 3, []byte(`{"values":["success","failed"]}`)),
		metadataFieldRow("entity", "Entity", "link", false, false, false, nil, nil, 4, []byte(`{"entity":"entity","foreign-key":false}`)),
		metadataFieldRow("record-id", "Record ID", "bigint", false, false, false, nil, nil, 5, nil),
		metadataFieldRow("actor", "Actor", "link", false, false, false, nil, nil, 6, []byte(`{"entity":"user","foreign-key":false}`)),
		metadataFieldRow("title", "Title", "text", true, false, false, nil, nil, 7, nil),
		metadataFieldRow("message", "Message", "long-text", false, false, false, nil, nil, 8, nil),
		metadataFieldRow("changes", "Changes", "json", false, false, false, nil, nil, 9, nil),
		metadataFieldRow("snapshot", "Snapshot", "json", false, false, false, nil, nil, 10, nil),
		metadataFieldRow("details", "Details", "json", false, false, false, nil, nil, 11, nil),
	}
}

func newUserRecordQueryer() *fakeRecordQueryer {
	meta := userEntityMeta()
	return &fakeRecordQueryer{
		row: meta.row(),
		rows: []pgx.Rows{
			newFakeRows(userFieldRows()),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}
}

func newSystemUserRecordQueryer() *fakeRecordQueryer {
	queryer := newUserRecordQueryer()
	meta := userEntityMeta()
	meta.isSystem = true
	queryer.row = meta.row()
	return queryer
}

func newLeadRecordQueryer() *fakeRecordQueryer {
	meta := leadEntityMeta()
	return &fakeRecordQueryer{
		row: meta.row(),
		rows: []pgx.Rows{
			newFakeRows(leadFieldRows()),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}
}

func newTemplateRecordQueryer() *fakeRecordQueryer {
	meta := templateEntityMeta()
	return &fakeRecordQueryer{
		row: meta.row(),
		rows: []pgx.Rows{
			newFakeRows(templateFieldRows()),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}
}

func newEventRecordQueryer() *fakeRecordQueryer {
	meta := eventEntityMeta()
	return &fakeRecordQueryer{
		row: meta.row(),
		rows: []pgx.Rows{
			newFakeRows(eventFieldRows()),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}
}

func newSingleSettingsRecordQueryer() *fakeRecordQueryer {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	meta := singleSettingsEntityMeta()
	return &fakeRecordQueryer{
		row: meta.row(),
		rows: []pgx.Rows{
			newFakeRows(singleSettingsFieldRows()),
			newFakeRows(nil),
			newFakeRows(nil),
			newFakeRows([][]any{
				{int64(7), "invoice-settings", now, now, int64(30)},
			}),
		},
	}
}

func newSystemSingleSettingsRecordQueryer() *fakeRecordQueryer {
	queryer := newSingleSettingsRecordQueryer()
	meta := singleSettingsEntityMeta()
	meta.isSystem = true
	queryer.row = meta.row()
	return queryer
}

func newCollectionRecordQueryer() *fakeRecordQueryer {
	meta := collectionEntityMeta()
	return &fakeRecordQueryer{
		row: meta.row(),
		rows: []pgx.Rows{
			newFakeRows(collectionFieldRows()),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}
}

func newActivityRecordQueryer() *fakeRecordQueryer {
	meta := activityEntityMeta()
	return &fakeRecordQueryer{
		row: meta.row(),
	}
}

type fakeRecordQueryer struct {
	row                 pgx.Row
	rows                []pgx.Rows
	queryErrs           []error
	execTags            []pgconn.CommandTag
	execErrs            []error
	collectionFieldRows [][]any

	queries []string
	args    [][]any
	rowSQL  []string
	rowArgs [][]any
	execSQL []string
	execArg [][]any
}

func (q *fakeRecordQueryer) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.queries = append(q.queries, sql)
	q.args = append(q.args, args)
	if rows, ok := fakeActivityMetadataRows(sql, args...); ok {
		return rows, nil
	}
	if rows, ok := q.fakeCollectionMetadataRows(sql, args...); ok {
		return rows, nil
	}
	if len(q.queryErrs) > 0 {
		err := q.queryErrs[0]
		q.queryErrs = q.queryErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if len(q.rows) == 0 {
		return newFakeRows(nil), nil
	}
	rows := q.rows[0]
	q.rows = q.rows[1:]
	return rows, nil
}

func (q *fakeRecordQueryer) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.rowSQL = append(q.rowSQL, sql)
	q.rowArgs = append(q.rowArgs, args)
	if isActivityMetadataQuery(sql, args...) {
		return fakeActivityEntityRow()
	}
	if isLeadContactMetadataQuery(sql, args...) {
		return leadContactEntityMeta().row()
	}
	if strings.Contains(sql, `SELECT "id" FROM "entity"`) && len(args) == 1 {
		switch args[0] {
		case "core.user":
			return newFakeRow(int64(10))
		case "crm.lead":
			return newFakeRow(int64(20))
		case "sales.invoice-settings":
			return newFakeRow(int64(30))
		case "support.ticket":
			return newFakeRow(int64(50))
		}
	}
	if strings.Contains(sql, `SELECT "id" FROM "user"`) && len(args) == 1 && args[0] == "admin@example.com" {
		return newFakeRow(int64(99))
	}
	if strings.Contains(sql, `SELECT "name" FROM "entity"`) && len(args) == 1 && args[0] == int64(10) {
		return newFakeRow("core.user")
	}
	if strings.Contains(sql, `SELECT "name" FROM "user"`) && len(args) == 1 && args[0] == int64(99) {
		return newFakeRow("admin@example.com")
	}
	if q.row == nil {
		return fakeRow{err: pgx.ErrNoRows}
	}
	return q.row
}

func (q *fakeRecordQueryer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execSQL = append(q.execSQL, sql)
	q.execArg = append(q.execArg, args)
	if len(q.execErrs) > 0 {
		err := q.execErrs[0]
		q.execErrs = q.execErrs[1:]
		if err != nil {
			return pgconn.CommandTag{}, err
		}
	}
	if len(q.execTags) == 0 {
		return pgconn.NewCommandTag("DELETE 0"), nil
	}
	tag := q.execTags[0]
	q.execTags = q.execTags[1:]
	return tag, nil
}

type fakeTransactionalRecordQueryer struct {
	*fakeRecordQueryer
	tx *fakeRecordTx
}

func (q *fakeTransactionalRecordQueryer) Begin(context.Context) (pgx.Tx, error) {
	q.tx = &fakeRecordTx{fakeRecordQueryer: q.fakeRecordQueryer}
	return q.tx, nil
}

type fakeRecordTx struct {
	*fakeRecordQueryer
	committed  bool
	rolledBack bool
}

func (tx *fakeRecordTx) Begin(context.Context) (pgx.Tx, error) {
	return &fakeRecordTx{fakeRecordQueryer: tx.fakeRecordQueryer}, nil
}

func (tx *fakeRecordTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *fakeRecordTx) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *fakeRecordTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (tx *fakeRecordTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (tx *fakeRecordTx) LargeObjects() pgx.LargeObjects {
	return pgx.LargeObjects{}
}

func (tx *fakeRecordTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, nil
}

func (tx *fakeRecordTx) Conn() *pgx.Conn {
	return nil
}

func recordInput(values map[string]string) RecordInput {
	input := RecordInput{}
	for key, value := range values {
		input[key] = json.RawMessage(value)
	}
	return input
}

func collectionRowsJSON(count int) string {
	var builder strings.Builder
	builder.WriteByte('[')
	for index := 0; index < count; index++ {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(`{"email":"row@example.com"}`)
	}
	builder.WriteByte(']')
	return builder.String()
}

func lastQueryContaining(t *testing.T, queryer *fakeRecordQueryer, fragment string) (string, []any) {
	t.Helper()
	for index := len(queryer.queries) - 1; index >= 0; index-- {
		if strings.Contains(queryer.queries[index], fragment) {
			return queryer.queries[index], queryer.args[index]
		}
	}
	t.Fatalf("query containing %q was not executed; queries = %#v", fragment, queryer.queries)
	return "", nil
}

func lastExecContaining(t *testing.T, queryer *fakeRecordQueryer, fragment string) (string, []any) {
	t.Helper()
	for index := len(queryer.execSQL) - 1; index >= 0; index-- {
		if strings.Contains(queryer.execSQL[index], fragment) {
			return queryer.execSQL[index], queryer.execArg[index]
		}
	}
	t.Fatalf("exec containing %q was not executed; exec SQL = %#v", fragment, queryer.execSQL)
	return "", nil
}

func executedSQLContaining(queryer *fakeRecordQueryer, fragment string) bool {
	for _, sql := range queryer.execSQL {
		if strings.Contains(sql, fragment) {
			return true
		}
	}
	return false
}

func fakeActivityEntityRow() pgx.Row {
	return activityEntityMeta().row()
}

func isActivityMetadataQuery(sql string, args ...any) bool {
	return strings.Contains(sql, `WHERE a.name = $1 AND e.key = $2`) &&
		len(args) == 2 &&
		args[0] == "core" &&
		args[1] == "activity"
}

func fakeActivityMetadataRows(sql string, args ...any) (pgx.Rows, bool) {
	if len(args) != 1 || args[0] != int64(1) {
		return nil, false
	}
	switch {
	case strings.Contains(sql, `FROM "field"`):
		return newFakeRows(activityFieldRows()), true
	case strings.Contains(sql, `FROM "index"`), strings.Contains(sql, `FROM "constraint"`):
		return newFakeRows(nil), true
	default:
		return nil, false
	}
}

func isLeadContactMetadataQuery(sql string, args ...any) bool {
	return strings.Contains(sql, `WHERE a.name = $1 AND e.key = $2`) &&
		len(args) == 2 &&
		args[0] == "crm" &&
		args[1] == "lead-contact"
}

func (q *fakeRecordQueryer) fakeCollectionMetadataRows(sql string, args ...any) (pgx.Rows, bool) {
	if len(args) != 1 || args[0] != int64(21) {
		return nil, false
	}
	switch {
	case strings.Contains(sql, `FROM "field"`):
		rows := q.collectionFieldRows
		if rows == nil {
			rows = leadContactFieldRows()
		}
		return newFakeRows(rows), true
	case strings.Contains(sql, `FROM "index"`), strings.Contains(sql, `FROM "constraint"`):
		return newFakeRows(nil), true
	default:
		return nil, false
	}
}

func activityArgs(t *testing.T, queryer *fakeRecordQueryer) []any {
	t.Helper()
	for index := len(queryer.execSQL) - 1; index >= 0; index-- {
		if strings.Contains(queryer.execSQL[index], `INSERT INTO "activity"`) {
			return orderedActivityArgs(t, queryer.execSQL[index], queryer.execArg[index])
		}
	}
	t.Fatal("activity insert was not executed")
	return nil
}

func orderedActivityArgs(t *testing.T, sql string, args []any) []any {
	t.Helper()
	start := strings.Index(sql, `("`)
	end := strings.Index(sql, `) VALUES`)
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("activity insert SQL = %q, want explicit columns", sql)
	}
	columns := strings.Split(sql[start+1:end], ", ")
	values := map[string]any{}
	for index, column := range columns {
		if index >= len(args) {
			t.Fatalf("activity insert args = %#v, want value for column %q", args, column)
		}
		values[strings.Trim(column, `"`)] = args[index]
	}
	orderedColumns := []string{"kind", "operation", "status", "entity_id", "record_id", "actor_id", "title", "message", "changes", "snapshot", "details", "name"}
	ordered := make([]any, len(orderedColumns))
	for index, column := range orderedColumns {
		ordered[index] = values[column]
	}
	return ordered
}

func decodeActivityObject(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, ok := value.(string)
	if !ok {
		t.Fatalf("activity JSON arg type = %T, want string", value)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatalf("Unmarshal(activity object) error = %v", err)
	}
	return decoded
}

func decodeActivityList(t *testing.T, value any) []map[string]any {
	t.Helper()
	encoded, ok := value.(string)
	if !ok {
		t.Fatalf("activity JSON arg type = %T, want string", value)
	}
	var decoded []map[string]any
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatalf("Unmarshal(activity list) error = %v", err)
	}
	return decoded
}

func assertRecordError(t *testing.T, err error, code string, field string) {
	t.Helper()

	var recordErr RecordError
	if !errors.As(err, &recordErr) {
		t.Fatalf("error = %v, want RecordError", err)
	}
	if recordErr.Code != code {
		t.Fatalf("RecordError code = %q, want %q", recordErr.Code, code)
	}
	if field != "" && recordErr.Details["field"] != field {
		t.Fatalf("RecordError details = %#v, want field %q", recordErr.Details, field)
	}
}

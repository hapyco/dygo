package db

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dygo-dev/dygo/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestRecordStoreListRecords(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(1), "a@example.com", now, now, "a@example.com", "A User", true},
		{int64(2), "b@example.com", now, now, "b@example.com", "B User", false},
	}))

	result, err := NewRecordStore(queryer).ListRecords(context.Background(), "user", RecordListParams{})
	if err != nil {
		t.Fatalf("ListRecords() error = %v, want nil", err)
	}
	if result.Limit != 50 || result.Offset != 0 || result.Count != 2 {
		t.Fatalf("ListRecords() result = %+v, want default pagination and two records", result)
	}
	if result.Records[0]["email"] != "a@example.com" || result.Records[0]["full-name"] != "A User" {
		t.Fatalf("ListRecords() first record = %+v, want metadata field names", result.Records[0])
	}
	if _, ok := result.Records[0]["password"]; ok {
		t.Fatalf("ListRecords() returned password field: %+v", result.Records[0])
	}
	lastQuery := queryer.queries[len(queryer.queries)-1]
	if !strings.Contains(lastQuery, `FROM "user"`) || !strings.Contains(lastQuery, `ORDER BY "id" ASC LIMIT $1 OFFSET $2`) {
		t.Fatalf("list query = %q, want deterministic paginated query", lastQuery)
	}
	if got := queryer.args[len(queryer.args)-1]; len(got) != 2 || got[0] != 50 || got[1] != 0 {
		t.Fatalf("list args = %#v, want default limit/offset", got)
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
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{`INSERT INTO "user"`, `"email", "full_name"`, `"name"`, `RETURNING "id", "name", "created_at", "updated_at"`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("create query = %q, want %q", lastQuery, want)
		}
	}
	args := queryer.args[len(queryer.args)-1]
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
	args := queryer.args[len(queryer.args)-1]
	name, ok := args[len(args)-1].(string)
	if !ok {
		t.Fatalf("random name arg type = %T, want string", args[len(args)-1])
	}
	if len(name) != 16 {
		t.Fatalf("random name length = %d, want 16", len(name))
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
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{`UPDATE "user" SET "full_name" = $1`, `"updated_at" = now()`, `WHERE "id" = $2`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("update query = %q, want %q", lastQuery, want)
		}
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
	lastQuery := queryer.queries[len(queryer.queries)-1]
	for _, want := range []string{`"password_hash"`, `RETURNING "id", "name", "created_at", "updated_at", "email", "full_name", "enabled"`} {
		if !strings.Contains(lastQuery, want) {
			t.Fatalf("create query = %q, want %q", lastQuery, want)
		}
	}
	if strings.Contains(lastQuery, `RETURNING "id", "name", "created_at", "updated_at", "email", "full_name", "password_hash"`) {
		t.Fatalf("create query = %q, returned password_hash", lastQuery)
	}
	args := queryer.args[len(queryer.args)-1]
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
	lastQuery := queryer.queries[len(queryer.queries)-1]
	if !strings.Contains(lastQuery, `UPDATE "user" SET "password_hash" = $1`) {
		t.Fatalf("update query = %q, want password_hash update", lastQuery)
	}
	args := queryer.args[len(queryer.args)-1]
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

func TestRecordStoreCreateRecordWritesActivity(t *testing.T) {
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := newUserRecordQueryer()
	queryer.rows = append(queryer.rows, newFakeRows([][]any{
		{int64(7), "a@example.com", now, now, "a@example.com", "A User", true},
	}))

	ctx := WithActivityActor(WithActivitySource(context.Background(), ActivitySourceAPI), 99)
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
	if args[1] != activityOperationCreate || args[3] != int64(10) || args[4] != int64(7) || args[5] != int64(99) {
		t.Fatalf("activity args = %#v, want create for user entity and actor", args)
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
	if args[1] != activityOperationUpdate || args[4] != int64(7) {
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
	if args[1] != activityOperationDelete || args[4] != int64(7) {
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
		{int64(1), "activity-1", now, now, "record", "create", "success", "Created User"},
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
			name:      "child table write",
			entity:    "lead",
			queryer:   newLeadRecordQueryer(),
			input:     recordInput(map[string]string{"status": `"New"`, "contacts": `[]`}),
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
	_, err := NewRecordStore(newUserRecordQueryer()).ListRecords(context.Background(), "user", RecordListParams{Limit: 101})
	assertRecordError(t, err, RecordErrorInvalidRequest, "")

	_, err = NewRecordStore(newUserRecordQueryer()).GetRecord(context.Background(), "user", 0)
	assertRecordError(t, err, RecordErrorInvalidRequest, "")
}

func newUserRecordQueryer() *fakeRecordQueryer {
	return &fakeRecordQueryer{
		row: newFakeRow(int64(10), "user", "user", "User", "User identity", []byte(`{"strategy":"field","field":"email"}`), "core", "Core"),
		rows: []pgx.Rows{
			newFakeRows([][]any{
				{"email", "Email", "email", true, true, false, nil, nil, 1, nil},
				{"full-name", "Full Name", "text", true, false, false, nil, nil, 2, nil},
				{"password", "Password", "password", false, false, false, nil, nil, 3, nil},
				{"enabled", "Enabled", "boolean", false, false, true, []byte("true"), nil, 4, nil},
			}),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}
}

func newLeadRecordQueryer() *fakeRecordQueryer {
	return &fakeRecordQueryer{
		row: newFakeRow(int64(20), "lead", "lead", "Lead", "Sales lead", []byte(`{"strategy":"random","length":16}`), "crm", "CRM"),
		rows: []pgx.Rows{
			newFakeRows([][]any{
				{"status", "Status", "select", true, false, false, nil, nil, 1, []byte(`{"values":["New","Qualified"]}`)},
				{"contacts", "Contacts", "child-table", false, false, false, nil, nil, 2, []byte(`{"entity":"lead-contact"}`)},
			}),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}
}

func newActivityRecordQueryer() *fakeRecordQueryer {
	return &fakeRecordQueryer{
		row: newFakeRow(int64(1), "activity", "activity", "Activity", "Timeline entry", []byte(`{"strategy":"random","length":16}`), "core", "Core"),
		rows: []pgx.Rows{
			newFakeRows([][]any{
				{"kind", "Kind", "select", true, false, true, nil, nil, 1, []byte(`{"values":["record"]}`)},
				{"operation", "Operation", "select", true, false, true, nil, nil, 2, []byte(`{"values":["create"]}`)},
				{"status", "Status", "select", true, false, true, nil, nil, 3, []byte(`{"values":["success"]}`)},
				{"title", "Title", "text", true, false, false, nil, nil, 4, nil},
			}),
			newFakeRows(nil),
			newFakeRows(nil),
		},
	}
}

type fakeRecordQueryer struct {
	row       pgx.Row
	rows      []pgx.Rows
	queryErrs []error
	execTags  []pgconn.CommandTag
	execErrs  []error

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

func activityArgs(t *testing.T, queryer *fakeRecordQueryer) []any {
	t.Helper()
	if len(queryer.execSQL) == 0 {
		t.Fatal("activity insert was not executed")
	}
	index := len(queryer.execSQL) - 1
	if !strings.Contains(queryer.execSQL[index], `INSERT INTO "activity"`) {
		t.Fatalf("last exec SQL = %q, want activity insert", queryer.execSQL[index])
	}
	return queryer.execArg[index]
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

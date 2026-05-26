package fixtures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/schema"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/shape"
)

func TestDiscoverLoadsEntityBundleFixtureFiles(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "core")
	entityDir := filepath.Join(appDir, "entities", "role")
	writeFixtureFile(t, filepath.Join(entityDir, "fixtures.yml"), `
entity: role
match: [name]
records:
  - name: system-manager
    label: System Manager
`)
	writeFixtureFile(t, filepath.Join(appDir, "entities", "_collections", "role-row", "fixtures.yml"), `
entity: role-row
match: [name]
records:
  - name: ignored
`)

	files, err := Discover([]manifest.LoadedApp{{
		Dir: appDir,
		Manifest: manifest.Manifest{
			Name:  "core",
			Paths: manifest.DefaultPaths(),
		},
	}})
	if err != nil {
		t.Fatalf("Discover() error = %v, want nil", err)
	}
	if len(files) != 1 || files[0].AppName != "core" || files[0].Fixture.Entity != "role" {
		t.Fatalf("Discover() files = %+v, want one core role fixture", files)
	}
	if files[0].Path != filepath.Join(entityDir, "fixtures.yml") {
		t.Fatalf("Discover() fixture path = %q, want Entity bundle fixture", files[0].Path)
	}
}

func TestDiscoverRejectsEntityBundleFixtureEntityMismatch(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "core")
	writeFixtureFile(t, filepath.Join(appDir, "entities", "role", "fixtures.yml"), `
entity: permission
match: [name]
records:
  - name: system-manager
`)

	_, err := Discover([]manifest.LoadedApp{{
		Dir: appDir,
		Manifest: manifest.Manifest{
			Name:  "core",
			Paths: manifest.DefaultPaths(),
		},
	}})
	if err == nil {
		t.Fatal("Discover() error = nil, want entity mismatch error")
	}
	if !strings.Contains(err.Error(), `fixture entity "permission" must match Entity bundle "role"`) {
		t.Fatalf("Discover() error = %q, want Entity bundle mismatch", err.Error())
	}
}

func TestDiscoverRequiresEntityNamedBundleFixtureFiles(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "core")
	writeFixtureFile(t, filepath.Join(appDir, "entities", "roles", "fixtures.yml"), `
entity: role
match: [name]
records:
  - name: system-manager
`)

	_, err := Discover([]manifest.LoadedApp{{
		Dir: appDir,
		Manifest: manifest.Manifest{
			Name:  "core",
			Paths: manifest.DefaultPaths(),
		},
	}})
	if err == nil {
		t.Fatal("Discover() error = nil, want Entity bundle mismatch error")
	}
	if !strings.Contains(err.Error(), `fixture entity "role" must match Entity bundle "roles"`) {
		t.Fatalf("Discover() error = %q, want Entity bundle mismatch", err.Error())
	}
}

func TestPlanExportReportsUnresolvedLinksWithoutIncludeLinks(t *testing.T) {
	root := t.TempDir()
	store := fixtureExportStore()
	metadata := fixtureExportMetadata(root)

	plan, err := PlanExport(context.Background(), store, metadata, shape.AppRef{App: "crm", Name: "lead"}, false)
	if err != nil {
		t.Fatalf("PlanExport() error = %v, want nil", err)
	}
	if plan.FileCount() != 1 || plan.RecordCount() != 1 {
		t.Fatalf("PlanExport() counts = %d files %d records, want 1/1", plan.FileCount(), plan.RecordCount())
	}
	if len(plan.UnresolvedLinks) != 1 {
		t.Fatalf("PlanExport() unresolved links = %d, want 1", len(plan.UnresolvedLinks))
	}
	link := plan.UnresolvedLinks[0]
	if link.SourceApp != "crm" || link.SourceEntity != "lead" || link.SourceRecord != "lead-one" || link.Field != "owner" || link.TargetApp != "core" || link.TargetEntity != "user" || link.TargetRecord != "admin" {
		t.Fatalf("PlanExport() unresolved link = %+v, want crm/lead owner -> core/user admin", link)
	}
	content := string(plan.Files[0].Content)
	for _, want := range []string{
		"entity: lead",
		"match:",
		"- name",
		"name: lead-one",
		"title: Lead One",
		"owner:",
		"name: admin",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("export content = %q, want substring %q", content, want)
		}
	}
	if plan.Files[0].ProjectPath != "apps/crm/entities/lead/fixtures.yml" {
		t.Fatalf("PlanExport() path = %q, want canonical Entity fixture path", plan.Files[0].ProjectPath)
	}
}

func TestPlanExportIncludesLinkedRecords(t *testing.T) {
	root := t.TempDir()
	store := fixtureExportStore()
	metadata := fixtureExportMetadata(root)

	plan, err := PlanExport(context.Background(), store, metadata, shape.AppRef{App: "crm", Name: "lead"}, true)
	if err != nil {
		t.Fatalf("PlanExport(include links) error = %v, want nil", err)
	}
	if plan.FileCount() != 2 || plan.RecordCount() != 2 {
		t.Fatalf("PlanExport(include links) counts = %d files %d records, want 2/2", plan.FileCount(), plan.RecordCount())
	}
	if len(plan.UnresolvedLinks) != 0 {
		t.Fatalf("PlanExport(include links) unresolved links = %+v, want none", plan.UnresolvedLinks)
	}
	if plan.Files[0].ProjectPath != "apps/crm/entities/lead/fixtures.yml" || plan.Files[1].ProjectPath != "apps/core/entities/user/fixtures.yml" {
		t.Fatalf("PlanExport(include links) files = %q, %q; want target first then dependency", plan.Files[0].ProjectPath, plan.Files[1].ProjectPath)
	}
	if !strings.Contains(string(plan.Files[1].Content), "full-name: Admin User") {
		t.Fatalf("dependency fixture content = %q, want user record", string(plan.Files[1].Content))
	}
}

func TestWriteExportPlanWritesFixtureFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "apps", "crm", "entities", "lead", "fixtures.yml")
	plan := ExportPlan{Files: []ExportFile{{
		Path:        path,
		ProjectPath: "apps/crm/entities/lead/fixtures.yml",
		Records:     []db.Record{{"name": "lead-one"}},
		Content:     []byte("entity: lead\nmatch: [name]\nrecords:\n  - name: lead-one\n"),
	}}}

	result, err := WriteExportPlan(plan)
	if err != nil {
		t.Fatalf("WriteExportPlan() error = %v, want nil", err)
	}
	if result.FilesWritten != 1 || result.RecordsWritten != 1 {
		t.Fatalf("WriteExportPlan() = %+v, want 1 file 1 record", result)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(exported fixture) error = %v, want nil", err)
	}
	if string(data) != string(plan.Files[0].Content) {
		t.Fatalf("exported fixture = %q, want %q", string(data), string(plan.Files[0].Content))
	}
}

func TestDiscoverAllowsMissingEntitiesDirectory(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "core")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(app) error = %v", err)
	}

	files, err := Discover([]manifest.LoadedApp{{
		Dir: appDir,
		Manifest: manifest.Manifest{
			Name:  "core",
			Paths: manifest.DefaultPaths(),
		},
	}})
	if err != nil {
		t.Fatalf("Discover() error = %v, want nil", err)
	}
	if len(files) != 0 {
		t.Fatalf("Discover() len = %d, want 0", len(files))
	}
}

func TestDecodeRejectsUnknownAndDuplicateFields(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknown top-level field",
			body: `
entity: role
match: [name]
unknown: true
records:
  - name: system-manager
`,
			want: "unknown fixture field",
		},
		{
			name: "duplicate record field",
			body: `
entity: role
match: [name]
records:
  - name: system-manager
    name: duplicate
`,
			want: "duplicate fixture key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode([]byte(tt.body))
			if err == nil {
				t.Fatal("Decode() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Decode() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestApplyFilesCreatesAndUpdatesRecords(t *testing.T) {
	store := newFakeStore()
	store.records["role"] = []db.Record{{"id": int64(1), "name": "system-manager", "label": "Old Label", "enabled": true}}
	files := []LoadedFile{loadedFixture(t, "roles.yml", `
entity: role
match: [name]
records:
  - name: system-manager
    label: System Manager
    enabled: true
  - name: sales-user
    label: Sales User
    enabled: true
`)}

	result, err := ApplyFiles(context.Background(), store, files)
	if err != nil {
		t.Fatalf("ApplyFiles() error = %v, want nil", err)
	}
	if result.Created != 1 || result.Updated != 1 {
		t.Fatalf("ApplyFiles() result = %+v, want 1 created and 1 updated", result)
	}
	if len(store.records["role"]) != 2 {
		t.Fatalf("role records len = %d, want 2", len(store.records["role"]))
	}
	if store.records["role"][0]["label"] != "System Manager" {
		t.Fatalf("updated role = %+v, want new label", store.records["role"][0])
	}
	if len(store.createSources) != 1 || store.createSources[0] != db.ActivitySourceFixtures {
		t.Fatalf("create activity sources = %#v, want fixtures source", store.createSources)
	}
	if len(store.updateSources) != 1 || store.updateSources[0] != db.ActivitySourceFixtures {
		t.Fatalf("update activity sources = %#v, want fixtures source", store.updateSources)
	}
	if _, ok := store.updateInputs[0]["name"]; ok {
		t.Fatalf("update input includes immutable name field: %#v", store.updateInputs[0])
	}
}

func TestApplyFilesRejectsNonUniqueMatch(t *testing.T) {
	store := newFakeStore()
	file := loadedFixture(t, "roles.yml", `
entity: role
match: [label]
records:
  - name: system-manager
    label: System Manager
`)

	_, err := ApplyFiles(context.Background(), store, []LoadedFile{file})
	if err == nil {
		t.Fatal("ApplyFiles() error = nil, want non-unique match error")
	}
	if !strings.Contains(err.Error(), "not backed by a unique field or constraint") {
		t.Fatalf("ApplyFiles() error = %q, want unique match error", err.Error())
	}
}

func TestApplyFilesResolvesLinkReferences(t *testing.T) {
	store := newFakeStore()
	store.records["entity"] = []db.Record{{"id": int64(10), "name": "core.user"}}
	store.records["role"] = []db.Record{{"id": int64(20), "name": "system-manager"}}
	file := loadedFixture(t, "permissions.yml", `
entity: permission
match: [entity, role]
records:
  - entity:
      match:
        name: core.user
    role:
      match:
        name: system-manager
    read: true
    create: true
`)

	result, err := ApplyFiles(context.Background(), store, []LoadedFile{file})
	if err != nil {
		t.Fatalf("ApplyFiles() error = %v, want nil", err)
	}
	if result.Created != 1 || result.Updated != 0 {
		t.Fatalf("ApplyFiles() result = %+v, want one created permission", result)
	}
	created := store.records["permission"][0]
	if created["entity"] != "core.user" || created["role"] != "system-manager" || created["read"] != true {
		t.Fatalf("created permission = %+v, want resolved link names", created)
	}
}

func TestApplyFilesOrdersByLinkDependencies(t *testing.T) {
	store := newFakeStore()
	seedEntityRecords(store)
	permission := loadedFixture(t, "permission.yml", `
entity: permission
match: [entity, role]
records:
  - entity:
      match:
        name: core.user
    role:
      match:
        name: system-manager
    read: true
`)
	role := loadedFixture(t, "role.yml", `
entity: role
match: [name]
records:
  - name: system-manager
    label: System Manager
`)

	result, err := ApplyFiles(context.Background(), store, []LoadedFile{permission, role})
	if err != nil {
		t.Fatalf("ApplyFiles() error = %v, want nil", err)
	}
	if result.Created != 2 || result.Updated != 0 {
		t.Fatalf("ApplyFiles() result = %+v, want role and permission created", result)
	}
}

func TestApplyFilesRejectsInvalidFixtureRecord(t *testing.T) {
	store := newFakeStore()
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknown field",
			body: `
entity: role
match: [name]
records:
  - name: system-manager
    missing: true
`,
			want: "unknown fixture field",
		},
		{
			name: "unsupported collection",
			body: `
entity: lead
match: [name]
records:
  - name: sample
    contacts: []
`,
			want: "unsupported collection",
		},
		{
			name: "missing link target",
			body: `
entity: permission
match: [entity, role]
records:
  - entity:
      match:
        name: user
    role:
      match:
        name: missing
`,
			want: "record not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ApplyFiles(context.Background(), store, []LoadedFile{loadedFixture(t, tt.name+".yml", tt.body)})
			if err == nil {
				t.Fatal("ApplyFiles() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ApplyFiles() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func TestApplyFilesDoesNotLeakSensitiveRecordErrors(t *testing.T) {
	store := newFakeStore()
	store.createErr = db.RecordError{Code: db.RecordErrorInternal, Message: "record query failed", Err: errors.New("postgres://user:secret@localhost failed")}
	file := loadedFixture(t, "roles.yml", `
entity: role
match: [name]
records:
  - name: system-manager
    label: System Manager
`)

	_, err := ApplyFiles(context.Background(), store, []LoadedFile{file})
	if err == nil {
		t.Fatal("ApplyFiles() error = nil, want error")
	}
	if strings.Contains(err.Error(), "postgres://") || strings.Contains(err.Error(), "secret") {
		t.Fatalf("ApplyFiles() error leaked sensitive detail: %q", err.Error())
	}
}

func TestNewRunnerWithHooksStoresRecordHooks(t *testing.T) {
	t.Parallel()

	recordHooks := db.NewRecordHookRegistry()
	runner := NewRunnerWithHooks(recordHooks)
	if runner.recordHooks != recordHooks {
		t.Fatal("NewRunnerWithHooks() did not retain configured Record hooks")
	}
}

type fakeStore struct {
	metadata      map[string]db.MetadataEntityMeta
	records       map[string][]db.Record
	createErr     error
	nextID        int64
	createSources []string
	updateSources []string
	updateInputs  []db.RecordInput
}

func newFakeStore() *fakeStore {
	store := &fakeStore{
		metadata: map[string]db.MetadataEntityMeta{},
		records:  map[string][]db.Record{},
		nextID:   100,
	}
	store.metadata["role"] = db.MetadataEntityMeta{
		MetadataEntity: db.MetadataEntity{Name: "role"},
		Fields: []db.MetadataField{
			{Name: "name", Type: "text", Unique: true, Required: true},
			{Name: "label", Type: "text", Required: true},
			{Name: "description", Type: "long-text"},
			{Name: "enabled", Type: "boolean"},
		},
	}
	store.metadata["entity"] = db.MetadataEntityMeta{
		MetadataEntity: db.MetadataEntity{Name: "entity"},
		Fields: []db.MetadataField{
			{Name: "name", Type: "text", Unique: true, Required: true},
			{Name: "label", Type: "text"},
		},
	}
	store.metadata["country"] = db.MetadataEntityMeta{
		MetadataEntity: db.MetadataEntity{Name: "country"},
		Fields: []db.MetadataField{
			{Name: "name", Type: "text", Unique: true, Required: true},
			{Name: "code", Type: "text", Unique: true, Required: true},
		},
	}
	store.metadata["currency"] = db.MetadataEntityMeta{
		MetadataEntity: db.MetadataEntity{Name: "currency"},
		Fields: []db.MetadataField{
			{Name: "code", Type: "text", Unique: true, Required: true},
			{Name: "numeric-code", Type: "text"},
			{Name: "display-name", Type: "text"},
			{Name: "symbol", Type: "text"},
			{Name: "minor-unit-digits", Type: "int"},
			{Name: "cash-rounding-increment", Type: "decimal"},
			{Name: "enabled", Type: "boolean"},
		},
	}
	store.metadata["language"] = db.MetadataEntityMeta{
		MetadataEntity: db.MetadataEntity{Name: "language"},
		Fields: []db.MetadataField{
			{Name: "name", Type: "text", Unique: true, Required: true},
			{Name: "code", Type: "text", Unique: true, Required: true},
			{Name: "enabled", Type: "boolean"},
		},
	}
	store.metadata["permission"] = db.MetadataEntityMeta{
		MetadataEntity: db.MetadataEntity{Name: "permission"},
		Fields: []db.MetadataField{
			{Name: "entity", Type: "link", Required: true, Options: json.RawMessage(`{"entity":"entity"}`)},
			{Name: "role", Type: "link", Required: true, Options: json.RawMessage(`{"entity":"role"}`)},
			{Name: "read", Type: "boolean"},
			{Name: "create", Type: "boolean"},
			{Name: "update", Type: "boolean"},
			{Name: "delete", Type: "boolean"},
			{Name: "export", Type: "boolean"},
			{Name: "print", Type: "boolean"},
		},
		Constraints: []db.MetadataConstraint{{
			Name:   "permission_entity_role_key",
			Type:   "unique",
			Fields: json.RawMessage(`["entity","role"]`),
		}},
	}
	store.metadata["lead"] = db.MetadataEntityMeta{
		MetadataEntity: db.MetadataEntity{Name: "lead"},
		Fields: []db.MetadataField{
			{Name: "name", Type: "text", Unique: true, Required: true},
			{Name: "contacts", Type: "collection"},
		},
	}
	return store
}

func seedEntityRecords(store *fakeStore) {
	names := []string{"activity", "app", "configuration", "constraint", "country", "currency", "entity", "field", "index", "language", "naming-series", "patch-run", "permission", "role", "session", "user", "user-role"}
	for i, name := range names {
		store.records["entity"] = append(store.records["entity"], db.Record{"id": int64(i + 1), "name": "core." + name})
	}
}

func (s *fakeStore) GetEntityMeta(_ context.Context, entity string) (db.MetadataEntityMeta, error) {
	meta, ok := s.metadata[entity]
	if !ok {
		return db.MetadataEntityMeta{}, db.MetadataNotFoundError{Kind: "entity", Name: entity}
	}
	return meta, nil
}

func (s *fakeStore) GetEntityMetaByIdentity(_ context.Context, appName string, entity string) (db.MetadataEntityMeta, error) {
	meta, ok := s.metadata[entity]
	if !ok {
		return db.MetadataEntityMeta{}, db.MetadataNotFoundError{Kind: "entity", Name: appName + "/" + entity}
	}
	meta.App.Name = appName
	meta.Key = entity
	if meta.Name == "" {
		meta.Name = entity
	}
	return meta, nil
}

func (s *fakeStore) ListRecordsByIdentity(_ context.Context, _ string, entity string, params db.RecordListParams) (db.RecordListResult, error) {
	records := append([]db.Record(nil), s.records[entity]...)
	if len(params.Filters) > 0 {
		filtered := []db.Record{}
		for _, record := range records {
			matched := true
			for _, filter := range params.Filters {
				if fmt.Sprint(record[filter.Field]) != filter.Value {
					matched = false
					break
				}
			}
			if matched {
				filtered = append(filtered, record)
			}
		}
		records = filtered
	}
	limit := params.Limit
	if limit == 0 || limit > len(records) {
		limit = len(records)
	}
	offset := params.Offset
	if offset > len(records) {
		offset = len(records)
	}
	end := offset + limit
	if end > len(records) {
		end = len(records)
	}
	page := append([]db.Record(nil), records[offset:end]...)
	return db.RecordListResult{Records: page, Limit: limit, Offset: offset, Count: len(page), Total: len(records)}, nil
}

func (s *fakeStore) FindRecord(_ context.Context, entity string, match db.RecordInput) (db.Record, error) {
	var found []db.Record
	for _, record := range s.records[entity] {
		if recordMatches(record, match) {
			found = append(found, record)
		}
	}
	if len(found) == 0 {
		return nil, db.RecordError{Code: db.RecordErrorNotFound, Message: "record not found"}
	}
	if len(found) > 1 {
		return nil, db.RecordError{Code: db.RecordErrorValidation, Message: "record match is ambiguous"}
	}
	return found[0], nil
}

func (s *fakeStore) CreateRecord(ctx context.Context, entity string, input db.RecordInput) (db.Record, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	source, _ := db.ActivitySourceFromContext(ctx)
	s.createSources = append(s.createSources, source)
	s.nextID++
	record := recordFromInput(input)
	record["id"] = s.nextID
	s.records[entity] = append(s.records[entity], record)
	return record, nil
}

func (s *fakeStore) UpdateRecord(ctx context.Context, entity string, id int64, input db.RecordInput) (db.Record, error) {
	for _, record := range s.records[entity] {
		if record["id"] == id {
			source, _ := db.ActivitySourceFromContext(ctx)
			s.updateSources = append(s.updateSources, source)
			s.updateInputs = append(s.updateInputs, input)
			for key, value := range recordFromInput(input) {
				record[key] = value
			}
			return record, nil
		}
	}
	return nil, db.RecordError{Code: db.RecordErrorNotFound, Message: "record not found"}
}

func loadedFixture(t *testing.T, name string, body string) LoadedFile {
	t.Helper()
	fixture, err := Decode([]byte(body))
	if err != nil {
		t.Fatalf("Decode(%s) error = %v", name, err)
	}
	return LoadedFile{AppName: "core", AppDir: "/tmp/core", Path: name, Fixture: fixture}
}

func writeFixtureFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func recordMatches(record db.Record, input db.RecordInput) bool {
	for field, raw := range input {
		value := rawValue(raw)
		if record[field] != value {
			return false
		}
	}
	return true
}

func recordFromInput(input db.RecordInput) db.Record {
	record := db.Record{}
	for field, raw := range input {
		record[field] = rawValue(raw)
	}
	return record
}

func rawValue(raw json.RawMessage) any {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	if number, ok := value.(float64); ok && number == float64(int64(number)) {
		return int64(number)
	}
	return value
}

func fixtureExportStore() *fakeStore {
	store := newFakeStore()
	store.metadata["lead"] = db.MetadataEntityMeta{
		MetadataEntity: db.MetadataEntity{
			Name: "lead",
			Key:  "lead",
			App:  db.MetadataAppRef{Name: "crm"},
		},
		Fields: []db.MetadataField{
			{Name: "title", Type: "text", Required: true},
			{Name: "owner", Type: "link", Options: json.RawMessage(`{"app":"core","entity":"user"}`)},
		},
	}
	store.metadata["user"] = db.MetadataEntityMeta{
		MetadataEntity: db.MetadataEntity{
			Name: "user",
			Key:  "user",
			App:  db.MetadataAppRef{Name: "core"},
		},
		Fields: []db.MetadataField{
			{Name: "full-name", Type: "text", Required: true},
		},
	}
	store.records["lead"] = []db.Record{{
		"id":    int64(1),
		"name":  "lead-one",
		"title": "Lead One",
		"owner": "admin",
	}}
	store.records["user"] = []db.Record{{
		"id":        int64(10),
		"name":      "admin",
		"full-name": "Admin User",
	}}
	return store
}

func fixtureExportMetadata(root string) project.Metadata {
	crmDir := filepath.Join(root, "apps", "crm")
	coreDir := filepath.Join(root, "apps", "core")
	return project.Metadata{
		Entities: []catalog.LoadedEntity{
			{
				AppName: "crm",
				AppDir:  crmDir,
				Path:    filepath.Join(crmDir, "entities", "lead", "entity.yml"),
				Entity:  schema.Entity{Name: "lead", Label: "Lead", Fields: []schema.Field{{Name: "title", Label: "Title", Type: "text"}}},
			},
			{
				AppName: "core",
				AppDir:  coreDir,
				Path:    filepath.Join(coreDir, "entities", "user", "entity.yml"),
				Entity:  schema.Entity{Name: "user", Label: "User", Fields: []schema.Field{{Name: "full-name", Label: "Full Name", Type: "text"}}},
			},
		},
	}
}

package fixtures

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/app/manifest"
	"github.com/dygo-dev/dygo/internal/db"
)

func TestDiscoverLoadsAppFixtureFiles(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "core")
	fixturesDir := filepath.Join(appDir, "fixtures")
	if err := os.MkdirAll(filepath.Join(fixturesDir, "nested"), 0o755); err != nil {
		t.Fatalf("MkdirAll(fixtures) error = %v", err)
	}
	writeFixtureFile(t, filepath.Join(fixturesDir, "role.yml"), `
entity: role
match: [name]
records:
  - name: system-manager
    label: System Manager
`)
	writeFixtureFile(t, filepath.Join(fixturesDir, "notes.txt"), `ignored`)
	writeFixtureFile(t, filepath.Join(fixturesDir, "nested", "ignored.yml"), `
entity: role
match: [name]
records:
  - name: nested
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
}

func TestDiscoverRequiresEntityNamedFixtureFiles(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "apps", "core")
	fixturesDir := filepath.Join(appDir, "fixtures")
	if err := os.MkdirAll(fixturesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(fixtures) error = %v", err)
	}
	writeFixtureFile(t, filepath.Join(fixturesDir, "roles.yml"), `
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
		t.Fatal("Discover() error = nil, want file/entity mismatch error")
	}
	if !strings.Contains(err.Error(), `fixture entity "role" must match file name "roles"`) {
		t.Fatalf("Discover() error = %q, want entity/file mismatch", err.Error())
	}
}

func TestDiscoverAllowsMissingFixtureDirectory(t *testing.T) {
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
	store.records["entity"] = []db.Record{{"id": int64(10), "name": "user"}}
	store.records["role"] = []db.Record{{"id": int64(20), "name": "system-manager"}}
	file := loadedFixture(t, "permissions.yml", `
entity: permission
match: [entity, role]
records:
  - entity:
      match:
        name: user
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
	if created["entity"] != int64(10) || created["role"] != int64(20) || created["read"] != true {
		t.Fatalf("created permission = %+v, want resolved link ids", created)
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
        name: user
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

func TestRepositoryCoreFixturesApply(t *testing.T) {
	store := newFakeStore()
	seedEntityRecords(store)

	fixtureDir := filepath.Join("..", "..", "apps", "core", "fixtures")
	entries, err := os.ReadDir(fixtureDir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", fixtureDir, err)
	}
	var files []LoadedFile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		path := filepath.Join(fixtureDir, entry.Name())
		fixture, err := LoadFile(path)
		if err != nil {
			t.Fatalf("LoadFile(%s) error = %v", path, err)
		}
		files = append(files, LoadedFile{AppName: "core", AppDir: filepath.Join("..", "..", "apps", "core"), Path: path, Fixture: fixture})
	}

	result, err := ApplyFiles(context.Background(), store, files)
	if err != nil {
		t.Fatalf("ApplyFiles(core fixtures) error = %v, want nil", err)
	}
	if result.Created != 19 || result.Updated != 0 {
		t.Fatalf("ApplyFiles(core fixtures) result = %+v, want 19 created", result)
	}

	result, err = ApplyFiles(context.Background(), store, files)
	if err != nil {
		t.Fatalf("ApplyFiles(core fixtures second run) error = %v, want nil", err)
	}
	if result.Created != 0 || result.Updated != 19 {
		t.Fatalf("ApplyFiles(core fixtures second run) result = %+v, want 19 updated", result)
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
			name: "unsupported child table",
			body: `
entity: lead
match: [name]
records:
  - name: sample
    contacts: []
`,
			want: "unsupported child-table",
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
			{Name: "contacts", Type: "child-table"},
		},
	}
	return store
}

func seedEntityRecords(store *fakeStore) {
	names := []string{"activity", "app", "constraint", "entity", "field", "index", "naming-series", "permission", "role", "session", "user", "user-role"}
	for i, name := range names {
		store.records["entity"] = append(store.records["entity"], db.Record{"id": int64(i + 1), "name": name})
	}
}

func (s *fakeStore) GetEntityMeta(_ context.Context, entity string) (db.MetadataEntityMeta, error) {
	meta, ok := s.metadata[entity]
	if !ok {
		return db.MetadataEntityMeta{}, db.MetadataNotFoundError{Kind: "entity", Name: entity}
	}
	return meta, nil
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

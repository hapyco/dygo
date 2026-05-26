// Package fixtures applies app-owned seed data through the Record runtime.
package fixtures

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/yamlmeta"
	"gopkg.in/yaml.v3"
)

// Result reports how many fixture records changed runtime state.
type Result struct {
	Created int
	Updated int
}

// Runner applies discovered fixture files.
type Runner struct {
	recordHooks *db.RecordHookRegistry
}

// LoadedFile is one fixture file loaded from an App.
type LoadedFile struct {
	AppName string
	AppDir  string
	Path    string
	Fixture Fixture
}

// Fixture is one per-Entity fixture document.
type Fixture struct {
	Entity  string
	Match   []string
	Records []Record
	Line    int
}

// Record is one fixture record.
type Record struct {
	Values map[string]Value
	Line   int
}

// Value is one YAML fixture value with source context.
type Value struct {
	Node yaml.Node
	Line int
}

// Store is the metadata and Record behavior needed by fixture apply.
type Store interface {
	GetEntityMeta(context.Context, string) (db.MetadataEntityMeta, error)
	FindRecord(context.Context, string, db.RecordInput) (db.Record, error)
	CreateRecord(context.Context, string, db.RecordInput) (db.Record, error)
	UpdateRecord(context.Context, string, int64, db.RecordInput) (db.Record, error)
}

type runtimeStore struct {
	metadata db.MetadataReader
	records  db.RecordStore
}

// NewRunner returns the default fixture runner.
func NewRunner() Runner {
	return Runner{}
}

// NewRunnerWithHooks returns a fixture runner using compiled Record hooks.
func NewRunnerWithHooks(recordHooks *db.RecordHookRegistry) Runner {
	return Runner{recordHooks: recordHooks}
}

// Apply discovers and applies all app-owned fixtures in one transaction.
func (r Runner) Apply(ctx context.Context, root string, databaseURL string) (Result, error) {
	apps, err := project.LoadApps(root)
	if err != nil {
		return Result{}, fmt.Errorf("validate apps for fixtures: %w", err)
	}
	files, err := Discover(apps)
	if err != nil {
		return Result{}, err
	}
	if len(files) == 0 {
		return Result{}, nil
	}

	pool, err := db.OpenRuntimePool(ctx, databaseURL)
	if err != nil {
		return Result{}, fmt.Errorf("open fixtures database: %w", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin fixtures transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	store := runtimeStore{
		metadata: db.NewMetadataReader(tx),
		records:  newFixtureRecordStore(tx, r.recordHooks),
	}
	result, err := ApplyFiles(ctx, store, files)
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("commit fixtures transaction: %w", err)
	}
	return result, nil
}

func newFixtureRecordStore(queryer db.RecordQueryer, recordHooks *db.RecordHookRegistry) db.RecordStore {
	if recordHooks != nil {
		return db.NewRecordStoreWithHooks(queryer, recordHooks)
	}
	return db.NewRecordStore(queryer)
}

// Discover loads fixture files from each app's canonical Entity bundles and legacy fixtures path.
func Discover(apps []manifest.LoadedApp) ([]LoadedFile, error) {
	var files []LoadedFile
	for _, app := range apps {
		appFiles, err := discoverAppFixtures(app)
		if err != nil {
			return nil, err
		}
		files = append(files, appFiles...)
	}
	sort.SliceStable(files, func(i, j int) bool {
		if files[i].AppName == files[j].AppName {
			return files[i].Path < files[j].Path
		}
		return files[i].AppName < files[j].AppName
	})
	return files, nil
}

func discoverAppFixtures(app manifest.LoadedApp) ([]LoadedFile, error) {
	var files []LoadedFile
	seen := map[string]LoadedFile{}
	addFile := func(file LoadedFile) error {
		key := file.Fixture.Entity
		if previous, ok := seen[key]; ok {
			return fmt.Errorf("fixture for app %q entity %q is defined twice. Use either %s or %s.", app.Manifest.Name, key, appRelativePath(app.Dir, previous.Path), appRelativePath(app.Dir, file.Path))
		}
		seen[key] = file
		files = append(files, file)
		return nil
	}

	canonical, err := discoverEntityBundleFixtures(app)
	if err != nil {
		return nil, err
	}
	for _, file := range canonical {
		if err := addFile(file); err != nil {
			return nil, err
		}
	}

	legacy, err := discoverLegacyFixtureFiles(app)
	if err != nil {
		return nil, err
	}
	for _, file := range legacy {
		if err := addFile(file); err != nil {
			return nil, err
		}
	}

	return files, nil
}

func discoverEntityBundleFixtures(app manifest.LoadedApp) ([]LoadedFile, error) {
	entitiesDir := filepath.Join(app.Dir, filepath.FromSlash(app.Manifest.Paths.WithDefaults().Entities))
	entries, err := os.ReadDir(entitiesDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read entities for fixture discovery in app %q: %w", app.Manifest.Name, err)
	}

	var files []LoadedFile
	for _, entry := range entries {
		if !entry.IsDir() || isFixtureCollectionDir(entry.Name()) {
			continue
		}
		path := filepath.Join(entitiesDir, entry.Name(), shape.EntityFixturesFile)
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("stat fixture for app %q from %s: %w", app.Manifest.Name, path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		file, err := loadDiscoveredFixture(app, path, entry.Name(), "Entity bundle")
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func discoverLegacyFixtureFiles(app manifest.LoadedApp) ([]LoadedFile, error) {
	fixturesDir := filepath.Join(app.Dir, filepath.FromSlash(app.Manifest.Paths.WithDefaults().Fixtures))
	entries, err := os.ReadDir(fixturesDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read fixtures for app %q: %w", app.Manifest.Name, err)
	}

	var files []LoadedFile
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		path := filepath.Join(fixturesDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat fixture for app %q from %s: %w", app.Manifest.Name, path, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		expectedEntity := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		file, err := loadDiscoveredFixture(app, path, expectedEntity, "file name")
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func loadDiscoveredFixture(app manifest.LoadedApp, path string, expectedEntity string, expectedSource string) (LoadedFile, error) {
	fixture, err := LoadFile(path)
	if err != nil {
		return LoadedFile{}, fmt.Errorf("%s: %w", path, err)
	}
	if fixture.Entity != expectedEntity {
		return LoadedFile{}, fmt.Errorf("%s: fixture entity %q must match %s %q", path, fixture.Entity, expectedSource, expectedEntity)
	}
	return LoadedFile{
		AppName: app.Manifest.Name,
		AppDir:  app.Dir,
		Path:    path,
		Fixture: fixture,
	}, nil
}

func isFixtureCollectionDir(name string) bool {
	return name == shape.CollectionDir || name == "collections"
}

func appRelativePath(appDir string, path string) string {
	relative, err := filepath.Rel(appDir, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

// LoadFile loads one fixture file.
func LoadFile(path string) (Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Fixture{}, err
	}
	return Decode(data)
}

// Decode decodes one fixture document.
func Decode(data []byte) (Fixture, error) {
	var root yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return Fixture{}, fmt.Errorf("fixture file is empty")
		}
		return Fixture{}, err
	}
	if err := yamlmeta.RejectDuplicateKeys(&root, func(duplicate yamlmeta.DuplicateKey) error {
		return fmt.Errorf("duplicate fixture key %q at %s line %d, previously defined at line %d", duplicate.Key, strings.TrimSuffix(duplicate.Location, "."+duplicate.Key), duplicate.Line, duplicate.PreviousLine)
	}); err != nil {
		return Fixture{}, err
	}
	document := yamlmeta.DocumentMapping(&root)
	if document == nil {
		return Fixture{}, fmt.Errorf("fixture document must be a mapping")
	}

	fixture := Fixture{Line: document.Line}
	seen := map[string]bool{}
	for i := 0; i < len(document.Content); i += 2 {
		key := document.Content[i]
		value := document.Content[i+1]
		seen[key.Value] = true
		switch key.Value {
		case "entity":
			entity, err := yamlmeta.ScalarString(value, "entity")
			if err != nil {
				return Fixture{}, err
			}
			fixture.Entity = entity
		case "match":
			match, err := yamlmeta.ScalarStringSequence(value, "match")
			if err != nil {
				return Fixture{}, err
			}
			fixture.Match = match
		case "records":
			records, err := decodeRecords(value)
			if err != nil {
				return Fixture{}, err
			}
			fixture.Records = records
		default:
			return Fixture{}, fmt.Errorf("unknown fixture field %q at line %d", key.Value, key.Line)
		}
	}
	if !seen["entity"] || strings.TrimSpace(fixture.Entity) == "" {
		return Fixture{}, fmt.Errorf("fixture entity is required")
	}
	if !seen["match"] || len(fixture.Match) == 0 {
		return Fixture{}, fmt.Errorf("fixture match is required")
	}
	if !seen["records"] || len(fixture.Records) == 0 {
		return Fixture{}, fmt.Errorf("fixture records are required")
	}
	return fixture, nil
}

// ApplyFiles validates and applies loaded fixtures through store.
func ApplyFiles(ctx context.Context, store Store, files []LoadedFile) (Result, error) {
	if store == nil {
		return Result{}, fmt.Errorf("fixture store is required")
	}
	ctx = db.WithActivitySource(ctx, db.ActivitySourceFixtures)
	prepared := make([]preparedFile, 0, len(files))
	for _, file := range files {
		parsed, err := prepareFile(ctx, store, file)
		if err != nil {
			return Result{}, err
		}
		prepared = append(prepared, parsed)
	}
	ordered, err := orderPreparedFiles(prepared)
	if err != nil {
		return Result{}, err
	}

	var result Result
	for _, file := range ordered {
		for _, record := range file.Fixture.Records {
			input, match, err := resolveRecord(ctx, store, file, record)
			if err != nil {
				return Result{}, err
			}
			existing, err := store.FindRecord(ctx, file.Fixture.Entity, match)
			if isRecordNotFound(err) {
				if _, err := store.CreateRecord(ctx, file.Fixture.Entity, input); err != nil {
					return Result{}, safeWrap("create fixture record", err)
				}
				result.Created++
				continue
			}
			if err != nil {
				return Result{}, safeWrap("find fixture record", err)
			}
			id, err := recordID(existing)
			if err != nil {
				return Result{}, err
			}
			updateInput := fixtureUpdateInput(input)
			if len(updateInput) == 0 {
				continue
			}
			if _, err := store.UpdateRecord(ctx, file.Fixture.Entity, id, updateInput); err != nil {
				return Result{}, safeWrap("update fixture record", err)
			}
			result.Updated++
		}
	}
	return result, nil
}

func fixtureUpdateInput(input db.RecordInput) db.RecordInput {
	update := make(db.RecordInput, len(input))
	for key, value := range input {
		if key == "name" {
			continue
		}
		update[key] = value
	}
	return update
}

func (s runtimeStore) GetEntityMeta(ctx context.Context, entity string) (db.MetadataEntityMeta, error) {
	return s.metadata.GetEntityMeta(ctx, entity)
}

func (s runtimeStore) FindRecord(ctx context.Context, entity string, match db.RecordInput) (db.Record, error) {
	return s.records.FindRecord(ctx, entity, match)
}

func (s runtimeStore) CreateRecord(ctx context.Context, entity string, input db.RecordInput) (db.Record, error) {
	return s.records.CreateRecord(ctx, entity, input)
}

func (s runtimeStore) UpdateRecord(ctx context.Context, entity string, id int64, input db.RecordInput) (db.Record, error) {
	return s.records.UpdateRecord(ctx, entity, id, input)
}

type preparedFile struct {
	LoadedFile
	Meta   db.MetadataEntityMeta
	Fields map[string]db.MetadataField
}

func prepareFile(ctx context.Context, store Store, file LoadedFile) (preparedFile, error) {
	meta, err := store.GetEntityMeta(ctx, file.Fixture.Entity)
	if err != nil {
		return preparedFile{}, safeWrap(fmt.Sprintf("%s: load fixture entity %q", file.Path, file.Fixture.Entity), err)
	}
	fields := db.MetadataFieldsByName(meta)
	if err := db.ValidateRecordMatch(meta, file.Fixture.Match); err != nil {
		return preparedFile{}, fmt.Errorf("%s: %w", file.Path, err)
	}
	for _, record := range file.Fixture.Records {
		if err := validateRecord(file, fields, record); err != nil {
			return preparedFile{}, err
		}
	}
	return preparedFile{LoadedFile: file, Meta: meta, Fields: fields}, nil
}

func orderPreparedFiles(files []preparedFile) ([]preparedFile, error) {
	fixturesByEntity := map[string][]int{}
	for i, file := range files {
		fixturesByEntity[file.Fixture.Entity] = append(fixturesByEntity[file.Fixture.Entity], i)
	}

	dependencies := map[int]map[int]bool{}
	for i, file := range files {
		for _, record := range file.Fixture.Records {
			for name := range record.Values {
				field, ok := db.RecordAddressableFieldByName(file.Fields, name)
				if !ok {
					continue
				}
				if field.Type != "link" {
					continue
				}
				target, err := db.LinkFieldTarget(field)
				if err != nil {
					return nil, fmt.Errorf("%s: fixture field %q: %w", file.Path, name, err)
				}
				for _, targetIndex := range fixturesByEntity[target] {
					if targetIndex == i {
						continue
					}
					if dependencies[i] == nil {
						dependencies[i] = map[int]bool{}
					}
					dependencies[i][targetIndex] = true
				}
			}
		}
	}

	pending := map[int]bool{}
	for i := range files {
		pending[i] = true
	}

	ordered := make([]preparedFile, 0, len(files))
	for len(pending) > 0 {
		progressed := false
		for i, file := range files {
			if !pending[i] {
				continue
			}
			blocked := false
			for dependency := range dependencies[i] {
				if pending[dependency] {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
			ordered = append(ordered, file)
			delete(pending, i)
			progressed = true
		}
		if !progressed {
			names := make([]string, 0, len(pending))
			for i := range pending {
				names = append(names, files[i].Fixture.Entity)
			}
			sort.Strings(names)
			return nil, fmt.Errorf("fixture dependency cycle among entities: %s", strings.Join(names, ", "))
		}
	}

	return ordered, nil
}

func validateRecord(file LoadedFile, fields map[string]db.MetadataField, record Record) error {
	for _, match := range file.Fixture.Match {
		value, ok := record.Values[match]
		if !ok {
			return fmt.Errorf("%s:%d: fixture record is missing match field %q", file.Path, record.Line, match)
		}
		if isNullNode(value.Node) {
			return fmt.Errorf("%s:%d: fixture match field %q cannot be null", file.Path, value.Line, match)
		}
	}
	for name, value := range record.Values {
		field, ok := db.RecordAddressableFieldByName(fields, name)
		if !ok {
			return fmt.Errorf("%s:%d: unknown fixture field %q", file.Path, value.Line, name)
		}
		if !db.MetadataFieldStored(field) {
			// TODO(collections): teach fixtures to upsert owned child rows through the parent Record payload.
			return fmt.Errorf("%s:%d: fixture field %q uses unsupported collection storage", file.Path, value.Line, name)
		}
		if field.Type == "link" {
			if _, err := decodeLinkReference(value.Node); err != nil {
				return fmt.Errorf("%s:%d: link fixture field %q: %w", file.Path, value.Line, name, err)
			}
		}
	}
	return nil
}

func resolveRecord(ctx context.Context, store Store, file preparedFile, record Record) (db.RecordInput, db.RecordInput, error) {
	input := db.RecordInput{}
	match := db.RecordInput{}
	for name, value := range record.Values {
		field, ok := db.RecordAddressableFieldByName(file.Fields, name)
		if !ok {
			return nil, nil, fmt.Errorf("%s:%d: unknown fixture field %q", file.Path, value.Line, name)
		}
		raw, err := resolveValue(ctx, store, field, value, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("%s:%d: resolve fixture field %q: %w", file.Path, value.Line, name, err)
		}
		input[name] = raw
		if stringInSlice(name, file.Fixture.Match) {
			match[name] = raw
		}
	}
	return input, match, nil
}

func resolveValue(ctx context.Context, store Store, field db.MetadataField, value Value, depth int) (json.RawMessage, error) {
	if field.Type != "link" {
		raw, err := nodeJSON(value.Node)
		if err != nil {
			return nil, err
		}
		return raw, nil
	}
	if depth > 8 {
		return nil, fmt.Errorf("link reference nesting is too deep")
	}
	reference, err := decodeLinkReference(value.Node)
	if err != nil {
		return nil, err
	}
	target, err := db.LinkFieldTarget(field)
	if err != nil {
		return nil, err
	}
	targetMeta, err := store.GetEntityMeta(ctx, target)
	if err != nil {
		return nil, safeWrap(fmt.Sprintf("load link target entity %q", target), err)
	}
	targetFields := db.MetadataFieldsByName(targetMeta)
	matchNames := sortedValueKeys(reference.Match)
	if err := db.ValidateRecordMatch(targetMeta, matchNames); err != nil {
		return nil, err
	}
	match := db.RecordInput{}
	for _, name := range matchNames {
		targetField, ok := db.RecordAddressableFieldByName(targetFields, name)
		if !ok {
			return nil, fmt.Errorf("link match field %q does not exist", name)
		}
		raw, err := resolveValue(ctx, store, targetField, reference.Match[name], depth+1)
		if err != nil {
			return nil, fmt.Errorf("resolve link match field %q: %w", name, err)
		}
		if string(raw) == "null" {
			return nil, fmt.Errorf("link match field %q cannot be null", name)
		}
		match[name] = raw
	}
	record, err := store.FindRecord(ctx, target, match)
	if err != nil {
		return nil, safeWrap(fmt.Sprintf("resolve link target %q", target), err)
	}
	name, err := recordName(record)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(strconv.Quote(name)), nil
}

type linkReference struct {
	Match map[string]Value
}

func decodeLinkReference(node yaml.Node) (linkReference, error) {
	mapping := yamlmeta.ValueMapping(&node)
	if mapping == nil {
		return linkReference{}, fmt.Errorf("link value must be a match mapping")
	}
	var reference linkReference
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		value := mapping.Content[i+1]
		switch key.Value {
		case "match":
			matchMapping := yamlmeta.ValueMapping(value)
			if matchMapping == nil {
				return linkReference{}, fmt.Errorf("match must be a mapping")
			}
			reference.Match = map[string]Value{}
			for j := 0; j < len(matchMapping.Content); j += 2 {
				matchKey := matchMapping.Content[j]
				matchValue := matchMapping.Content[j+1]
				reference.Match[matchKey.Value] = Value{Node: *matchValue, Line: matchValue.Line}
			}
		default:
			return linkReference{}, fmt.Errorf("unknown link field %q", key.Value)
		}
	}
	if len(reference.Match) == 0 {
		return linkReference{}, fmt.Errorf("link match is required")
	}
	return reference, nil
}

func recordID(record db.Record) (int64, error) {
	value, ok := record["id"]
	if !ok {
		return 0, fmt.Errorf("record id is missing")
	}
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case float64:
		if typed == float64(int64(typed)) {
			return int64(typed), nil
		}
	case json.Number:
		return typed.Int64()
	}
	return 0, fmt.Errorf("record id has unsupported type %T", value)
}

func recordName(record db.Record) (string, error) {
	value, ok := record["name"]
	if !ok {
		return "", fmt.Errorf("record name is missing")
	}
	name, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("record name has unsupported type %T", value)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("record name is empty")
	}
	return name, nil
}

func isRecordNotFound(err error) bool {
	var recordErr db.RecordError
	return errors.As(err, &recordErr) && recordErr.Code == db.RecordErrorNotFound
}

func safeWrap(message string, err error) error {
	if err == nil {
		return fmt.Errorf("%s failed", message)
	}
	var recordErr db.RecordError
	if errors.As(err, &recordErr) {
		return fmt.Errorf("%s: %s", message, recordErr.Message)
	}
	return fmt.Errorf("%s: %w", message, err)
}

func decodeRecords(node *yaml.Node) ([]Record, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("records must be a sequence at line %d", node.Line)
	}
	records := make([]Record, 0, len(node.Content))
	for _, item := range node.Content {
		mapping := yamlmeta.ValueMapping(item)
		if mapping == nil {
			return nil, fmt.Errorf("fixture record must be a mapping at line %d", item.Line)
		}
		record := Record{Values: map[string]Value{}, Line: item.Line}
		for i := 0; i < len(mapping.Content); i += 2 {
			key := mapping.Content[i]
			value := mapping.Content[i+1]
			record.Values[key.Value] = Value{Node: *value, Line: value.Line}
		}
		records = append(records, record)
	}
	return records, nil
}

func nodeJSON(node yaml.Node) (json.RawMessage, error) {
	value, err := nodeAny(&node)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func nodeAny(node *yaml.Node) (any, error) {
	switch node.Kind {
	case yaml.ScalarNode:
		var value any
		if err := node.Decode(&value); err != nil {
			return nil, err
		}
		return value, nil
	case yaml.SequenceNode:
		values := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := nodeAny(child)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	case yaml.MappingNode:
		values := map[string]any{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("mapping key must be a string at line %d", key.Line)
			}
			value, err := nodeAny(node.Content[i+1])
			if err != nil {
				return nil, err
			}
			values[key.Value] = value
		}
		return values, nil
	default:
		return nil, fmt.Errorf("unsupported YAML node at line %d", node.Line)
	}
}

func isNullNode(node yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Tag == "!!null"
}

func sortedValueKeys(values map[string]Value) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

package fixtures

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/recordquery"
	"github.com/hapyco/dygo/internal/shape"
	"gopkg.in/yaml.v3"
)

// ExportPlan previews fixture files generated from live Records.
type ExportPlan struct {
	Files           []ExportFile
	UnresolvedLinks []ExportLink
}

// FileCount returns the number of fixture files in the export plan.
func (p ExportPlan) FileCount() int {
	return len(p.Files)
}

// RecordCount returns the number of Records in the export plan.
func (p ExportPlan) RecordCount() int {
	count := 0
	for _, file := range p.Files {
		count += len(file.Records)
	}
	return count
}

// ExportFile is one fixture file that will be written.
type ExportFile struct {
	AppName     string
	Entity      string
	Path        string
	ProjectPath string
	Records     []db.Record
	Content     []byte
}

// ExportLink reports a link dependency that is not included in the export plan.
type ExportLink struct {
	SourceApp    string
	SourceEntity string
	SourceRecord string
	Field        string
	TargetApp    string
	TargetEntity string
	TargetRecord string
	Reason       string
}

// ExportResult reports fixture export writes.
type ExportResult struct {
	FilesWritten   int
	RecordsWritten int
}

// ExportStore is the metadata and Record behavior needed by fixture export.
type ExportStore interface {
	GetEntityMetaByIdentity(context.Context, string, string) (db.MetadataEntityMeta, error)
	ListRecordsByIdentity(context.Context, string, string, db.RecordListParams) (db.RecordListResult, error)
}

type runtimeExportStore struct {
	metadata db.MetadataReader
	records  db.RecordStore
}

type exportIdentity struct {
	app    string
	entity string
}

type exportQueueItem struct {
	identity exportIdentity
	all      bool
	name     string
}

type exportPlanner struct {
	ctx          context.Context
	store        ExportStore
	entities     map[exportIdentity]catalog.LoadedEntity
	metas        map[exportIdentity]db.MetadataEntityMeta
	records      map[exportIdentity]map[string]db.Record
	links        []ExportLink
	queue        []exportQueueItem
	exportAll    map[exportIdentity]bool
	processedAll map[exportIdentity]bool
	processedOne map[exportIdentity]map[string]bool
	includeLinks bool
}

// ExportPlan builds a fixture export plan from live Records without writing files.
func (r Runner) ExportPlan(ctx context.Context, root string, databaseURL string, target shape.AppRef, includeLinks bool) (ExportPlan, error) {
	metadata, err := project.LoadMetadata(root)
	if err != nil {
		return ExportPlan{}, err
	}

	pool, err := db.OpenRuntimePool(ctx, databaseURL)
	if err != nil {
		return ExportPlan{}, fmt.Errorf("open fixture export database: %w", err)
	}
	defer pool.Close()

	store := runtimeExportStore{
		metadata: db.NewMetadataReader(pool),
		records:  db.NewRecordStoreWithHookPolicy(pool, db.RecordMutationHooksNone),
	}
	return PlanExport(ctx, store, metadata, target, includeLinks)
}

// PlanExport builds a fixture export plan from an injected store.
func PlanExport(ctx context.Context, store ExportStore, metadata project.Metadata, target shape.AppRef, includeLinks bool) (ExportPlan, error) {
	if store == nil {
		return ExportPlan{}, fmt.Errorf("fixture export store is required")
	}
	if err := ctx.Err(); err != nil {
		return ExportPlan{}, fmt.Errorf("plan fixture export: %w", err)
	}
	planner := exportPlanner{
		ctx:          ctx,
		store:        store,
		entities:     exportEntitiesByIdentity(metadata.Entities),
		metas:        map[exportIdentity]db.MetadataEntityMeta{},
		records:      map[exportIdentity]map[string]db.Record{},
		exportAll:    map[exportIdentity]bool{},
		processedAll: map[exportIdentity]bool{},
		processedOne: map[exportIdentity]map[string]bool{},
		includeLinks: includeLinks,
	}
	identity := exportIdentity{app: target.App, entity: target.Name}
	loaded, ok := planner.entities[identity]
	if !ok {
		return ExportPlan{}, fmt.Errorf("Entity target %s/%s is not loaded", target.App, target.Name)
	}
	if loaded.IsCollection() {
		return ExportPlan{}, fmt.Errorf("cannot export fixtures for collection Entity %s/%s; export the parent Entity fixtures instead", target.App, target.Name)
	}

	planner.enqueueAll(identity)
	if err := planner.run(); err != nil {
		return ExportPlan{}, err
	}
	return planner.plan(identity)
}

// WriteExportPlan writes the fixture files described by plan.
func WriteExportPlan(plan ExportPlan) (ExportResult, error) {
	result := ExportResult{}
	for _, file := range plan.Files {
		if len(file.Records) == 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return ExportResult{}, fmt.Errorf("create fixture directory %s: %w", filepath.Dir(file.Path), err)
		}
		if err := os.WriteFile(file.Path, file.Content, 0o644); err != nil {
			return ExportResult{}, fmt.Errorf("write fixture file %s: %w", file.Path, err)
		}
		result.FilesWritten++
		result.RecordsWritten += len(file.Records)
	}
	return result, nil
}

func (s runtimeExportStore) GetEntityMetaByIdentity(ctx context.Context, appName string, entity string) (db.MetadataEntityMeta, error) {
	return s.metadata.GetEntityMetaByIdentity(ctx, appName, entity)
}

func (s runtimeExportStore) ListRecordsByIdentity(ctx context.Context, appName string, entity string, params db.RecordListParams) (db.RecordListResult, error) {
	return s.records.ListRecordsByIdentity(ctx, appName, entity, params)
}

func exportEntitiesByIdentity(entities []catalog.LoadedEntity) map[exportIdentity]catalog.LoadedEntity {
	index := map[exportIdentity]catalog.LoadedEntity{}
	for _, entity := range entities {
		index[exportIdentity{app: entity.AppName, entity: entity.Entity.Name}] = entity
	}
	return index
}

func (p *exportPlanner) enqueueAll(identity exportIdentity) {
	if p.exportAll[identity] {
		return
	}
	p.exportAll[identity] = true
	p.queue = append(p.queue, exportQueueItem{identity: identity, all: true})
}

func (p *exportPlanner) enqueueOne(identity exportIdentity, name string) {
	if p.exportAll[identity] {
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	if p.processedOne[identity] != nil && p.processedOne[identity][name] {
		return
	}
	p.queue = append(p.queue, exportQueueItem{identity: identity, name: name})
}

func (p *exportPlanner) run() error {
	for len(p.queue) > 0 {
		item := p.queue[0]
		p.queue = p.queue[1:]
		if item.all {
			if p.processedAll[item.identity] {
				continue
			}
			records, err := p.listAll(item.identity)
			if err != nil {
				return err
			}
			p.processedAll[item.identity] = true
			if err := p.addRecords(item.identity, records); err != nil {
				return err
			}
			continue
		}
		if p.exportAll[item.identity] {
			continue
		}
		if p.processedOne[item.identity] != nil && p.processedOne[item.identity][item.name] {
			continue
		}
		record, ok, err := p.findByName(item.identity, item.name)
		if err != nil {
			return err
		}
		if p.processedOne[item.identity] == nil {
			p.processedOne[item.identity] = map[string]bool{}
		}
		p.processedOne[item.identity][item.name] = true
		if !ok {
			continue
		}
		if err := p.addRecords(item.identity, []db.Record{record}); err != nil {
			return err
		}
	}
	return nil
}

func (p *exportPlanner) listAll(identity exportIdentity) ([]db.Record, error) {
	offset := 0
	records := []db.Record{}
	for {
		result, err := p.store.ListRecordsByIdentity(p.ctx, identity.app, identity.entity, db.RecordListParams{
			Limit:  recordquery.MaxLimit,
			Offset: offset,
			Sort:   []db.RecordSort{{Field: "id"}},
		})
		if err != nil {
			return nil, safeWrap(fmt.Sprintf("list records for %s/%s", identity.app, identity.entity), err)
		}
		records = append(records, result.Records...)
		if result.Count == 0 || offset+result.Count >= result.Total {
			return records, nil
		}
		offset += result.Count
	}
}

func (p *exportPlanner) findByName(identity exportIdentity, name string) (db.Record, bool, error) {
	result, err := p.store.ListRecordsByIdentity(p.ctx, identity.app, identity.entity, db.RecordListParams{
		Limit:   1,
		Filters: []db.RecordFilter{{Field: "name", Value: name}},
	})
	if err != nil {
		return nil, false, safeWrap(fmt.Sprintf("find record %q for %s/%s", name, identity.app, identity.entity), err)
	}
	if len(result.Records) == 0 {
		return nil, false, nil
	}
	return result.Records[0], true, nil
}

func (p *exportPlanner) addRecords(identity exportIdentity, records []db.Record) error {
	if len(records) == 0 {
		return nil
	}
	if _, err := p.meta(identity); err != nil {
		return err
	}
	if p.records[identity] == nil {
		p.records[identity] = map[string]db.Record{}
	}
	for _, record := range records {
		name, err := exportRecordName(record)
		if err != nil {
			return fmt.Errorf("export %s/%s record: %w", identity.app, identity.entity, err)
		}
		if _, exists := p.records[identity][name]; exists {
			continue
		}
		p.records[identity][name] = record
		if err := p.collectLinks(identity, record); err != nil {
			return err
		}
	}
	return nil
}

func (p *exportPlanner) meta(identity exportIdentity) (db.MetadataEntityMeta, error) {
	if meta, ok := p.metas[identity]; ok {
		return meta, nil
	}
	meta, err := p.store.GetEntityMetaByIdentity(p.ctx, identity.app, identity.entity)
	if err != nil {
		return db.MetadataEntityMeta{}, safeWrap(fmt.Sprintf("load metadata for %s/%s", identity.app, identity.entity), err)
	}
	if meta.IsCollection {
		return db.MetadataEntityMeta{}, fmt.Errorf("cannot export fixtures for collection Entity %s/%s; export the parent Entity fixtures instead", identity.app, identity.entity)
	}
	p.metas[identity] = meta
	return meta, nil
}

func (p *exportPlanner) collectLinks(identity exportIdentity, record db.Record) error {
	meta, err := p.meta(identity)
	if err != nil {
		return err
	}
	sourceName, err := exportRecordName(record)
	if err != nil {
		return err
	}
	for _, field := range meta.Fields {
		if field.Type != "link" {
			continue
		}
		value, ok := record[field.Name]
		if !ok || value == nil {
			continue
		}
		targetName, ok := value.(string)
		if !ok {
			return fmt.Errorf("export %s/%s record %q link field %q has unsupported value %T", identity.app, identity.entity, sourceName, field.Name, value)
		}
		targetName = strings.TrimSpace(targetName)
		if targetName == "" {
			continue
		}
		target, targetErr := p.resolveLinkTarget(identity, field)
		link := ExportLink{
			SourceApp:    identity.app,
			SourceEntity: identity.entity,
			SourceRecord: sourceName,
			Field:        field.Name,
			TargetRecord: targetName,
		}
		if targetErr != nil {
			link.Reason = targetErr.Error()
			p.links = append(p.links, link)
			continue
		}
		link.TargetApp = target.app
		link.TargetEntity = target.entity
		p.links = append(p.links, link)
		if p.includeLinks {
			p.enqueueOne(target, targetName)
		}
	}
	return nil
}

func (p *exportPlanner) resolveLinkTarget(owner exportIdentity, field db.MetadataField) (exportIdentity, error) {
	var options struct {
		App    string `json:"app"`
		Entity string `json:"entity"`
	}
	if err := json.Unmarshal(field.Options, &options); err != nil {
		return exportIdentity{}, fmt.Errorf("link field options are invalid")
	}
	if strings.TrimSpace(options.Entity) == "" {
		return exportIdentity{}, fmt.Errorf("link field target entity is required")
	}
	if strings.TrimSpace(options.App) != "" {
		target := exportIdentity{app: options.App, entity: options.Entity}
		loaded, ok := p.entities[target]
		if !ok {
			return exportIdentity{}, fmt.Errorf("target Entity %s/%s is not loaded", target.app, target.entity)
		}
		if loaded.IsCollection() {
			return exportIdentity{}, fmt.Errorf("target Entity %s/%s is a collection", target.app, target.entity)
		}
		return target, nil
	}
	sameApp := exportIdentity{app: owner.app, entity: options.Entity}
	if loaded, ok := p.entities[sameApp]; ok {
		if loaded.IsCollection() {
			return exportIdentity{}, fmt.Errorf("target Entity %s/%s is a collection", sameApp.app, sameApp.entity)
		}
		return sameApp, nil
	}
	var matches []exportIdentity
	for identity, loaded := range p.entities {
		if identity.entity == options.Entity && !loaded.IsCollection() {
			matches = append(matches, identity)
		}
	}
	switch len(matches) {
	case 0:
		return exportIdentity{}, fmt.Errorf("target Entity %q is not loaded", options.Entity)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, match := range matches {
			names = append(names, match.app)
		}
		sort.Strings(names)
		return exportIdentity{}, fmt.Errorf("target Entity %q is ambiguous in apps %s; set options.app", options.Entity, strings.Join(names, ", "))
	}
}

func (p *exportPlanner) plan(target exportIdentity) (ExportPlan, error) {
	included := p.includedRecordNames()
	unresolved := make([]ExportLink, 0, len(p.links))
	for _, link := range p.links {
		if link.Reason != "" {
			unresolved = append(unresolved, link)
			continue
		}
		targetIdentity := exportIdentity{app: link.TargetApp, entity: link.TargetEntity}
		if included[targetIdentity] != nil && included[targetIdentity][link.TargetRecord] {
			continue
		}
		link.Reason = "target record is not included in this export"
		unresolved = append(unresolved, link)
	}
	sortExportLinks(unresolved)

	identities := make([]exportIdentity, 0, len(p.records))
	for identity, records := range p.records {
		if len(records) == 0 {
			continue
		}
		identities = append(identities, identity)
	}
	sort.SliceStable(identities, func(i, j int) bool {
		if identities[i] == target {
			return true
		}
		if identities[j] == target {
			return false
		}
		if identities[i].app == identities[j].app {
			return identities[i].entity < identities[j].entity
		}
		return identities[i].app < identities[j].app
	})

	files := make([]ExportFile, 0, len(identities))
	for _, identity := range identities {
		loaded := p.entities[identity]
		meta := p.metas[identity]
		records := sortedExportRecords(p.records[identity])
		content, err := encodeExportFixture(meta, records)
		if err != nil {
			return ExportPlan{}, fmt.Errorf("encode fixture %s/%s: %w", identity.app, identity.entity, err)
		}
		path := filepath.Join(filepath.Dir(loaded.Path), shape.EntityFixturesFile)
		projectPath := filepath.ToSlash(filepath.Join(shape.AppDir(identity.app), shape.EntityFixturesPath(identity.entity)))
		files = append(files, ExportFile{
			AppName:     identity.app,
			Entity:      identity.entity,
			Path:        path,
			ProjectPath: projectPath,
			Records:     records,
			Content:     content,
		})
	}

	return ExportPlan{Files: files, UnresolvedLinks: unresolved}, nil
}

func (p *exportPlanner) includedRecordNames() map[exportIdentity]map[string]bool {
	included := map[exportIdentity]map[string]bool{}
	for identity, records := range p.records {
		included[identity] = map[string]bool{}
		for name := range records {
			included[identity][name] = true
		}
	}
	return included
}

func sortedExportRecords(records map[string]db.Record) []db.Record {
	names := make([]string, 0, len(records))
	for name := range records {
		names = append(names, name)
	}
	sort.Strings(names)
	sorted := make([]db.Record, 0, len(names))
	for _, name := range names {
		sorted = append(sorted, records[name])
	}
	return sorted
}

func sortExportLinks(links []ExportLink) {
	sort.SliceStable(links, func(i, j int) bool {
		left := []string{links[i].SourceApp, links[i].SourceEntity, links[i].SourceRecord, links[i].Field, links[i].TargetApp, links[i].TargetEntity, links[i].TargetRecord, links[i].Reason}
		right := []string{links[j].SourceApp, links[j].SourceEntity, links[j].SourceRecord, links[j].Field, links[j].TargetApp, links[j].TargetEntity, links[j].TargetRecord, links[j].Reason}
		return strings.Join(left, "\x00") < strings.Join(right, "\x00")
	})
}

func encodeExportFixture(meta db.MetadataEntityMeta, records []db.Record) ([]byte, error) {
	if len(records) == 0 {
		return nil, nil
	}
	document := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			yamlStringNode("entity"), yamlStringNode(meta.Key),
			yamlStringNode("match"), yamlSequenceNode([]*yaml.Node{yamlStringNode("name")}),
			yamlStringNode("records"), yamlRecordsNode(meta, records),
		},
	}
	var buf bytes.Buffer
	buf.WriteString("# Exported by dygo fixture export; review before committing.\n")
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func yamlRecordsNode(meta db.MetadataEntityMeta, records []db.Record) *yaml.Node {
	items := make([]*yaml.Node, 0, len(records))
	for _, record := range records {
		items = append(items, yamlRecordNode(meta, record))
	}
	return yamlSequenceNode(items)
}

func yamlRecordNode(meta db.MetadataEntityMeta, record db.Record) *yaml.Node {
	content := []*yaml.Node{
		yamlStringNode("name"), yamlValueNode(record["name"]),
	}
	for _, field := range meta.Fields {
		if field.Name == "name" {
			continue
		}
		if !db.MetadataFieldStored(field) || field.WriteOnly {
			continue
		}
		value, ok := record[field.Name]
		if !ok {
			continue
		}
		if field.Type == "link" {
			// TODO(fixtures): support exporting null link values once fixture apply can write link nulls.
			if value == nil {
				continue
			}
			content = append(content, yamlStringNode(field.Name), yamlLinkValueNode(value))
			continue
		}
		content = append(content, yamlStringNode(field.Name), yamlValueNode(value))
	}
	return &yaml.Node{Kind: yaml.MappingNode, Content: content}
}

func yamlLinkValueNode(value any) *yaml.Node {
	name, _ := value.(string)
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			yamlStringNode("match"),
			&yaml.Node{
				Kind: yaml.MappingNode,
				Content: []*yaml.Node{
					yamlStringNode("name"), yamlStringNode(name),
				},
			},
		},
	}
}

func yamlSequenceNode(items []*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Content: items}
}

func yamlStringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func yamlValueNode(value any) *yaml.Node {
	switch typed := value.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: ""}
	case string:
		return yamlStringNode(typed)
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(typed)}
	case int:
		return yamlIntNode(int64(typed))
	case int8:
		return yamlIntNode(int64(typed))
	case int16:
		return yamlIntNode(int64(typed))
	case int32:
		return yamlIntNode(int64(typed))
	case int64:
		return yamlIntNode(typed)
	case uint:
		return yamlUintNode(uint64(typed))
	case uint8:
		return yamlUintNode(uint64(typed))
	case uint16:
		return yamlUintNode(uint64(typed))
	case uint32:
		return yamlUintNode(uint64(typed))
	case uint64:
		return yamlUintNode(typed)
	case float32:
		return yamlFloatNode(float64(typed))
	case float64:
		return yamlFloatNode(typed)
	case json.Number:
		return yamlStringNode(typed.String())
	case []any:
		items := make([]*yaml.Node, 0, len(typed))
		for _, item := range typed {
			items = append(items, yamlValueNode(item))
		}
		return yamlSequenceNode(items)
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		content := make([]*yaml.Node, 0, len(keys)*2)
		for _, key := range keys {
			content = append(content, yamlStringNode(key), yamlValueNode(typed[key]))
		}
		return &yaml.Node{Kind: yaml.MappingNode, Content: content}
	default:
		var node yaml.Node
		if err := node.Encode(typed); err == nil {
			return &node
		}
		return yamlStringNode(fmt.Sprint(typed))
	}
}

func yamlIntNode(value int64) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(value, 10)}
}

func yamlUintNode(value uint64) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatUint(value, 10)}
}

func yamlFloatNode(value float64) *yaml.Node {
	if math.Trunc(value) == value {
		return yamlIntNode(int64(value))
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: strconv.FormatFloat(value, 'f', -1, 64)}
}

func exportRecordName(record db.Record) (string, error) {
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

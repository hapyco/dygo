package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/jackc/pgx/v5"
)

// MetadataQueryer is the database behavior needed by the metadata reader.
type MetadataQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// MetadataReader reads persisted Core metadata records from PostgreSQL.
type MetadataReader struct {
	queryer MetadataQueryer
}

// MetadataApp is one installed App record exposed through runtime metadata APIs.
type MetadataApp struct {
	Name    string `json:"name"`
	Label   string `json:"label"`
	Version string `json:"version"`
	Status  string `json:"status"`
}

// MetadataAppRef is a compact App reference embedded in metadata responses.
type MetadataAppRef struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

// MetadataEntity is one Entity metadata summary.
type MetadataEntity struct {
	ID           int64           `json:"-"`
	Name         string          `json:"name"`
	Key          string          `json:"key"`
	Slug         *string         `json:"slug"`
	Label        string          `json:"label"`
	Description  string          `json:"description"`
	Icon         string          `json:"icon,omitempty"`
	IsSingle     bool            `json:"is-single"`
	IsSystem     bool            `json:"is-system"`
	IsCollection bool            `json:"is-collection"`
	Naming       json.RawMessage `json:"naming,omitempty"`
	App          MetadataAppRef  `json:"app"`
}

// RouteSlug returns the public route slug for routeable Entities.
func (e MetadataEntity) RouteSlug() string {
	if e.Slug == nil {
		return ""
	}
	return *e.Slug
}

// MetadataEntityMeta is the complete persisted metadata for one Entity.
type MetadataEntityMeta struct {
	MetadataEntity
	Fields       []MetadataField               `json:"fields"`
	SystemFields []MetadataField               `json:"system-fields"`
	Indexes      []MetadataIndex               `json:"indexes"`
	Constraints  []MetadataConstraint          `json:"constraints"`
	Collections  map[string]MetadataEntityMeta `json:"collections,omitempty"`
}

// MetadataField is one persisted Field definition.
type MetadataField struct {
	ID             int64               `json:"-"`
	Name           string              `json:"name"`
	Label          string              `json:"label"`
	Type           string              `json:"type"`
	Required       bool                `json:"required"`
	Unique         bool                `json:"unique"`
	Index          bool                `json:"index"`
	Stored         bool                `json:"stored"`
	WriteOnly      bool                `json:"write-only"`
	Listable       bool                `json:"listable"`
	NameRenderable bool                `json:"name-renderable"`
	ValueKind      string              `json:"value-kind"`
	Studio         MetadataFieldStudio `json:"studio"`
	Default        json.RawMessage     `json:"default,omitempty"`
	Check          json.RawMessage     `json:"check,omitempty"`
	Position       int                 `json:"position"`
	Options        json.RawMessage     `json:"options,omitempty"`
}

// MetadataFieldStudio describes Studio rendering hints derived from field type behavior.
type MetadataFieldStudio struct {
	Editor  string `json:"editor"`
	Display string `json:"display"`
}

// MetadataIndex is one persisted top-level Entity index definition.
type MetadataIndex struct {
	Name     string          `json:"name"`
	Fields   json.RawMessage `json:"fields"`
	Position int             `json:"position"`
}

// MetadataConstraint is one persisted top-level Entity constraint definition.
type MetadataConstraint struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Fields   json.RawMessage `json:"fields,omitempty"`
	Field    string          `json:"field,omitempty"`
	Operator string          `json:"operator,omitempty"`
	Value    json.RawMessage `json:"value,omitempty"`
	Position int             `json:"position"`
}

// MetadataNotFoundError reports a missing persisted metadata resource.
type MetadataNotFoundError struct {
	Kind string
	Name string
}

func (e MetadataNotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Kind, e.Name)
}

// IsMetadataNotFound reports whether err is a MetadataNotFoundError.
func IsMetadataNotFound(err error) bool {
	var notFound MetadataNotFoundError
	return errors.As(err, &notFound)
}

// NewMetadataReader returns a metadata reader backed by queryer.
func NewMetadataReader(queryer MetadataQueryer) MetadataReader {
	return MetadataReader{queryer: queryer}
}

// ListApps returns all persisted Apps ordered by name.
func (r MetadataReader) ListApps(ctx context.Context) ([]MetadataApp, error) {
	if err := r.requireQueryer(); err != nil {
		return nil, err
	}
	rows, err := r.queryer.Query(ctx, `
SELECT name, label, version, status
FROM "app"
ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query metadata apps: %w", err)
	}
	defer rows.Close()

	apps := []MetadataApp{}
	for rows.Next() {
		var app MetadataApp
		if err := rows.Scan(&app.Name, &app.Label, &app.Version, &app.Status); err != nil {
			return nil, fmt.Errorf("scan metadata app: %w", err)
		}
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read metadata apps: %w", err)
	}
	return apps, nil
}

// GetApp returns one persisted App by name.
func (r MetadataReader) GetApp(ctx context.Context, name string) (MetadataApp, error) {
	if err := r.requireQueryer(); err != nil {
		return MetadataApp{}, err
	}
	var app MetadataApp
	err := r.queryer.QueryRow(ctx, `
SELECT name, label, version, status
FROM "app"
WHERE name = $1`, name).Scan(&app.Name, &app.Label, &app.Version, &app.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return MetadataApp{}, MetadataNotFoundError{Kind: "app", Name: name}
	}
	if err != nil {
		return MetadataApp{}, fmt.Errorf("query metadata app %q: %w", name, err)
	}
	return app, nil
}

// ListEntities returns all persisted Entities ordered by app and Entity key.
func (r MetadataReader) ListEntities(ctx context.Context) ([]MetadataEntity, error) {
	if err := r.requireQueryer(); err != nil {
		return nil, err
	}
	rows, err := r.queryer.Query(ctx, `
SELECT e.name, e.key, COALESCE(e.slug, ''), e.label, COALESCE(e.description, ''), COALESCE(e.icon, ''), COALESCE(e.is_single, false), COALESCE(e.is_system, false), COALESCE(e.is_collection, false), e.naming, a.name, a.label
FROM "entity" e
JOIN "app" a ON a.id = e.app_id
ORDER BY a.name, e.key`)
	if err != nil {
		return nil, fmt.Errorf("query metadata entities: %w", err)
	}
	defer rows.Close()

	entities := []MetadataEntity{}
	for rows.Next() {
		var entity MetadataEntity
		var naming []byte
		var slug string
		if err := rows.Scan(&entity.Name, &entity.Key, &slug, &entity.Label, &entity.Description, &entity.Icon, &entity.IsSingle, &entity.IsSystem, &entity.IsCollection, &naming, &entity.App.Name, &entity.App.Label); err != nil {
			return nil, fmt.Errorf("scan metadata entity: %w", err)
		}
		entity.Slug = stringPointerOrNil(slug)
		entity.Naming = rawJSONOrNil(naming)
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read metadata entities: %w", err)
	}
	return entities, nil
}

// GetEntityMeta returns complete persisted metadata for one Entity slug.
func (r MetadataReader) GetEntityMeta(ctx context.Context, slug string) (MetadataEntityMeta, error) {
	return r.getEntityMeta(ctx, slug, `
SELECT e.id, e.name, e.key, COALESCE(e.slug, ''), e.label, COALESCE(e.description, ''), COALESCE(e.icon, ''), COALESCE(e.is_single, false), COALESCE(e.is_system, false), COALESCE(e.is_collection, false), e.naming, a.name, a.label
FROM "entity" e
JOIN "app" a ON a.id = e.app_id
WHERE e.slug = $1`, slug)
}

// GetEntityMetaByIdentity returns complete persisted metadata for one app-scoped Entity identity.
func (r MetadataReader) GetEntityMetaByIdentity(ctx context.Context, appName string, entity string) (MetadataEntityMeta, error) {
	return r.getEntityMeta(ctx, appName+"/"+entity, `
SELECT e.id, e.name, e.key, COALESCE(e.slug, ''), e.label, COALESCE(e.description, ''), COALESCE(e.icon, ''), COALESCE(e.is_single, false), COALESCE(e.is_system, false), COALESCE(e.is_collection, false), e.naming, a.name, a.label
FROM "entity" e
JOIN "app" a ON a.id = e.app_id
WHERE a.name = $1 AND e.key = $2`, appName, entity)
}

func (r MetadataReader) getEntityMeta(ctx context.Context, name string, sql string, args ...any) (MetadataEntityMeta, error) {
	if err := r.requireQueryer(); err != nil {
		return MetadataEntityMeta{}, err
	}

	var meta MetadataEntityMeta
	var naming []byte
	var slug string
	err := r.queryer.QueryRow(ctx, sql, args...).Scan(&meta.ID, &meta.Name, &meta.Key, &slug, &meta.Label, &meta.Description, &meta.Icon, &meta.IsSingle, &meta.IsSystem, &meta.IsCollection, &naming, &meta.App.Name, &meta.App.Label)
	if errors.Is(err, pgx.ErrNoRows) {
		return MetadataEntityMeta{}, MetadataNotFoundError{Kind: "entity", Name: name}
	}
	if err != nil {
		return MetadataEntityMeta{}, fmt.Errorf("query metadata entity %q: %w", name, err)
	}
	meta.Slug = stringPointerOrNil(slug)
	meta.Naming = rawJSONOrNil(naming)

	fields, err := r.entityFields(ctx, meta.ID)
	if err != nil {
		return MetadataEntityMeta{}, err
	}
	indexes, err := r.entityIndexes(ctx, meta.ID)
	if err != nil {
		return MetadataEntityMeta{}, err
	}
	constraints, err := r.entityConstraints(ctx, meta.ID)
	if err != nil {
		return MetadataEntityMeta{}, err
	}
	meta.Fields = fields
	meta.SystemFields = metadataSystemFields()
	meta.Indexes = indexes
	meta.Constraints = constraints
	collections, err := r.entityCollections(ctx, meta, fields)
	if err != nil {
		return MetadataEntityMeta{}, err
	}
	meta.Collections = collections
	return meta, nil
}

func (r MetadataReader) entityCollections(ctx context.Context, parent MetadataEntityMeta, fields []MetadataField) (map[string]MetadataEntityMeta, error) {
	collections := map[string]MetadataEntityMeta{}
	for _, field := range fields {
		if field.Type != "collection" {
			continue
		}
		var options struct {
			App    string `json:"app"`
			Entity string `json:"entity"`
		}
		if err := json.Unmarshal(field.Options, &options); err != nil {
			return nil, fmt.Errorf("collection field %s.%s options are invalid: %w", parent.Key, field.Name, err)
		}
		appName := options.App
		if appName == "" {
			appName = parent.App.Name
		}
		if options.Entity == "" {
			return nil, fmt.Errorf("collection field %s.%s target entity is required", parent.Key, field.Name)
		}
		child, err := r.GetEntityMetaByIdentity(ctx, appName, options.Entity)
		if err != nil {
			return nil, fmt.Errorf("load collection metadata %s/%s for %s.%s: %w", appName, options.Entity, parent.Key, field.Name, err)
		}
		collections[field.Name] = child
	}
	if len(collections) == 0 {
		return nil, nil
	}
	return collections, nil
}

func metadataSystemFields() []MetadataField {
	fields := make([]MetadataField, 0, len(systemRecordFields))
	for index, systemField := range systemRecordFields {
		fields = append(fields, MetadataField{
			Name:           systemField.Name,
			Label:          systemField.Label,
			Type:           systemField.Type,
			Required:       true,
			Stored:         true,
			WriteOnly:      false,
			Listable:       systemField.Listable,
			NameRenderable: systemField.Nameable,
			ValueKind:      systemField.ValueKind,
			Studio: MetadataFieldStudio{
				Editor:  systemField.StudioEditor,
				Display: systemField.StudioDisplay,
			},
			Position: index + 1,
		})
	}
	return fields
}

func (r MetadataReader) entityFields(ctx context.Context, entityID int64) ([]MetadataField, error) {
	rows, err := r.queryer.Query(ctx, `
	SELECT id, field_name, label, type, required, "unique", "index", "default", "check", position, options
	FROM "field"
	WHERE entity_id = $1
	ORDER BY position, name`, entityID)
	if err != nil {
		return nil, fmt.Errorf("query metadata fields: %w", err)
	}
	defer rows.Close()

	fields := []MetadataField{}
	for rows.Next() {
		var field MetadataField
		var defaultValue []byte
		var check []byte
		var options []byte
		if err := rows.Scan(&field.ID, &field.Name, &field.Label, &field.Type, &field.Required, &field.Unique, &field.Index, &defaultValue, &check, &field.Position, &options); err != nil {
			return nil, fmt.Errorf("scan metadata field: %w", err)
		}
		field.Default = rawJSONOrNil(defaultValue)
		field.Check = rawJSONOrNil(check)
		field.Options = rawJSONOrNil(options)
		field.applyTypeBehavior()
		fields = append(fields, field)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read metadata fields: %w", err)
	}
	return fields, nil
}

func (f *MetadataField) applyTypeBehavior() {
	definition, ok := fieldtype.DefaultDefinition(f.Type)
	if !ok {
		return
	}
	f.Stored = definition.Behavior.Stored
	f.WriteOnly = definition.Behavior.WriteOnly
	f.Listable = definition.Behavior.Listable
	f.NameRenderable = definition.Behavior.NameRenderable
	f.ValueKind = definition.Behavior.ValueKind
	f.Studio = MetadataFieldStudio{
		Editor:  definition.Behavior.StudioEditor,
		Display: definition.Behavior.StudioDisplay,
	}
}

func (r MetadataReader) entityIndexes(ctx context.Context, entityID int64) ([]MetadataIndex, error) {
	rows, err := r.queryer.Query(ctx, `
SELECT index_name, field_names, position
FROM "index"
WHERE entity_id = $1
ORDER BY position, name`, entityID)
	if err != nil {
		return nil, fmt.Errorf("query metadata indexes: %w", err)
	}
	defer rows.Close()

	indexes := []MetadataIndex{}
	for rows.Next() {
		var index MetadataIndex
		var fields []byte
		if err := rows.Scan(&index.Name, &fields, &index.Position); err != nil {
			return nil, fmt.Errorf("scan metadata index: %w", err)
		}
		index.Fields = rawJSONOrNil(fields)
		indexes = append(indexes, index)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read metadata indexes: %w", err)
	}
	return indexes, nil
}

func (r MetadataReader) entityConstraints(ctx context.Context, entityID int64) ([]MetadataConstraint, error) {
	rows, err := r.queryer.Query(ctx, `
SELECT constraint_name, type, field_names, COALESCE(field, ''), COALESCE(operator, ''), value, position
FROM "constraint"
WHERE entity_id = $1
ORDER BY position, name`, entityID)
	if err != nil {
		return nil, fmt.Errorf("query metadata constraints: %w", err)
	}
	defer rows.Close()

	constraints := []MetadataConstraint{}
	for rows.Next() {
		var constraint MetadataConstraint
		var fields []byte
		var value []byte
		if err := rows.Scan(&constraint.Name, &constraint.Type, &fields, &constraint.Field, &constraint.Operator, &value, &constraint.Position); err != nil {
			return nil, fmt.Errorf("scan metadata constraint: %w", err)
		}
		constraint.Fields = rawJSONOrNil(fields)
		constraint.Value = rawJSONOrNil(value)
		constraints = append(constraints, constraint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read metadata constraints: %w", err)
	}
	return constraints, nil
}

func (r MetadataReader) requireQueryer() error {
	if r.queryer == nil {
		return fmt.Errorf("metadata queryer is required")
	}
	return nil
}

func rawJSONOrNil(value []byte) json.RawMessage {
	if len(value) == 0 || string(value) == "null" {
		return nil
	}
	copied := append([]byte(nil), value...)
	return json.RawMessage(copied)
}

func stringPointerOrNil(value string) *string {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}

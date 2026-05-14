package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
	ID          int64           `json:"-"`
	Name        string          `json:"name"`
	RouteSlug   string          `json:"route-slug"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	Naming      json.RawMessage `json:"naming,omitempty"`
	App         MetadataAppRef  `json:"app"`
}

// MetadataEntityMeta is the complete persisted metadata for one Entity.
type MetadataEntityMeta struct {
	MetadataEntity
	Fields      []MetadataField      `json:"fields"`
	Indexes     []MetadataIndex      `json:"indexes"`
	Constraints []MetadataConstraint `json:"constraints"`
}

// MetadataField is one persisted Field definition.
type MetadataField struct {
	Name     string          `json:"name"`
	Label    string          `json:"label"`
	Type     string          `json:"type"`
	Required bool            `json:"required"`
	Unique   bool            `json:"unique"`
	Index    bool            `json:"index"`
	Default  json.RawMessage `json:"default,omitempty"`
	Check    json.RawMessage `json:"check,omitempty"`
	Position int             `json:"position"`
	Options  json.RawMessage `json:"options,omitempty"`
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

// ListEntities returns all persisted Entities ordered by app and Entity name.
func (r MetadataReader) ListEntities(ctx context.Context) ([]MetadataEntity, error) {
	if err := r.requireQueryer(); err != nil {
		return nil, err
	}
	rows, err := r.queryer.Query(ctx, `
SELECT e.name, e.route_slug, e.label, COALESCE(e.description, ''), e.naming, a.name, a.label
FROM "entity" e
JOIN "app" a ON a.id = e.app_id
ORDER BY a.name, e.name`)
	if err != nil {
		return nil, fmt.Errorf("query metadata entities: %w", err)
	}
	defer rows.Close()

	entities := []MetadataEntity{}
	for rows.Next() {
		var entity MetadataEntity
		var naming []byte
		if err := rows.Scan(&entity.Name, &entity.RouteSlug, &entity.Label, &entity.Description, &naming, &entity.App.Name, &entity.App.Label); err != nil {
			return nil, fmt.Errorf("scan metadata entity: %w", err)
		}
		entity.Naming = rawJSONOrNil(naming)
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read metadata entities: %w", err)
	}
	return entities, nil
}

// GetEntityMeta returns complete persisted metadata for one Entity route slug.
func (r MetadataReader) GetEntityMeta(ctx context.Context, routeSlug string) (MetadataEntityMeta, error) {
	return r.getEntityMeta(ctx, routeSlug, `
SELECT e.id, e.name, e.route_slug, e.label, COALESCE(e.description, ''), e.naming, a.name, a.label
FROM "entity" e
JOIN "app" a ON a.id = e.app_id
WHERE e.route_slug = $1`, routeSlug)
}

// GetEntityMetaByIdentity returns complete persisted metadata for one app-scoped Entity identity.
func (r MetadataReader) GetEntityMetaByIdentity(ctx context.Context, appName string, entity string) (MetadataEntityMeta, error) {
	return r.getEntityMeta(ctx, appName+"/"+entity, `
SELECT e.id, e.name, e.route_slug, e.label, COALESCE(e.description, ''), e.naming, a.name, a.label
FROM "entity" e
JOIN "app" a ON a.id = e.app_id
WHERE a.name = $1 AND e.name = $2`, appName, entity)
}

func (r MetadataReader) getEntityMeta(ctx context.Context, name string, sql string, args ...any) (MetadataEntityMeta, error) {
	if err := r.requireQueryer(); err != nil {
		return MetadataEntityMeta{}, err
	}

	var meta MetadataEntityMeta
	var naming []byte
	err := r.queryer.QueryRow(ctx, sql, args...).Scan(&meta.ID, &meta.Name, &meta.RouteSlug, &meta.Label, &meta.Description, &naming, &meta.App.Name, &meta.App.Label)
	if errors.Is(err, pgx.ErrNoRows) {
		return MetadataEntityMeta{}, MetadataNotFoundError{Kind: "entity", Name: name}
	}
	if err != nil {
		return MetadataEntityMeta{}, fmt.Errorf("query metadata entity %q: %w", name, err)
	}
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
	meta.Indexes = indexes
	meta.Constraints = constraints
	return meta, nil
}

func (r MetadataReader) entityFields(ctx context.Context, entityID int64) ([]MetadataField, error) {
	rows, err := r.queryer.Query(ctx, `
SELECT field_name, label, type, required, "unique", "index", "default", "check", position, options
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
		if err := rows.Scan(&field.Name, &field.Label, &field.Type, &field.Required, &field.Unique, &field.Index, &defaultValue, &check, &field.Position, &options); err != nil {
			return nil, fmt.Errorf("scan metadata field: %w", err)
		}
		field.Default = rawJSONOrNil(defaultValue)
		field.Check = rawJSONOrNil(check)
		field.Options = rawJSONOrNil(options)
		fields = append(fields, field)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read metadata fields: %w", err)
	}
	return fields, nil
}

func (r MetadataReader) entityIndexes(ctx context.Context, entityID int64) ([]MetadataIndex, error) {
	rows, err := r.queryer.Query(ctx, `
SELECT index_name, fields, position
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
SELECT constraint_name, type, fields, COALESCE(field, ''), COALESCE(operator, ''), value, position
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

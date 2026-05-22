package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/dygo-dev/dygo/internal/entity/schema"
	"github.com/jackc/pgx/v5"
	"gopkg.in/yaml.v3"
)

type metadataPersistResult struct {
	Apps        int
	Entities    int
	Fields      int
	Indexes     int
	Constraints int
}

type metadataRecordSet struct {
	Apps        []appRecord
	Entities    []entityRecord
	Fields      []fieldRecord
	Indexes     []indexRecord
	Constraints []constraintRecord
}

type appRecord struct {
	Name    string
	Label   string
	Version string
	Status  string
}

type entityRecord struct {
	AppName     string
	Name        string
	RouteSlug   string
	Label       string
	Description string
	Icon        string
	IsSingle    bool
	Naming      []byte
}

type fieldRecord struct {
	EntityAppName string
	EntityName    string
	RecordName    string
	Name          string
	Label         string
	Type          string
	Required      bool
	Unique        bool
	Index         bool
	Default       []byte
	Check         []byte
	Position      int
	Options       []byte
}

type indexRecord struct {
	EntityAppName string
	EntityName    string
	RecordName    string
	Name          string
	Fields        []byte
	Position      int
}

type constraintRecord struct {
	EntityAppName string
	EntityName    string
	RecordName    string
	Name          string
	Type          string
	Fields        []byte
	Field         string
	Operator      string
	Value         []byte
	Position      int
}

func persistMetadataRecords(ctx context.Context, tx pgx.Tx, metadata metadataCatalog) (metadataPersistResult, error) {
	records, err := buildMetadataRecords(metadata)
	if err != nil {
		return metadataPersistResult{}, err
	}

	appIDs := map[string]int64{}
	for _, app := range records.Apps {
		var id int64
		if err := tx.QueryRow(ctx, `
INSERT INTO "app" (name, label, version, status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (name) DO UPDATE
SET label = EXCLUDED.label,
	version = EXCLUDED.version,
	status = EXCLUDED.status,
	updated_at = now()
RETURNING id`, app.Name, app.Label, app.Version, app.Status).Scan(&id); err != nil {
			return metadataPersistResult{}, fmt.Errorf("persist app metadata %q: %w", app.Name, err)
		}
		appIDs[app.Name] = id
	}

	entityIDs := map[string]int64{}
	for _, entity := range records.Entities {
		appID, ok := appIDs[entity.AppName]
		if !ok {
			return metadataPersistResult{}, fmt.Errorf("persist entity metadata %q: app %q was not persisted", entity.Name, entity.AppName)
		}
		var id int64
		if err := tx.QueryRow(ctx, `
INSERT INTO "entity" (app_id, name, route_slug, label, description, icon, is_single, naming)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (app_id, name) DO UPDATE
SET app_id = EXCLUDED.app_id,
	name = EXCLUDED.name,
	route_slug = EXCLUDED.route_slug,
	label = EXCLUDED.label,
	description = EXCLUDED.description,
	icon = EXCLUDED.icon,
	is_single = EXCLUDED.is_single,
	naming = EXCLUDED.naming,
	updated_at = now()
RETURNING id`, appID, entity.Name, entity.RouteSlug, entity.Label, entity.Description, entity.Icon, entity.IsSingle, entity.Naming).Scan(&id); err != nil {
			return metadataPersistResult{}, fmt.Errorf("persist entity metadata %s/%s: %w", entity.AppName, entity.Name, err)
		}
		entityIDs[entityKey(entity.AppName, entity.Name)] = id
	}

	for _, field := range records.Fields {
		entityID, ok := entityIDs[entityKey(field.EntityAppName, field.EntityName)]
		if !ok {
			return metadataPersistResult{}, fmt.Errorf("persist field metadata %s/%s.%s: entity was not persisted", field.EntityAppName, field.EntityName, field.Name)
		}
		if err := persistFieldRecord(ctx, tx, entityID, field); err != nil {
			return metadataPersistResult{}, err
		}
	}

	for _, index := range records.Indexes {
		entityID, ok := entityIDs[entityKey(index.EntityAppName, index.EntityName)]
		if !ok {
			return metadataPersistResult{}, fmt.Errorf("persist index metadata %s/%s.%s: entity was not persisted", index.EntityAppName, index.EntityName, index.Name)
		}
		if err := persistIndexRecord(ctx, tx, entityID, index); err != nil {
			return metadataPersistResult{}, err
		}
	}

	for _, constraint := range records.Constraints {
		entityID, ok := entityIDs[entityKey(constraint.EntityAppName, constraint.EntityName)]
		if !ok {
			return metadataPersistResult{}, fmt.Errorf("persist constraint metadata %s/%s.%s: entity was not persisted", constraint.EntityAppName, constraint.EntityName, constraint.Name)
		}
		if err := persistConstraintRecord(ctx, tx, entityID, constraint); err != nil {
			return metadataPersistResult{}, err
		}
	}

	return metadataPersistResult{
		Apps:        len(records.Apps),
		Entities:    len(records.Entities),
		Fields:      len(records.Fields),
		Indexes:     len(records.Indexes),
		Constraints: len(records.Constraints),
	}, nil
}

func persistFieldRecord(ctx context.Context, tx pgx.Tx, entityID int64, field fieldRecord) error {
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM "field" WHERE entity_id = $1 AND field_name = $2`, entityID, field.Name).Scan(&id)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("find field metadata %s/%s.%s: %w", field.EntityAppName, field.EntityName, field.Name, err)
	}
	if err == pgx.ErrNoRows {
		if _, err := tx.Exec(ctx, `
INSERT INTO "field" (name, entity_id, field_name, label, type, required, "unique", "index", "default", "check", position, options)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, field.RecordName, entityID, field.Name, field.Label, field.Type, field.Required, field.Unique, field.Index, field.Default, field.Check, field.Position, field.Options); err != nil {
			return fmt.Errorf("persist field metadata %s/%s.%s: %w", field.EntityAppName, field.EntityName, field.Name, err)
		}
		return nil
	}
	if _, err := tx.Exec(ctx, `
UPDATE "field"
SET name = $2,
	label = $3,
	type = $4,
	required = $5,
	"unique" = $6,
	"index" = $7,
	"default" = $8,
	"check" = $9,
	position = $10,
	options = $11,
	updated_at = now()
WHERE id = $1`, id, field.RecordName, field.Label, field.Type, field.Required, field.Unique, field.Index, field.Default, field.Check, field.Position, field.Options); err != nil {
		return fmt.Errorf("persist field metadata %s/%s.%s: %w", field.EntityAppName, field.EntityName, field.Name, err)
	}
	return nil
}

func persistIndexRecord(ctx context.Context, tx pgx.Tx, entityID int64, index indexRecord) error {
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM "index" WHERE entity_id = $1 AND index_name = $2`, entityID, index.Name).Scan(&id)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("find index metadata %s/%s.%s: %w", index.EntityAppName, index.EntityName, index.Name, err)
	}
	if err == pgx.ErrNoRows {
		if _, err := tx.Exec(ctx, `
INSERT INTO "index" (name, entity_id, index_name, fields, position)
VALUES ($1, $2, $3, $4, $5)`, index.RecordName, entityID, index.Name, index.Fields, index.Position); err != nil {
			return fmt.Errorf("persist index metadata %s/%s.%s: %w", index.EntityAppName, index.EntityName, index.Name, err)
		}
		return nil
	}
	if _, err := tx.Exec(ctx, `
UPDATE "index"
SET name = $2,
	fields = $3,
	position = $4,
	updated_at = now()
WHERE id = $1`, id, index.RecordName, index.Fields, index.Position); err != nil {
		return fmt.Errorf("persist index metadata %s/%s.%s: %w", index.EntityAppName, index.EntityName, index.Name, err)
	}
	return nil
}

func persistConstraintRecord(ctx context.Context, tx pgx.Tx, entityID int64, constraint constraintRecord) error {
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM "constraint" WHERE entity_id = $1 AND constraint_name = $2`, entityID, constraint.Name).Scan(&id)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("find constraint metadata %s/%s.%s: %w", constraint.EntityAppName, constraint.EntityName, constraint.Name, err)
	}
	if err == pgx.ErrNoRows {
		if _, err := tx.Exec(ctx, `
INSERT INTO "constraint" (name, entity_id, constraint_name, type, fields, field, operator, value, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, constraint.RecordName, entityID, constraint.Name, constraint.Type, constraint.Fields, nullIfEmpty(constraint.Field), nullIfEmpty(constraint.Operator), constraint.Value, constraint.Position); err != nil {
			return fmt.Errorf("persist constraint metadata %s/%s.%s: %w", constraint.EntityAppName, constraint.EntityName, constraint.Name, err)
		}
		return nil
	}
	if _, err := tx.Exec(ctx, `
UPDATE "constraint"
SET name = $2,
	type = $3,
	fields = $4,
	field = $5,
	operator = $6,
	value = $7,
	position = $8,
	updated_at = now()
WHERE id = $1`, id, constraint.RecordName, constraint.Type, constraint.Fields, nullIfEmpty(constraint.Field), nullIfEmpty(constraint.Operator), constraint.Value, constraint.Position); err != nil {
		return fmt.Errorf("persist constraint metadata %s/%s.%s: %w", constraint.EntityAppName, constraint.EntityName, constraint.Name, err)
	}
	return nil
}

func buildMetadataRecords(metadata metadataCatalog) (metadataRecordSet, error) {
	records := metadataRecordSet{}
	for _, app := range metadata.Apps {
		records.Apps = append(records.Apps, appRecord{
			Name:    app.Manifest.Name,
			Label:   app.Manifest.Label,
			Version: app.Manifest.Version,
			Status:  "active",
		})
	}
	for _, loaded := range metadata.Entities {
		var namingJSON []byte
		if !loaded.Entity.IsSingle {
			var err error
			namingJSON, err = entityNamingJSON(loaded.Entity.EffectiveNaming())
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build entity metadata %s/%s naming: %w", loaded.AppName, loaded.Entity.Name, err)
			}
		}
		records.Entities = append(records.Entities, entityRecord{
			AppName:     loaded.AppName,
			Name:        loaded.Entity.Name,
			RouteSlug:   loaded.Entity.EffectiveRouteSlug(),
			Label:       loaded.Entity.Label,
			Description: loaded.Entity.Description,
			Icon:        strings.TrimSpace(loaded.Entity.Icon),
			IsSingle:    loaded.Entity.IsSingle,
			Naming:      namingJSON,
		})
		for index, field := range loaded.Entity.Fields {
			defaultJSON, err := fieldDefaultJSON(field.Default)
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build field metadata %s/%s.%s default: %w", loaded.AppName, loaded.Entity.Name, field.Name, err)
			}
			optionsJSON, err := fieldOptionsJSON(field.Options)
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build field metadata %s/%s.%s options: %w", loaded.AppName, loaded.Entity.Name, field.Name, err)
			}
			checkJSON, err := fieldCheckJSON(field.Check)
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build field metadata %s/%s.%s check: %w", loaded.AppName, loaded.Entity.Name, field.Name, err)
			}
			records.Fields = append(records.Fields, fieldRecord{
				EntityAppName: loaded.AppName,
				EntityName:    loaded.Entity.Name,
				RecordName:    loaded.Entity.Name + "." + field.Name,
				Name:          field.Name,
				Label:         field.Label,
				Type:          field.Type,
				Required:      field.Required,
				Unique:        field.Unique,
				Index:         field.Index,
				Default:       defaultJSON,
				Check:         checkJSON,
				Position:      index + 1,
				Options:       optionsJSON,
			})
		}
		for indexPosition, index := range loaded.Entity.Indexes {
			fieldsJSON, err := json.Marshal(index.Fields)
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build index metadata %s/%s.%s fields: %w", loaded.AppName, loaded.Entity.Name, index.EffectiveName(loaded.Entity), err)
			}
			records.Indexes = append(records.Indexes, indexRecord{
				EntityAppName: loaded.AppName,
				EntityName:    loaded.Entity.Name,
				RecordName:    loaded.Entity.Name + "." + index.EffectiveName(loaded.Entity),
				Name:          index.EffectiveName(loaded.Entity),
				Fields:        fieldsJSON,
				Position:      indexPosition + 1,
			})
		}
		for constraintPosition, constraint := range loaded.Entity.Constraints {
			fieldsJSON, err := json.Marshal(constraint.Fields)
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build constraint metadata %s/%s.%s fields: %w", loaded.AppName, loaded.Entity.Name, constraint.EffectiveName(loaded.Entity), err)
			}
			valueJSON, err := constraintValueJSON(constraint.Value)
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build constraint metadata %s/%s.%s value: %w", loaded.AppName, loaded.Entity.Name, constraint.EffectiveName(loaded.Entity), err)
			}
			records.Constraints = append(records.Constraints, constraintRecord{
				EntityAppName: loaded.AppName,
				EntityName:    loaded.Entity.Name,
				RecordName:    loaded.Entity.Name + "." + constraint.EffectiveName(loaded.Entity),
				Name:          constraint.EffectiveName(loaded.Entity),
				Type:          constraint.Type,
				Fields:        fieldsJSON,
				Field:         constraint.Field,
				Operator:      constraint.Operator,
				Value:         valueJSON,
				Position:      constraintPosition + 1,
			})
		}
	}
	return records, nil
}

func fieldOptionsJSON(options fieldtype.Options) ([]byte, error) {
	values := map[string]any{}
	if len(options.Values) > 0 {
		values["values"] = options.Values
	}
	if options.App != "" {
		values["app"] = options.App
	}
	if options.Entity != "" {
		values["entity"] = options.Entity
	}
	if len(values) == 0 {
		return nil, nil
	}
	return json.Marshal(values)
}

func entityNamingJSON(naming schema.Naming) ([]byte, error) {
	values := map[string]any{
		"strategy": naming.Strategy,
	}
	switch naming.Strategy {
	case schema.NamingStrategyRandom:
		values["length"] = naming.Length
	case schema.NamingStrategyField:
		values["field"] = naming.Field
	case schema.NamingStrategySeries:
		values["pattern"] = naming.Pattern
	}
	return json.Marshal(values)
}

func fieldDefaultJSON(node yaml.Node) ([]byte, error) {
	if node.Kind == 0 {
		return nil, nil
	}
	return scalarNodeJSON(node, "default")
}

func fieldCheckJSON(check *schema.Check) ([]byte, error) {
	if check == nil {
		return nil, nil
	}
	value, err := checkValueAny(check.Value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"operator": check.Operator,
		"value":    value,
	})
}

func constraintValueJSON(node yaml.Node) ([]byte, error) {
	if node.Kind == 0 {
		return nil, nil
	}
	value, err := checkValueAny(node)
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func checkValueAny(node yaml.Node) (any, error) {
	if node.Kind == yaml.SequenceNode {
		values := make([]any, 0, len(node.Content))
		for _, item := range node.Content {
			value, err := scalarNodeAny(*item, "value")
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	}
	return scalarNodeAny(node, "value")
}

func scalarNodeJSON(node yaml.Node, name string) ([]byte, error) {
	if node.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("%s must be a scalar value", name)
	}
	value, err := scalarNodeAny(node, name)
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func scalarNodeAny(node yaml.Node, name string) (any, error) {
	if node.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("%s must be a scalar value", name)
	}
	switch node.Tag {
	case "!!bool":
		value, err := strconv.ParseBool(node.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid boolean %s %q", name, node.Value)
		}
		return value, nil
	case "!!int":
		value, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer %s %q", name, node.Value)
		}
		return value, nil
	case "!!float":
		value, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float %s %q", name, node.Value)
		}
		return value, nil
	case "!!null":
		return nil, nil
	default:
		return node.Value, nil
	}
}

func entityKey(appName string, entityName string) string {
	return appName + "\x00" + entityName
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

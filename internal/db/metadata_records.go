package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/jackc/pgx/v5"
	"gopkg.in/yaml.v3"
)

type metadataPersistResult struct {
	Apps     int
	Entities int
	Fields   int
}

type metadataRecordSet struct {
	Apps     []appRecord
	Entities []entityRecord
	Fields   []fieldRecord
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
	Label       string
	PluralName  string
	PluralLabel string
	Description string
}

type fieldRecord struct {
	EntityAppName string
	EntityName    string
	Name          string
	Label         string
	Type          string
	Required      bool
	Unique        bool
	Index         bool
	Default       []byte
	Position      int
	Options       []byte
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
INSERT INTO apps (name, label, version, status)
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
INSERT INTO entities (app_id, name, label, plural_name, plural_label, description)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (name) DO UPDATE
SET app_id = EXCLUDED.app_id,
	label = EXCLUDED.label,
	plural_name = EXCLUDED.plural_name,
	plural_label = EXCLUDED.plural_label,
	description = EXCLUDED.description,
	updated_at = now()
RETURNING id`, appID, entity.Name, entity.Label, entity.PluralName, entity.PluralLabel, entity.Description).Scan(&id); err != nil {
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

	return metadataPersistResult{Apps: len(records.Apps), Entities: len(records.Entities), Fields: len(records.Fields)}, nil
}

func persistFieldRecord(ctx context.Context, tx pgx.Tx, entityID int64, field fieldRecord) error {
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM fields WHERE entity_id = $1 AND name = $2`, entityID, field.Name).Scan(&id)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("find field metadata %s/%s.%s: %w", field.EntityAppName, field.EntityName, field.Name, err)
	}
	if err == pgx.ErrNoRows {
		if _, err := tx.Exec(ctx, `
INSERT INTO fields (entity_id, name, label, type, required, "unique", "index", "default", position, options)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, entityID, field.Name, field.Label, field.Type, field.Required, field.Unique, field.Index, field.Default, field.Position, field.Options); err != nil {
			return fmt.Errorf("persist field metadata %s/%s.%s: %w", field.EntityAppName, field.EntityName, field.Name, err)
		}
		return nil
	}
	if _, err := tx.Exec(ctx, `
UPDATE fields
SET label = $2,
	type = $3,
	required = $4,
	"unique" = $5,
	"index" = $6,
	"default" = $7,
	position = $8,
	options = $9,
	updated_at = now()
WHERE id = $1`, id, field.Label, field.Type, field.Required, field.Unique, field.Index, field.Default, field.Position, field.Options); err != nil {
		return fmt.Errorf("persist field metadata %s/%s.%s: %w", field.EntityAppName, field.EntityName, field.Name, err)
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
		records.Entities = append(records.Entities, entityRecord{
			AppName:     loaded.AppName,
			Name:        loaded.Entity.Name,
			Label:       loaded.Entity.Label,
			PluralName:  loaded.Entity.PluralName,
			PluralLabel: loaded.Entity.PluralLabel,
			Description: loaded.Entity.Description,
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
			records.Fields = append(records.Fields, fieldRecord{
				EntityAppName: loaded.AppName,
				EntityName:    loaded.Entity.Name,
				Name:          field.Name,
				Label:         field.Label,
				Type:          field.Type,
				Required:      field.Required,
				Unique:        field.Unique,
				Index:         field.Index,
				Default:       defaultJSON,
				Position:      index + 1,
				Options:       optionsJSON,
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
	if options.Entity != "" {
		values["entity"] = options.Entity
	}
	if len(values) == 0 {
		return nil, nil
	}
	return json.Marshal(values)
}

func fieldDefaultJSON(node yaml.Node) ([]byte, error) {
	if node.Kind == 0 {
		return nil, nil
	}
	if node.Kind != yaml.ScalarNode {
		return nil, fmt.Errorf("default must be a scalar value")
	}
	switch node.Tag {
	case "!!bool":
		value, err := strconv.ParseBool(node.Value)
		if err != nil {
			return nil, fmt.Errorf("invalid boolean default %q", node.Value)
		}
		return json.Marshal(value)
	case "!!int":
		value, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer default %q", node.Value)
		}
		return json.Marshal(value)
	case "!!float":
		value, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float default %q", node.Value)
		}
		return json.Marshal(value)
	case "!!null":
		return json.Marshal(nil)
	default:
		return json.Marshal(node.Value)
	}
}

func entityKey(appName string, entityName string) string {
	return appName + "\x00" + entityName
}

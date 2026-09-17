package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hapyco/dygo/internal/corevalues"
	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/internal/entity/schema"
	"github.com/hapyco/dygo/internal/jobs"
	namegen "github.com/hapyco/dygo/internal/naming"
	"github.com/hapyco/dygo/internal/schedules"
	"github.com/jackc/pgx/v5"
	"gopkg.in/yaml.v3"
)

type metadataPersistResult struct {
	Apps        int
	Entities    int
	Pages       int
	Jobs        int
	Schedules   int
	Fields      int
	Indexes     int
	Constraints int
}

type metadataRecordSet struct {
	Apps        []appRecord
	Entities    []entityRecord
	Pages       []pageRecord
	Jobs        []jobRecord
	Schedules   []scheduleRecord
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
	AppName           string
	Name              string
	Key               string
	Slug              *string
	Label             string
	Description       string
	Icon              string
	IsSingle          bool
	IsSystem          bool
	IsCollection      bool
	IsPrivate         bool
	PrivateOwnerField string
	Naming            []byte
	Tree              []byte
}

type pageRecord struct {
	AppName     string
	Name        string
	Key         string
	Label       string
	Description string
	Icon        string
	Path        string
	Renderer    string
	Options     []byte
	Source      string
	Retired     bool
}

type jobRecord struct {
	AppName     string
	Name        string
	Key         string
	Source      string
	Label       string
	Description string
	Queue       string
	Cron        string
	Timeout     string
	Retry       []byte
	Enabled     bool
	Retired     bool
}

type scheduleRecord struct {
	AppName     string
	Name        string
	Key         string
	Source      string
	Label       string
	Description string
	Cron        string
	Timezone    string
	JobAppName  string
	JobName     string
	Enabled     bool
	Retired     bool
	NextRunAt   time.Time
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
	Fetch         []byte
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
	if err := validateTreeData(ctx, tx, metadata.Entities); err != nil {
		return metadataPersistResult{}, err
	}
	records, err := buildMetadataRecords(metadata)
	if err != nil {
		return metadataPersistResult{}, err
	}

	if err := reconcileMetadataRecords(ctx, tx, records); err != nil {
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
	status = "app".status,
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
			return metadataPersistResult{}, fmt.Errorf("persist entity metadata %q: app %q was not persisted", entity.Key, entity.AppName)
		}
		var id int64
		if err := tx.QueryRow(ctx, `
INSERT INTO "entity" (app_id, name, key, slug, label, description, icon, is_single, is_system, is_collection, is_private, private_owner_field, naming, tree)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (name) DO UPDATE
SET app_id = EXCLUDED.app_id,
	name = EXCLUDED.name,
	key = EXCLUDED.key,
	slug = EXCLUDED.slug,
	label = EXCLUDED.label,
	description = EXCLUDED.description,
	icon = EXCLUDED.icon,
	is_single = EXCLUDED.is_single,
	is_system = EXCLUDED.is_system,
	is_collection = EXCLUDED.is_collection,
	is_private = EXCLUDED.is_private,
	private_owner_field = EXCLUDED.private_owner_field,
	naming = EXCLUDED.naming,
	tree = EXCLUDED.tree,
	retired = false,
	updated_at = now()
RETURNING id`, appID, entity.Name, entity.Key, entity.Slug, entity.Label, entity.Description, entity.Icon, entity.IsSingle, entity.IsSystem, entity.IsCollection, entity.IsPrivate, nullIfEmpty(entity.PrivateOwnerField), entity.Naming, entity.Tree).Scan(&id); err != nil {
			return metadataPersistResult{}, fmt.Errorf("persist entity metadata %s/%s: %w", entity.AppName, entity.Key, err)
		}
		entityIDs[entityKey(entity.AppName, entity.Key)] = id
	}

	for _, page := range records.Pages {
		appID, ok := appIDs[page.AppName]
		if !ok {
			return metadataPersistResult{}, fmt.Errorf("persist page metadata %q: app %q was not persisted", page.Key, page.AppName)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO "page" (name, app_id, key, label, description, icon, path, renderer, options, source, retired)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (app_id, key) DO UPDATE
SET name = EXCLUDED.name,
	label = EXCLUDED.label,
	description = EXCLUDED.description,
	icon = EXCLUDED.icon,
	path = EXCLUDED.path,
	renderer = EXCLUDED.renderer,
	options = EXCLUDED.options,
	source = EXCLUDED.source,
	retired = false,
	updated_at = now()`, page.Name, appID, page.Key, page.Label, nullIfEmpty(page.Description), nullIfEmpty(page.Icon), page.Path, page.Renderer, page.Options, page.Source, page.Retired); err != nil {
			return metadataPersistResult{}, fmt.Errorf("persist page metadata %s/%s: %w", page.AppName, page.Key, err)
		}
	}

	jobIDs := map[string]int64{}
	for _, job := range records.Jobs {
		appID, ok := appIDs[job.AppName]
		if !ok {
			return metadataPersistResult{}, fmt.Errorf("persist job metadata %q: app %q was not persisted", job.Key, job.AppName)
		}
		jobID, err := persistJobRecord(ctx, tx, appID, job)
		if err != nil {
			return metadataPersistResult{}, err
		}
		jobIDs[jobKey(job.AppName, job.Key)] = jobID
	}

	for _, schedule := range records.Schedules {
		appID, ok := appIDs[schedule.AppName]
		if !ok {
			return metadataPersistResult{}, fmt.Errorf("persist schedule metadata %q: app %q was not persisted", schedule.Key, schedule.AppName)
		}
		jobID, ok := jobIDs[jobKey(schedule.JobAppName, schedule.JobName)]
		if !ok {
			return metadataPersistResult{}, fmt.Errorf("persist schedule metadata %s/%s: target job %s/%s was not persisted", schedule.AppName, schedule.Key, schedule.JobAppName, schedule.JobName)
		}
		if err := persistScheduleRecord(ctx, tx, appID, jobID, schedule); err != nil {
			return metadataPersistResult{}, err
		}
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
		Pages:       len(records.Pages),
		Jobs:        len(records.Jobs),
		Schedules:   len(records.Schedules),
		Fields:      len(records.Fields),
		Indexes:     len(records.Indexes),
		Constraints: len(records.Constraints),
	}, nil
}

// reconcileMetadataRecords retains registry IDs used by history and collection ownership.
// Only authored definitions remain active. Physical data is removed by explicit prune.
// This bootstrap path cannot use Records: it defines the metadata that Records load.
func reconcileMetadataRecords(ctx context.Context, tx pgx.Tx, records metadataRecordSet) error {
	names := map[string][]string{"entity": {}, "field": {}, "index": {}, "constraint": {}, "page": {}, "job": {}, "schedule": {}}
	for _, entity := range records.Entities {
		names["entity"] = append(names["entity"], entity.Name)
	}
	for _, field := range records.Fields {
		names["field"] = append(names["field"], field.RecordName)
	}
	for _, index := range records.Indexes {
		names["index"] = append(names["index"], index.RecordName)
	}
	for _, constraint := range records.Constraints {
		names["constraint"] = append(names["constraint"], constraint.RecordName)
	}
	for _, page := range records.Pages {
		names["page"] = append(names["page"], page.Name)
	}
	for _, job := range records.Jobs {
		names["job"] = append(names["job"], job.Name)
	}
	for _, schedule := range records.Schedules {
		names["schedule"] = append(names["schedule"], schedule.Name)
	}
	for _, table := range []string{"entity", "field", "index", "constraint", "page", "job", "schedule"} {
		set := "retired = NOT (name = ANY($1::text[])), updated_at = now()"
		if table == "entity" {
			// Release removed routes before current definitions are upserted.
			set += ", slug = CASE WHEN name = ANY($1::text[]) THEN slug ELSE NULL END"
		}
		query := "UPDATE " + quoteIdent(table) + " SET " + set + " WHERE retired IS DISTINCT FROM (NOT (name = ANY($1::text[])))"
		if table == "page" || table == "job" || table == "schedule" {
			// File-backed upserts reactivate definitions and reset schedule timing.
			query += " AND source = 'file' AND NOT (name = ANY($1::text[]))"
		}
		if _, err := tx.Exec(ctx, query, names[table]); err != nil {
			return fmt.Errorf("reconcile %s metadata: %w", table, err)
		}
	}
	return nil
}

func persistJobRecord(ctx context.Context, tx pgx.Tx, appID int64, job jobRecord) (int64, error) {
	source := strings.TrimSpace(job.Source)
	if source == "" {
		source = jobs.JobSourceFile
	}
	var id int64
	err := tx.QueryRow(ctx, `
	INSERT INTO "job" (name, app_id, key, source, label, description, queue, cron, timeout, retry, enabled, retired)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	ON CONFLICT (app_id, key) DO UPDATE
SET name = EXCLUDED.name,
	source = EXCLUDED.source,
	label = EXCLUDED.label,
	description = EXCLUDED.description,
	queue = EXCLUDED.queue,
	cron = EXCLUDED.cron,
	timeout = EXCLUDED.timeout,
	retry = EXCLUDED.retry,
	retired = false,
	updated_at = now()
	WHERE "job"."source" = $13
	RETURNING id`, job.Name, appID, job.Key, source, job.Label, nullIfEmpty(job.Description), job.Queue, nullIfEmpty(job.Cron), job.Timeout, job.Retry, job.Enabled, job.Retired, jobs.JobSourceFile).Scan(&id)
	if err != nil && err != pgx.ErrNoRows {
		return 0, fmt.Errorf("persist job metadata %s/%s: %w", job.AppName, job.Key, err)
	}
	if err == pgx.ErrNoRows {
		var existingSource string
		if err := tx.QueryRow(ctx, `SELECT source FROM "job" WHERE app_id = $1 AND key = $2`, appID, job.Key).Scan(&existingSource); err != nil {
			return 0, fmt.Errorf("persist job metadata %s/%s: load existing job source: %w", job.AppName, job.Key, err)
		}
		return 0, fmt.Errorf("persist job metadata %s/%s: existing job source %q cannot be overwritten by file-backed metadata", job.AppName, job.Key, existingSource)
	}
	return id, nil
}

func persistScheduleRecord(ctx context.Context, tx pgx.Tx, appID int64, jobID int64, schedule scheduleRecord) error {
	source := strings.TrimSpace(schedule.Source)
	if source == "" {
		source = schedules.ScheduleSourceFile
	}
	tag, err := tx.Exec(ctx, `
INSERT INTO "schedule" (name, app_id, key, source, label, description, cron, timezone, job_id, job_app_name, job_name, enabled, retired, next_run_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (app_id, key) DO UPDATE
SET name = EXCLUDED.name,
	source = EXCLUDED.source,
	label = EXCLUDED.label,
	description = EXCLUDED.description,
	cron = EXCLUDED.cron,
	timezone = EXCLUDED.timezone,
	job_id = EXCLUDED.job_id,
	job_app_name = EXCLUDED.job_app_name,
	job_name = EXCLUDED.job_name,
	enabled = EXCLUDED.enabled,
	retired = false,
	next_run_at = CASE
		WHEN "schedule".cron IS DISTINCT FROM EXCLUDED.cron
			OR "schedule".timezone IS DISTINCT FROM EXCLUDED.timezone
			OR "schedule".enabled IS DISTINCT FROM EXCLUDED.enabled
			OR "schedule".retired = true
		THEN EXCLUDED.next_run_at
		ELSE "schedule".next_run_at
	END,
	updated_at = now()
WHERE "schedule"."source" = $15`, schedule.Name, appID, schedule.Key, source, schedule.Label, nullIfEmpty(schedule.Description), schedule.Cron, schedule.Timezone, jobID, schedule.JobAppName, schedule.JobName, schedule.Enabled, schedule.Retired, schedule.NextRunAt, schedules.ScheduleSourceFile)
	if err != nil {
		return fmt.Errorf("persist schedule metadata %s/%s: %w", schedule.AppName, schedule.Key, err)
	}
	if tag.RowsAffected() == 0 {
		var existingSource string
		if err := tx.QueryRow(ctx, `SELECT source FROM "schedule" WHERE app_id = $1 AND key = $2`, appID, schedule.Key).Scan(&existingSource); err != nil {
			return fmt.Errorf("persist schedule metadata %s/%s: load existing schedule source: %w", schedule.AppName, schedule.Key, err)
		}
		return fmt.Errorf("persist schedule metadata %s/%s: existing schedule source %q cannot be overwritten by file-backed metadata", schedule.AppName, schedule.Key, existingSource)
	}
	return nil
}

func persistFieldRecord(ctx context.Context, tx pgx.Tx, entityID int64, field fieldRecord) error {
	var id int64
	err := tx.QueryRow(ctx, `SELECT id FROM "field" WHERE entity_id = $1 AND field_name = $2`, entityID, field.Name).Scan(&id)
	if err != nil && err != pgx.ErrNoRows {
		return fmt.Errorf("find field metadata %s/%s.%s: %w", field.EntityAppName, field.EntityName, field.Name, err)
	}
	if err == pgx.ErrNoRows {
		if _, err := tx.Exec(ctx, `
INSERT INTO "field" (name, entity_id, field_name, label, type, required, "unique", "index", "default", "check", "fetch", position, options)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`, field.RecordName, entityID, field.Name, field.Label, field.Type, field.Required, field.Unique, field.Index, field.Default, field.Check, field.Fetch, field.Position, field.Options); err != nil {
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
	"fetch" = $10,
	position = $11,
	options = $12,
	retired = false,
	updated_at = now()
WHERE id = $1`, id, field.RecordName, field.Label, field.Type, field.Required, field.Unique, field.Index, field.Default, field.Check, field.Fetch, field.Position, field.Options); err != nil {
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
INSERT INTO "index" (name, entity_id, index_name, field_names, position)
VALUES ($1, $2, $3, $4, $5)`, index.RecordName, entityID, index.Name, index.Fields, index.Position); err != nil {
			return fmt.Errorf("persist index metadata %s/%s.%s: %w", index.EntityAppName, index.EntityName, index.Name, err)
		}
		return nil
	}
	if _, err := tx.Exec(ctx, `
UPDATE "index"
SET name = $2,
	field_names = $3,
	position = $4,
	retired = false,
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
INSERT INTO "constraint" (name, entity_id, constraint_name, type, field_names, field, operator, value, position)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, constraint.RecordName, entityID, constraint.Name, constraint.Type, constraint.Fields, nullIfEmpty(constraint.Field), nullIfEmpty(constraint.Operator), constraint.Value, constraint.Position); err != nil {
			return fmt.Errorf("persist constraint metadata %s/%s.%s: %w", constraint.EntityAppName, constraint.EntityName, constraint.Name, err)
		}
		return nil
	}
	if _, err := tx.Exec(ctx, `
UPDATE "constraint"
SET name = $2,
	type = $3,
	field_names = $4,
	field = $5,
	operator = $6,
	value = $7,
	position = $8,
	retired = false,
	updated_at = now()
WHERE id = $1`, id, constraint.RecordName, constraint.Type, constraint.Fields, nullIfEmpty(constraint.Field), nullIfEmpty(constraint.Operator), constraint.Value, constraint.Position); err != nil {
		return fmt.Errorf("persist constraint metadata %s/%s.%s: %w", constraint.EntityAppName, constraint.EntityName, constraint.Name, err)
	}
	return nil
}

func buildMetadataRecords(metadata metadataCatalog) (metadataRecordSet, error) {
	records := metadataRecordSet{}
	namings := metadataRecordNamings(metadata.Entities)
	for _, app := range metadata.Apps {
		records.Apps = append(records.Apps, appRecord{
			Name:    app.Manifest.Name,
			Label:   app.Manifest.Label,
			Version: app.Manifest.Version,
			Status:  corevalues.AppStatusInstalled,
		})
	}
	for _, loaded := range metadata.Entities {
		var namingJSON []byte
		if !loaded.Entity.IsSingle && !loaded.IsCollection() {
			var err error
			namingJSON, err = entityNamingJSON(loaded.Entity.EffectiveNaming())
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build entity metadata %s/%s naming: %w", loaded.AppName, loaded.Entity.Name, err)
			}
		}
		entityName, err := metadataEntityRecordName(loaded, namings.Entity)
		if err != nil {
			return metadataRecordSet{}, fmt.Errorf("build entity metadata %s/%s name: %w", loaded.AppName, loaded.Entity.Name, err)
		}
		slug := stringPointerOrNil(loaded.RouteSlug())
		var treeJSON []byte
		if loaded.Entity.Tree != nil {
			treeJSON, err = json.Marshal(loaded.Entity.Tree)
			if err != nil {
				return metadataRecordSet{}, err
			}
		}
		records.Entities = append(records.Entities, entityRecord{
			AppName:           loaded.AppName,
			Name:              entityName,
			Key:               loaded.Entity.Name,
			Slug:              slug,
			Label:             loaded.Entity.Label,
			Description:       loaded.Entity.Description,
			Icon:              strings.TrimSpace(loaded.Entity.Icon),
			IsSingle:          loaded.Entity.IsSingle,
			IsSystem:          loaded.Entity.IsSystem,
			IsCollection:      loaded.IsCollection() || loaded.Entity.IsCollection,
			IsPrivate:         loaded.Entity.IsPrivate,
			PrivateOwnerField: strings.TrimSpace(loaded.Entity.PrivateOwnerField),
			Naming:            namingJSON,
			Tree:              treeJSON,
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
			fetchJSON, err := fieldFetchJSON(field.Fetch)
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build field metadata %s/%s.%s fetch: %w", loaded.AppName, loaded.Entity.Name, field.Name, err)
			}
			recordName, err := deterministicRecordNameFromValues("field", namings.Field, map[string]string{
				"entity":     entityName,
				"field-name": field.Name,
				"label":      field.Label,
				"type":       field.Type,
				"required":   strconv.FormatBool(field.Required),
				"unique":     strconv.FormatBool(field.Unique),
				"index":      strconv.FormatBool(field.Index),
				"position":   strconv.Itoa(index + 1),
			})
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build field metadata %s/%s.%s name: %w", loaded.AppName, loaded.Entity.Name, field.Name, err)
			}
			records.Fields = append(records.Fields, fieldRecord{
				EntityAppName: loaded.AppName,
				EntityName:    loaded.Entity.Name,
				RecordName:    recordName,
				Name:          field.Name,
				Label:         field.Label,
				Type:          field.Type,
				Required:      field.Required,
				Unique:        field.Unique,
				Index:         field.Index,
				Default:       defaultJSON,
				Check:         checkJSON,
				Fetch:         fetchJSON,
				Position:      index + 1,
				Options:       optionsJSON,
			})
		}
		for indexPosition, index := range loaded.Entity.Indexes {
			fieldsJSON, err := json.Marshal(index.Fields)
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build index metadata %s/%s.%s fields: %w", loaded.AppName, loaded.Entity.Name, index.EffectiveName(loaded.Entity), err)
			}
			indexName := index.EffectiveName(loaded.Entity)
			recordName, err := deterministicRecordNameFromValues("index", namings.Index, map[string]string{
				"entity":     entityName,
				"index-name": indexName,
				"position":   strconv.Itoa(indexPosition + 1),
			})
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build index metadata %s/%s.%s name: %w", loaded.AppName, loaded.Entity.Name, indexName, err)
			}
			records.Indexes = append(records.Indexes, indexRecord{
				EntityAppName: loaded.AppName,
				EntityName:    loaded.Entity.Name,
				RecordName:    recordName,
				Name:          indexName,
				Fields:        fieldsJSON,
				Position:      indexPosition + 1,
			})
		}
		for constraintPosition, constraint := range loaded.Entity.Constraints {
			constraintName := constraint.EffectiveName(loaded.Entity)
			fieldsJSON, err := json.Marshal(constraint.Fields)
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build constraint metadata %s/%s.%s fields: %w", loaded.AppName, loaded.Entity.Name, constraintName, err)
			}
			valueJSON, err := constraintValueJSON(constraint.Value)
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build constraint metadata %s/%s.%s value: %w", loaded.AppName, loaded.Entity.Name, constraintName, err)
			}
			recordName, err := deterministicRecordNameFromValues("constraint", namings.Constraint, map[string]string{
				"entity":          entityName,
				"constraint-name": constraintName,
				"type":            constraint.Type,
				"field":           constraint.Field,
				"operator":        constraint.Operator,
				"position":        strconv.Itoa(constraintPosition + 1),
			})
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build constraint metadata %s/%s.%s name: %w", loaded.AppName, loaded.Entity.Name, constraintName, err)
			}
			records.Constraints = append(records.Constraints, constraintRecord{
				EntityAppName: loaded.AppName,
				EntityName:    loaded.Entity.Name,
				RecordName:    recordName,
				Name:          constraintName,
				Type:          constraint.Type,
				Fields:        fieldsJSON,
				Field:         constraint.Field,
				Operator:      constraint.Operator,
				Value:         valueJSON,
				Position:      constraintPosition + 1,
			})
		}
	}
	for _, loaded := range metadata.Pages {
		optionsJSON, err := json.Marshal(loaded.Page.Options)
		if err != nil {
			return metadataRecordSet{}, fmt.Errorf("build page metadata %s/%s options: %w", loaded.AppName, loaded.Page.Key, err)
		}
		records.Pages = append(records.Pages, pageRecord{
			AppName:     loaded.AppName,
			Name:        loaded.AppName + "." + loaded.Page.Key,
			Key:         loaded.Page.Key,
			Label:       loaded.Page.Label,
			Description: loaded.Page.Description,
			Icon:        loaded.Page.Icon,
			Path:        loaded.RoutePath(),
			Renderer:    loaded.Page.Renderer,
			Options:     optionsJSON,
			Source:      "file",
		})
	}
	for _, loaded := range metadata.Jobs {
		retryJSON, err := jobRetryJSON(loaded.Job.EffectiveRetry())
		if err != nil {
			return metadataRecordSet{}, fmt.Errorf("build job metadata %s/%s retry: %w", loaded.AppName, loaded.Job.Name, err)
		}
		recordName, err := deterministicRecordNameFromValues("job", namings.Job, map[string]string{
			"app":     loaded.AppName,
			"key":     loaded.Job.Name,
			"label":   loaded.Job.Label,
			"queue":   loaded.Job.EffectiveQueue(),
			"timeout": loaded.Job.Timeout,
		})
		if err != nil {
			return metadataRecordSet{}, fmt.Errorf("build job metadata %s/%s name: %w", loaded.AppName, loaded.Job.Name, err)
		}
		records.Jobs = append(records.Jobs, jobRecord{
			AppName:     loaded.AppName,
			Name:        recordName,
			Key:         loaded.Job.Name,
			Source:      jobs.JobSourceFile,
			Label:       loaded.Job.Label,
			Description: loaded.Job.Description,
			Queue:       loaded.Job.EffectiveQueue(),
			Cron:        loaded.Job.Cron,
			Timeout:     loaded.Job.Timeout,
			Retry:       retryJSON,
			Enabled:     true,
			Retired:     false,
		})
		if strings.TrimSpace(loaded.Job.Cron) != "" {
			nextRunAt, err := schedules.NextRunAt(loaded.Job.Cron, "UTC", time.Now().UTC())
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build Job schedule metadata %s/%s: %w", loaded.AppName, loaded.Job.Name, err)
			}
			scheduleKey := "job-" + loaded.Job.Name
			scheduleName, err := deterministicRecordNameFromValues("schedule", namings.Schedule, map[string]string{
				"app": loaded.AppName, "key": scheduleKey, "label": loaded.Job.Label,
				"cron": loaded.Job.Cron, "timezone": "UTC", "job": loaded.AppName + "/" + loaded.Job.Name,
			})
			if err != nil {
				return metadataRecordSet{}, fmt.Errorf("build Job schedule metadata %s/%s name: %w", loaded.AppName, loaded.Job.Name, err)
			}
			records.Schedules = append(records.Schedules, scheduleRecord{
				AppName: loaded.AppName, Name: scheduleName, Key: scheduleKey,
				Source: schedules.ScheduleSourceFile, Label: loaded.Job.Label,
				Description: "Schedule for Job " + loaded.AppName + "/" + loaded.Job.Name,
				Cron:        loaded.Job.Cron, Timezone: "UTC", JobAppName: loaded.AppName,
				JobName: loaded.Job.Name, Enabled: true, NextRunAt: nextRunAt,
			})
		}
	}
	for _, loaded := range metadata.Schedules {
		jobRef, err := loaded.Schedule.JobRef()
		if err != nil {
			return metadataRecordSet{}, fmt.Errorf("build schedule metadata %s/%s job: %w", loaded.AppName, loaded.Schedule.Name, err)
		}
		nextRunAt, err := schedules.NextRunAt(loaded.Schedule.Cron, loaded.Schedule.Timezone, time.Now().UTC())
		if err != nil {
			return metadataRecordSet{}, fmt.Errorf("build schedule metadata %s/%s next run: %w", loaded.AppName, loaded.Schedule.Name, err)
		}
		recordName, err := deterministicRecordNameFromValues("schedule", namings.Schedule, map[string]string{
			"app":      loaded.AppName,
			"key":      loaded.Schedule.Name,
			"label":    loaded.Schedule.Label,
			"cron":     loaded.Schedule.Cron,
			"timezone": loaded.Schedule.Timezone,
			"job":      loaded.Schedule.Job,
		})
		if err != nil {
			return metadataRecordSet{}, fmt.Errorf("build schedule metadata %s/%s name: %w", loaded.AppName, loaded.Schedule.Name, err)
		}
		records.Schedules = append(records.Schedules, scheduleRecord{
			AppName:     loaded.AppName,
			Name:        recordName,
			Key:         loaded.Schedule.Name,
			Source:      schedules.ScheduleSourceFile,
			Label:       loaded.Schedule.Label,
			Description: loaded.Schedule.Description,
			Cron:        loaded.Schedule.Cron,
			Timezone:    loaded.Schedule.Timezone,
			JobAppName:  jobRef.App,
			JobName:     jobRef.Name,
			Enabled:     loaded.Schedule.EffectiveEnabled(),
			Retired:     false,
			NextRunAt:   nextRunAt,
		})
	}
	return records, nil
}

type metadataNamings struct {
	Entity     schema.Naming
	Job        schema.Naming
	Schedule   schema.Naming
	Field      schema.Naming
	Index      schema.Naming
	Constraint schema.Naming
}

func metadataRecordNamings(entities []catalog.LoadedEntity) metadataNamings {
	return metadataNamings{
		Entity: metadataCoreEntityNaming(entities, "entity", schema.Naming{
			Strategy: schema.NamingStrategyFormat,
			Format:   "{app}.{key}",
		}),
		Job: metadataCoreEntityNaming(entities, "job", schema.Naming{
			Strategy: schema.NamingStrategyFormat,
			Format:   "{app}.{key}",
		}),
		Schedule: metadataCoreEntityNaming(entities, "schedule", schema.Naming{
			Strategy: schema.NamingStrategyFormat,
			Format:   "{app}.{key}",
		}),
		Field: metadataCoreEntityNaming(entities, "field", schema.Naming{
			Strategy: schema.NamingStrategyFormat,
			Format:   "{entity}.{field-name}",
		}),
		Index: metadataCoreEntityNaming(entities, "index", schema.Naming{
			Strategy: schema.NamingStrategyFormat,
			Format:   "{entity}.{index-name}",
		}),
		Constraint: metadataCoreEntityNaming(entities, "constraint", schema.Naming{
			Strategy: schema.NamingStrategyFormat,
			Format:   "{entity}.{constraint-name}",
		}),
	}
}

func metadataCoreEntityNaming(entities []catalog.LoadedEntity, key string, fallback schema.Naming) schema.Naming {
	for _, loaded := range entities {
		if loaded.AppName == "core" && loaded.Entity.Name == key {
			return loaded.Entity.EffectiveNaming()
		}
	}
	return fallback
}

func metadataEntityRecordName(loaded catalog.LoadedEntity, naming schema.Naming) (string, error) {
	values := map[string]string{
		"app":           loaded.AppName,
		"key":           loaded.Entity.Name,
		"slug":          loaded.RouteSlug(),
		"label":         loaded.Entity.Label,
		"description":   loaded.Entity.Description,
		"icon":          strings.TrimSpace(loaded.Entity.Icon),
		"is-single":     strconv.FormatBool(loaded.Entity.IsSingle),
		"is-system":     strconv.FormatBool(loaded.Entity.IsSystem),
		"is-collection": strconv.FormatBool(loaded.IsCollection() || loaded.Entity.IsCollection),
	}
	return deterministicRecordNameFromValues("entity", naming, values)
}

func deterministicRecordNameFromValues(kind string, naming schema.Naming, values map[string]string) (string, error) {
	name, err := namegen.GenerateDeterministic(context.Background(), naming, namegen.MapResolver(values))
	if err != nil {
		return "", fmt.Errorf("%s metadata naming: %w", kind, err)
	}
	return name, nil
}

func entityRecordName(appName string, key string) string {
	return appName + "." + key
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
	if options.DisplayField != "" {
		values["display-field"] = options.DisplayField
	}
	if len(options.Filters) > 0 {
		values["filters"] = options.Filters
	}
	if options.ForeignKey != nil {
		values["foreign-key"] = *options.ForeignKey
	}
	if len(values) == 0 {
		return nil, nil
	}
	return json.Marshal(values)
}

func entityNamingJSON(naming schema.Naming) ([]byte, error) {
	return json.Marshal(recordNaming{
		Strategy: naming.Strategy,
		Label:    naming.Label,
		Length:   naming.Length,
		Pattern:  naming.Pattern,
		Format:   naming.Format,
	})
}

func jobRetryJSON(retry *jobs.Retry) ([]byte, error) {
	if retry == nil {
		return nil, nil
	}
	return json.Marshal(map[string]any{
		"attempts":      retry.Attempts,
		"initial-delay": retry.InitialDelay,
		"max-delay":     retry.MaxDelay,
	})
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

func fieldFetchJSON(fetch *schema.Fetch) ([]byte, error) {
	if fetch == nil {
		return nil, nil
	}
	return json.Marshal(fetch)
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

func jobKey(appName string, jobName string) string {
	return appName + "\x00" + jobName
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

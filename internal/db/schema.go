package db

import (
	"context"
	"fmt"

	"github.com/dygo-dev/dygo/internal/app/manifest"
	"github.com/dygo-dev/dygo/internal/app/registry"
	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PlanMetadataSchema compares discovered Entity metadata with the live database.
func PlanMetadataSchema(ctx context.Context, pool *pgxpool.Pool, root string) (SchemaPlan, error) {
	metadata, err := loadMetadataCatalog(root)
	if err != nil {
		return SchemaPlan{}, err
	}
	live, err := InspectLiveSchema(ctx, pool)
	if err != nil {
		return SchemaPlan{}, err
	}
	return BuildMetadataSchemaPlan(metadata.Entities, live)
}

// SyncMetadataSchema creates or updates PostgreSQL tables from discovered app Entity metadata.
func SyncMetadataSchema(ctx context.Context, pool *pgxpool.Pool, root string) (SchemaSyncResult, error) {
	metadata, err := loadMetadataCatalog(root)
	if err != nil {
		return SchemaSyncResult{}, err
	}
	live, err := InspectLiveSchema(ctx, pool)
	if err != nil {
		return SchemaSyncResult{}, err
	}
	plan, err := BuildMetadataSchemaPlan(metadata.Entities, live)
	if err != nil {
		return SchemaSyncResult{}, err
	}
	return applyMetadataSchemaPlanAndRecords(ctx, pool, plan, metadata)
}

// ApplyMetadataSchema applies Entity metadata tables to PostgreSQL.
func ApplyMetadataSchema(ctx context.Context, pool *pgxpool.Pool, entities []catalog.LoadedEntity) (SchemaSyncResult, error) {
	live, err := InspectLiveSchema(ctx, pool)
	if err != nil {
		return SchemaSyncResult{}, err
	}
	plan, err := BuildMetadataSchemaPlan(entities, live)
	if err != nil {
		return SchemaSyncResult{}, err
	}
	return ApplyMetadataSchemaPlan(ctx, pool, plan)
}

// ApplyMetadataSchemaPlan applies safe operations from a schema plan in one transaction.
func ApplyMetadataSchemaPlan(ctx context.Context, pool *pgxpool.Pool, plan SchemaPlan) (SchemaSyncResult, error) {
	if err := plan.BlockerError(); err != nil {
		return SchemaSyncResult{}, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return SchemaSyncResult{}, fmt.Errorf("begin metadata schema transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := executeSchemaPlan(ctx, tx, plan); err != nil {
		return SchemaSyncResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SchemaSyncResult{}, fmt.Errorf("commit metadata schema transaction: %w", err)
	}
	return plan.Result(), nil
}

func applyMetadataSchemaPlanAndRecords(ctx context.Context, pool *pgxpool.Pool, plan SchemaPlan, metadata metadataCatalog) (SchemaSyncResult, error) {
	if err := plan.BlockerError(); err != nil {
		return SchemaSyncResult{}, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return SchemaSyncResult{}, fmt.Errorf("begin metadata schema transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := executeSchemaPlan(ctx, tx, plan); err != nil {
		return SchemaSyncResult{}, err
	}
	if _, err := persistMetadataRecords(ctx, tx, metadata); err != nil {
		return SchemaSyncResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SchemaSyncResult{}, fmt.Errorf("commit metadata schema transaction: %w", err)
	}
	result := plan.Result()
	result.Apps = len(metadata.Apps)
	return result, nil
}

func executeSchemaPlan(ctx context.Context, tx pgx.Tx, plan SchemaPlan) error {
	for _, operation := range plan.Operations {
		if _, err := tx.Exec(ctx, operation.SQL); err != nil {
			return fmt.Errorf("apply metadata schema operation %q: %w", operation.Description, err)
		}
	}
	return nil
}

type metadataCatalog struct {
	Apps     []manifest.LoadedApp
	Entities []catalog.LoadedEntity
}

func loadMetadataCatalog(root string) (metadataCatalog, error) {
	apps, err := registry.New(root).Validate()
	if err != nil {
		return metadataCatalog{}, fmt.Errorf("validate apps for metadata schema: %w", err)
	}
	entities, err := catalog.New(apps, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		return metadataCatalog{}, fmt.Errorf("validate entities for metadata schema: %w", err)
	}
	return metadataCatalog{Apps: apps, Entities: entities}, nil
}

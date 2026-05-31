package project

import (
	"fmt"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/app/registry"
	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/internal/jobs"
	"github.com/hapyco/dygo/internal/queues"
	"github.com/hapyco/dygo/internal/schedules"
)

// Metadata is the validated app and Entity context for one dygo project.
type Metadata struct {
	Apps     []manifest.LoadedApp
	Entities []catalog.LoadedEntity
}

// RuntimeMetadata is the validated metadata context used by runtime schema sync.
type RuntimeMetadata struct {
	Apps      []manifest.LoadedApp
	Entities  []catalog.LoadedEntity
	Queues    queues.Config
	Jobs      []jobs.LoadedJob
	Schedules []schedules.LoadedSchedule
}

// LoadApps discovers and validates app manifests for a project root.
func LoadApps(root string) ([]manifest.LoadedApp, error) {
	apps, err := registry.New(root).Validate()
	if err != nil {
		return nil, fmt.Errorf("validate apps: %w", err)
	}
	return apps, nil
}

// LoadEntities validates Entity metadata for already loaded apps.
func LoadEntities(apps []manifest.LoadedApp) ([]catalog.LoadedEntity, error) {
	entities, err := catalog.New(apps, fieldtype.DefaultRegistry()).Validate()
	if err != nil {
		return nil, fmt.Errorf("validate entities: %w", err)
	}
	return entities, nil
}

// LoadQueues validates the project queue registry.
func LoadQueues(root string) (queues.Config, error) {
	queueConfig, err := queues.Load(root)
	if err != nil {
		return queues.Config{}, fmt.Errorf("validate queues: %w", err)
	}
	return queueConfig, nil
}

// LoadJobs validates Job metadata for already loaded apps.
func LoadJobs(apps []manifest.LoadedApp, queueConfig queues.Config) ([]jobs.LoadedJob, error) {
	loaded, err := jobs.New(apps, queueConfig).Validate()
	if err != nil {
		return nil, fmt.Errorf("validate jobs: %w", err)
	}
	return loaded, nil
}

// LoadSchedules validates Schedule metadata for already loaded apps and Jobs.
func LoadSchedules(apps []manifest.LoadedApp, loadedJobs []jobs.LoadedJob) ([]schedules.LoadedSchedule, error) {
	loaded, err := schedules.New(apps, loadedJobs).Validate()
	if err != nil {
		return nil, fmt.Errorf("validate schedules: %w", err)
	}
	return loaded, nil
}

// LoadMetadata loads the validated app and Entity metadata context for a project root.
func LoadMetadata(root string) (Metadata, error) {
	apps, err := LoadApps(root)
	if err != nil {
		return Metadata{}, err
	}
	entities, err := LoadEntities(apps)
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{Apps: apps, Entities: entities}, nil
}

// LoadRuntimeMetadata loads metadata needed by database runtime sync.
func LoadRuntimeMetadata(root string) (RuntimeMetadata, error) {
	apps, err := LoadApps(root)
	if err != nil {
		return RuntimeMetadata{}, err
	}
	entities, err := LoadEntities(apps)
	if err != nil {
		return RuntimeMetadata{}, err
	}
	queueConfig, err := LoadQueues(root)
	if err != nil {
		return RuntimeMetadata{}, err
	}
	loadedJobs, err := LoadJobs(apps, queueConfig)
	if err != nil {
		return RuntimeMetadata{}, err
	}
	loadedSchedules, err := LoadSchedules(apps, loadedJobs)
	if err != nil {
		return RuntimeMetadata{}, err
	}
	return RuntimeMetadata{Apps: apps, Entities: entities, Queues: queueConfig, Jobs: loadedJobs, Schedules: loadedSchedules}, nil
}

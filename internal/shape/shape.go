// Package shape centralizes dygo project and app filesystem conventions.
package shape

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ProjectConfigFile = "dygo.yml"

	AppsDir          = "apps"
	ConfigDir        = "config"
	ConfigSecretsDir = "config/secrets"
	DatabaseDir      = "db"
	SchemaSnapshot   = "db/schema.sql"
	DocsDir          = "docs"

	LocalStateDir       = ".dygo"
	LocalAppsDir        = ".dygo/apps"
	LocalStudioAppDir   = ".dygo/apps/studio"
	LocalFilesDir       = ".dygo/files"
	LocalLogsDir        = ".dygo/logs"
	LocalTempDir        = ".dygo/tmp"
	LocalSecretsDir     = ".dygo/secrets"
	LocalSecretKeyFile  = ".dygo/secrets/master.key"
	LocalSecretsTempDir = ".dygo/secrets/tmp"

	AppManifestFile  = "app.yml"
	AppEntitiesDir   = "entities"
	AppJobsDir       = "jobs"
	AppPagesDir      = "pages"
	AppReportsDir    = "reports"
	AppRolesFile     = "roles.yml"
	AppSchedulesFile = "jobs/_schedules.yml"

	EntityMetadataFile    = "entity.yml"
	EntityFixturesFile    = "fixtures.yml"
	EntityHooksFile       = "hooks.go"
	EntityPermissionsFile = "permissions.yml"
	EntityViewsFile       = "views.yml"

	CollectionDir = "_collections"

	JobMetadataFile    = "job.yml"
	JobRunFile         = "run.go"
	PageMetadataFile   = "page.yml"
	ReportMetadataFile = "report.yml"
)

var metadataNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// AppRef identifies one app-owned object from CLI slash syntax.
type AppRef struct {
	App  string
	Name string
}

// ParseAppRef parses CLI targets such as crm/lead.
func ParseAppRef(value string) (AppRef, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return AppRef{}, fmt.Errorf("target %q must use <app>/<name>", value)
	}
	if !metadataNamePattern.MatchString(parts[0]) {
		return AppRef{}, fmt.Errorf("app %q must be kebab-case", parts[0])
	}
	if !metadataNamePattern.MatchString(parts[1]) {
		return AppRef{}, fmt.Errorf("name %q must be kebab-case", parts[1])
	}
	return AppRef{App: parts[0], Name: parts[1]}, nil
}

// ValidateMetadataName verifies one kebab-case metadata identifier.
func ValidateMetadataName(kind string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", kind)
	}
	if !metadataNamePattern.MatchString(value) {
		return fmt.Errorf("%s %q must be kebab-case", kind, value)
	}
	return nil
}

// AppDir returns the project-relative directory for an app.
func AppDir(app string) string {
	return filepath.ToSlash(filepath.Join(AppsDir, app))
}

// AppManifestPath returns the project-relative manifest path for an app.
func AppManifestPath(app string) string {
	return filepath.ToSlash(filepath.Join(AppDir(app), AppManifestFile))
}

// AppEntitiesPath returns the project-relative Entity root for an app.
func AppEntitiesPath(app string) string {
	return filepath.ToSlash(filepath.Join(AppDir(app), AppEntitiesDir))
}

// AppCollectionDirPath returns the project-relative collection root for an app.
func AppCollectionDirPath(app string) string {
	return filepath.ToSlash(filepath.Join(AppEntitiesPath(app), CollectionDir))
}

// AppJobsPath returns the project-relative jobs root for an app.
func AppJobsPath(app string) string {
	return filepath.ToSlash(filepath.Join(AppDir(app), AppJobsDir))
}

// AppSchedulesPath returns the project-relative recurring job schedules path for an app.
func AppSchedulesPath(app string) string {
	return filepath.ToSlash(filepath.Join(AppDir(app), AppSchedulesFile))
}

// AppPagesPath returns the project-relative pages root for an app.
func AppPagesPath(app string) string {
	return filepath.ToSlash(filepath.Join(AppDir(app), AppPagesDir))
}

// AppReportsPath returns the project-relative reports root for an app.
func AppReportsPath(app string) string {
	return filepath.ToSlash(filepath.Join(AppDir(app), AppReportsDir))
}

// AppRolesPath returns the project-relative role metadata path for an app.
func AppRolesPath(app string) string {
	return filepath.ToSlash(filepath.Join(AppDir(app), AppRolesFile))
}

// EntityDir returns the app-relative directory for a normal Entity bundle.
func EntityDir(entity string) string {
	return filepath.ToSlash(filepath.Join(AppEntitiesDir, entity))
}

// EntityMetadataPath returns the app-relative metadata path for a normal Entity.
func EntityMetadataPath(entity string) string {
	return filepath.ToSlash(filepath.Join(EntityDir(entity), EntityMetadataFile))
}

// EntityFixturesPath returns the app-relative fixture path for a normal Entity.
func EntityFixturesPath(entity string) string {
	return filepath.ToSlash(filepath.Join(EntityDir(entity), EntityFixturesFile))
}

// EntityHooksPath returns the app-relative hook scaffold path for a normal Entity.
func EntityHooksPath(entity string) string {
	return filepath.ToSlash(filepath.Join(EntityDir(entity), EntityHooksFile))
}

// EntityPermissionsPath returns the app-relative permission metadata path for a normal Entity.
func EntityPermissionsPath(entity string) string {
	return filepath.ToSlash(filepath.Join(EntityDir(entity), EntityPermissionsFile))
}

// EntityViewsPath returns the app-relative view metadata path for a normal Entity.
func EntityViewsPath(entity string) string {
	return filepath.ToSlash(filepath.Join(EntityDir(entity), EntityViewsFile))
}

// CollectionMetadataPath returns the app-relative metadata path for a collection row Entity.
func CollectionMetadataPath(collection string) string {
	return filepath.ToSlash(filepath.Join(AppEntitiesDir, CollectionDir, collection+".yml"))
}

// CollectionBundleMetadataPath returns the app-relative bundle-form collection metadata path.
func CollectionBundleMetadataPath(collection string) string {
	return filepath.ToSlash(filepath.Join(AppEntitiesDir, CollectionDir, collection, EntityMetadataFile))
}

// JobMetadataPath returns the app-relative metadata path for a job bundle.
func JobMetadataPath(job string) string {
	return filepath.ToSlash(filepath.Join(AppJobsDir, job, JobMetadataFile))
}

// JobRunPath returns the app-relative Go runner path for a job bundle.
func JobRunPath(job string) string {
	return filepath.ToSlash(filepath.Join(AppJobsDir, job, JobRunFile))
}

// PageMetadataPath returns the app-relative metadata path for a custom page bundle.
func PageMetadataPath(page string) string {
	return filepath.ToSlash(filepath.Join(AppPagesDir, page, PageMetadataFile))
}

// ReportFilePath returns the app-relative single-file report metadata path.
func ReportFilePath(report string) string {
	return filepath.ToSlash(filepath.Join(AppReportsDir, report+".yml"))
}

// ReportMetadataPath returns the app-relative bundle-form report metadata path.
func ReportMetadataPath(report string) string {
	return filepath.ToSlash(filepath.Join(AppReportsDir, report, ReportMetadataFile))
}

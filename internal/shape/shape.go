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
	LocalFilesDir       = ".dygo/files"
	LocalLogsDir        = ".dygo/logs"
	LocalTempDir        = ".dygo/tmp"
	LocalSecretsDir     = ".dygo/secrets"
	LocalSecretKeyFile  = ".dygo/secrets/master.key"
	LocalSecretsTempDir = ".dygo/secrets/tmp"

	AppManifestFile = "app.yml"
	AppEntitiesDir  = "entities"
	AppJobsDir      = "jobs"
	AppPagesDir     = "pages"
	AppReportsDir   = "reports"
	AppRolesFile    = "roles.yml"

	EntityMetadataFile    = "entity.yml"
	EntityFixturesFile    = "fixtures.yml"
	EntityHooksFile       = "hooks.go"
	EntityPermissionsFile = "permissions.yml"
	EntityViewsFile       = "views.yml"

	CollectionDir = "_collections"
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

// CollectionMetadataPath returns the app-relative metadata path for a collection row Entity.
func CollectionMetadataPath(collection string) string {
	return filepath.ToSlash(filepath.Join(AppEntitiesDir, CollectionDir, collection+".yml"))
}

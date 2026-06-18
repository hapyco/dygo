// Package access loads and applies app-owned access metadata.
package access

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hapyco/dygo/internal/app/manifest"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/naming"
	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/yamlmeta"
	"github.com/jackc/pgx/v5"
	"gopkg.in/yaml.v3"
)

// Plan is the loaded access metadata ready for validation or apply.
type Plan struct {
	Roles    []Role
	Policies []PolicyFile
	Grants   []Grant
}

// Role is one role declared in access/_roles.yml.
type Role struct {
	App         string
	Name        string
	Label       string
	Description string
	Path        string
	ProjectPath string
	Line        int
}

// PolicyFile is one Entity access file.
type PolicyFile struct {
	ContributorApp string
	TargetApp      string
	Entity         string
	Path           string
	ProjectPath    string
	Items          []PolicyItem
}

// PolicyItem is one authored grant contribution.
type PolicyItem struct {
	Role     string
	Can      []permissions.Action
	Override bool
	Path     string
	Line     int
}

// Grant is the effective DB permission for one Entity and role.
type Grant struct {
	TargetApp string
	Entity    string
	Role      string
	Can       []permissions.Action
	Source    PolicyItem
}

// Result reports access apply writes.
type Result struct {
	Roles       int
	Permissions int
}

// ExportPlan previews access files generated from live role and permission records.
type ExportPlan struct {
	DestinationApp string
	Target         *shape.AppRef
	Files          []ExportFile
}

// FileCount returns the number of files in the export plan.
func (p ExportPlan) FileCount() int {
	return len(p.Files)
}

// RoleCount returns the number of role items changed by the export plan.
func (p ExportPlan) RoleCount() int {
	count := 0
	for _, file := range p.Files {
		count += file.Roles
	}
	return count
}

// PolicyCount returns the number of policy items changed by the export plan.
func (p ExportPlan) PolicyCount() int {
	count := 0
	for _, file := range p.Files {
		count += file.PolicyItems
	}
	return count
}

// ExportFile is one access metadata file that will be written.
type ExportFile struct {
	Path        string
	ProjectPath string
	Kind        string
	Roles       int
	PolicyItems int
	Content     []byte
}

// ExportResult reports access export writes.
type ExportResult struct {
	FilesWritten       int
	RolesWritten       int
	PolicyItemsWritten int
}

type exportedPermission struct {
	Role Role
	Can  []permissions.Action
}

// BuildPlan loads and validates project access metadata.
func BuildPlan(root string, existingRoles []string) (Plan, error) {
	metadata, err := project.LoadMetadata(root)
	if err != nil {
		return Plan{}, err
	}
	plan, err := Discover(root, metadata)
	if err != nil {
		return Plan{}, err
	}
	if err := Validate(&plan, metadata.Entities, existingRoles); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

// Discover loads access files from all apps.
func Discover(root string, metadata project.Metadata) (Plan, error) {
	var plan Plan
	for _, app := range metadata.Apps {
		appPlan, err := discoverApp(root, app)
		if err != nil {
			return Plan{}, err
		}
		plan.Roles = append(plan.Roles, appPlan.Roles...)
		plan.Policies = append(plan.Policies, appPlan.Policies...)
	}
	sort.SliceStable(plan.Roles, func(i, j int) bool {
		if plan.Roles[i].App == plan.Roles[j].App {
			return plan.Roles[i].Name < plan.Roles[j].Name
		}
		return plan.Roles[i].App < plan.Roles[j].App
	})
	sort.SliceStable(plan.Policies, func(i, j int) bool {
		left := []string{plan.Policies[i].ContributorApp, plan.Policies[i].TargetApp, plan.Policies[i].Entity, plan.Policies[i].ProjectPath}
		right := []string{plan.Policies[j].ContributorApp, plan.Policies[j].TargetApp, plan.Policies[j].Entity, plan.Policies[j].ProjectPath}
		return strings.Join(left, "\x00") < strings.Join(right, "\x00")
	})
	return plan, nil
}

func discoverApp(root string, app manifest.LoadedApp) (Plan, error) {
	var plan Plan
	accessDir := filepath.Join(app.Dir, shape.AppAccessDir)
	entries, err := os.ReadDir(accessDir)
	if errors.Is(err, os.ErrNotExist) {
		return plan, nil
	}
	if err != nil {
		return Plan{}, fmt.Errorf("read access metadata for app %q: %w", app.Manifest.Name, err)
	}
	for _, entry := range entries {
		path := filepath.Join(accessDir, entry.Name())
		if entry.IsDir() {
			files, err := discoverCrossAppPolicies(root, app, path, entry.Name())
			if err != nil {
				return Plan{}, err
			}
			plan.Policies = append(plan.Policies, files...)
			continue
		}
		if entry.Name() == shape.AppRolesFile {
			roles, err := loadRolesFile(root, app.Manifest.Name, path)
			if err != nil {
				return Plan{}, err
			}
			plan.Roles = append(plan.Roles, roles...)
			continue
		}
		if strings.HasSuffix(entry.Name(), ".access.yml") {
			entity := strings.TrimSuffix(entry.Name(), ".access.yml")
			file, err := loadPolicyFile(root, app.Manifest.Name, app.Manifest.Name, entity, path)
			if err != nil {
				return Plan{}, err
			}
			plan.Policies = append(plan.Policies, file)
			continue
		}
		if strings.HasSuffix(entry.Name(), ".yml") {
			return Plan{}, fmt.Errorf("%s is not a valid access file", rel(root, path))
		}
	}
	return plan, nil
}

func discoverCrossAppPolicies(root string, app manifest.LoadedApp, dir string, targetApp string) ([]PolicyFile, error) {
	if err := shape.ValidateMetadataName("access target app", targetApp); err != nil {
		return nil, fmt.Errorf("%s: %w", rel(root, dir), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read cross-app access metadata for app %q: %w", app.Manifest.Name, err)
	}
	var files []PolicyFile
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			return nil, fmt.Errorf("%s is not a valid access file", rel(root, path))
		}
		if !strings.HasSuffix(entry.Name(), ".access.yml") {
			if strings.HasSuffix(entry.Name(), ".yml") {
				return nil, fmt.Errorf("%s is not a valid access file", rel(root, path))
			}
			continue
		}
		entity := strings.TrimSuffix(entry.Name(), ".access.yml")
		file, err := loadPolicyFile(root, app.Manifest.Name, targetApp, entity, path)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func loadRolesFile(root string, appName string, path string) ([]Role, error) {
	node, err := parseYAMLFile(path)
	if err != nil {
		return nil, err
	}
	document := yamlmeta.DocumentMapping(&node)
	if document == nil {
		return nil, fmt.Errorf("%s: role file must be a mapping", rel(root, path))
	}
	var roles []Role
	seenRolesField := false
	for i := 0; i < len(document.Content); i += 2 {
		key := document.Content[i]
		value := document.Content[i+1]
		switch key.Value {
		case "roles":
			seenRolesField = true
			if value.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("%s:%d: roles must be a sequence", rel(root, path), value.Line)
			}
			for _, item := range value.Content {
				role, err := decodeRole(root, appName, path, item)
				if err != nil {
					return nil, err
				}
				roles = append(roles, role)
			}
		default:
			return nil, fmt.Errorf("%s:%d: unknown role file field %q", rel(root, path), key.Line, key.Value)
		}
	}
	if !seenRolesField {
		return nil, fmt.Errorf("%s: roles is required", rel(root, path))
	}
	return roles, nil
}

func decodeRole(root string, appName string, path string, node *yaml.Node) (Role, error) {
	mapping := yamlmeta.ValueMapping(node)
	if mapping == nil {
		return Role{}, fmt.Errorf("%s:%d: role item must be a mapping", rel(root, path), node.Line)
	}
	role := Role{App: appName, Path: path, ProjectPath: rel(root, path), Line: node.Line}
	seen := map[string]bool{}
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		value := mapping.Content[i+1]
		seen[key.Value] = true
		var err error
		switch key.Value {
		case "name":
			role.Name, err = yamlmeta.ScalarString(value, "role name")
		case "label":
			role.Label, err = yamlmeta.ScalarString(value, "role label")
		case "description":
			role.Description, err = yamlmeta.ScalarString(value, "role description")
		default:
			return Role{}, fmt.Errorf("%s:%d: unknown role field %q", rel(root, path), key.Line, key.Value)
		}
		if err != nil {
			return Role{}, fmt.Errorf("%s:%d: %w", rel(root, path), value.Line, err)
		}
	}
	if !seen["name"] || role.Name == "" {
		return Role{}, fmt.Errorf("%s:%d: role name is required", rel(root, path), node.Line)
	}
	if !seen["label"] || role.Label == "" {
		return Role{}, fmt.Errorf("%s:%d: role label is required", rel(root, path), node.Line)
	}
	if err := shape.ValidateMetadataName("role name", role.Name); err != nil {
		return Role{}, fmt.Errorf("%s:%d: %w", rel(root, path), node.Line, err)
	}
	return role, nil
}

func loadPolicyFile(root string, contributorApp string, targetApp string, entity string, path string) (PolicyFile, error) {
	if err := shape.ValidateMetadataName("access entity", entity); err != nil {
		return PolicyFile{}, fmt.Errorf("%s: %w", rel(root, path), err)
	}
	node, err := parseYAMLFile(path)
	if err != nil {
		return PolicyFile{}, err
	}
	document := yamlmeta.DocumentMapping(&node)
	if document == nil {
		return PolicyFile{}, fmt.Errorf("%s: access file must be a mapping", rel(root, path))
	}
	file := PolicyFile{
		ContributorApp: contributorApp,
		TargetApp:      targetApp,
		Entity:         entity,
		Path:           path,
		ProjectPath:    rel(root, path),
	}
	seenPolicy := false
	for i := 0; i < len(document.Content); i += 2 {
		key := document.Content[i]
		value := document.Content[i+1]
		switch key.Value {
		case "policy":
			seenPolicy = true
			if value.Kind != yaml.SequenceNode {
				return PolicyFile{}, fmt.Errorf("%s:%d: policy must be a sequence", rel(root, path), value.Line)
			}
			for _, item := range value.Content {
				policy, err := decodePolicyItem(root, path, item)
				if err != nil {
					return PolicyFile{}, err
				}
				file.Items = append(file.Items, policy)
			}
		default:
			return PolicyFile{}, fmt.Errorf("%s:%d: unknown access field %q", rel(root, path), key.Line, key.Value)
		}
	}
	if !seenPolicy {
		return PolicyFile{}, fmt.Errorf("%s: policy is required", rel(root, path))
	}
	return file, nil
}

func decodePolicyItem(root string, path string, node *yaml.Node) (PolicyItem, error) {
	mapping := yamlmeta.ValueMapping(node)
	if mapping == nil {
		return PolicyItem{}, fmt.Errorf("%s:%d: policy item must be a mapping", rel(root, path), node.Line)
	}
	item := PolicyItem{Path: path, Line: node.Line}
	seen := map[string]bool{}
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		value := mapping.Content[i+1]
		seen[key.Value] = true
		switch key.Value {
		case "role":
			role, err := yamlmeta.ScalarString(value, "policy role")
			if err != nil {
				return PolicyItem{}, fmt.Errorf("%s:%d: %w", rel(root, path), value.Line, err)
			}
			item.Role = role
		case "can":
			actions, err := decodeActions(value)
			if err != nil {
				return PolicyItem{}, fmt.Errorf("%s:%d: %w", rel(root, path), value.Line, err)
			}
			item.Can = actions
		case "override":
			var override bool
			if err := value.Decode(&override); err != nil {
				return PolicyItem{}, fmt.Errorf("%s:%d: override must be a boolean", rel(root, path), value.Line)
			}
			item.Override = override
		default:
			return PolicyItem{}, fmt.Errorf("%s:%d: unknown policy field %q", rel(root, path), key.Line, key.Value)
		}
	}
	if !seen["role"] || item.Role == "" {
		return PolicyItem{}, fmt.Errorf("%s:%d: policy role is required", rel(root, path), node.Line)
	}
	if !seen["can"] {
		return PolicyItem{}, fmt.Errorf("%s:%d: policy can is required", rel(root, path), node.Line)
	}
	if err := shape.ValidateMetadataName("policy role", item.Role); err != nil {
		return PolicyItem{}, fmt.Errorf("%s:%d: %w", rel(root, path), node.Line, err)
	}
	return item, nil
}

func decodeActions(node *yaml.Node) ([]permissions.Action, error) {
	values, err := yamlmeta.ScalarStringSequence(node, "policy can")
	if err != nil {
		return nil, err
	}
	seen := map[permissions.Action]bool{}
	actions := make([]permissions.Action, 0, len(values))
	for _, value := range values {
		action, err := permissions.ParseAction(value)
		if err != nil {
			return nil, err
		}
		if seen[action] {
			return nil, fmt.Errorf("policy can contains duplicate action %q", action)
		}
		seen[action] = true
		actions = append(actions, action)
	}
	return actions, nil
}

func parseYAMLFile(path string) (yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return yaml.Node{}, err
	}
	node, err := yamlmeta.Parse(data, "parse access file "+path)
	if err != nil {
		return yaml.Node{}, err
	}
	if err := yamlmeta.RejectDuplicateKeys(&node, func(duplicate yamlmeta.DuplicateKey) error {
		return fmt.Errorf("duplicate access key %q at %s line %d, previously defined at line %d", duplicate.Key, strings.TrimSuffix(duplicate.Location, "."+duplicate.Key), duplicate.Line, duplicate.PreviousLine)
	}); err != nil {
		return yaml.Node{}, fmt.Errorf("%s: %w", path, err)
	}
	return node, nil
}

// Validate checks role references, Entity targets, and duplicate policy resolution.
func Validate(plan *Plan, entities []catalog.LoadedEntity, existingRoles []string) error {
	knownRoles := map[string]bool{}
	roleByName := map[string]Role{}
	for _, role := range plan.Roles {
		if previous, exists := roleByName[role.Name]; exists {
			return fmt.Errorf("%s:%d: role %q duplicates role from %s:%d", role.ProjectPath, role.Line, role.Name, previous.ProjectPath, previous.Line)
		}
		roleByName[role.Name] = role
		knownRoles[role.Name] = true
	}
	for _, role := range existingRoles {
		knownRoles[role] = true
	}

	entityIndex := map[string]bool{}
	for _, entity := range entities {
		entityIndex[catalog.EntityKey(entity.AppName, entity.Entity.Name)] = true
	}

	groups := map[string][]groupedPolicy{}
	for _, file := range plan.Policies {
		if !entityIndex[catalog.EntityKey(file.TargetApp, file.Entity)] {
			return fmt.Errorf("%s: access target Entity %s/%s is not loaded", file.ProjectPath, file.TargetApp, file.Entity)
		}
		for _, item := range file.Items {
			if !knownRoles[item.Role] {
				return fmt.Errorf("%s:%d: policy references unknown role %q", file.ProjectPath, item.Line, item.Role)
			}
			key := policyKey(file.TargetApp, file.Entity, item.Role)
			groups[key] = append(groups[key], groupedPolicy{file: file, item: item})
		}
	}

	plan.Grants = plan.Grants[:0]
	for _, group := range groups {
		grant, err := resolvePolicyGroup(group)
		if err != nil {
			return err
		}
		plan.Grants = append(plan.Grants, grant)
	}
	sort.SliceStable(plan.Grants, func(i, j int) bool {
		left := []string{plan.Grants[i].TargetApp, plan.Grants[i].Entity, plan.Grants[i].Role}
		right := []string{plan.Grants[j].TargetApp, plan.Grants[j].Entity, plan.Grants[j].Role}
		return strings.Join(left, "\x00") < strings.Join(right, "\x00")
	})
	return nil
}

type groupedPolicy struct {
	file PolicyFile
	item PolicyItem
}

func resolvePolicyGroup(group []groupedPolicy) (Grant, error) {
	if len(group) == 0 {
		return Grant{}, fmt.Errorf("empty policy group")
	}
	effective := group[0]
	if len(group) > 1 {
		overrideCount := 0
		for _, item := range group {
			if item.item.Override {
				overrideCount++
				effective = item
			}
		}
		if overrideCount != 1 {
			first := group[0]
			return Grant{}, fmt.Errorf("%s:%d: duplicate policy for %s/%s role %q; add override: true to the intended replacement", first.file.ProjectPath, first.item.Line, first.file.TargetApp, first.file.Entity, first.item.Role)
		}
	}
	return Grant{
		TargetApp: effective.file.TargetApp,
		Entity:    effective.file.Entity,
		Role:      effective.item.Role,
		Can:       effective.item.Can,
		Source:    effective.item,
	}, nil
}

func policyKey(appName string, entity string, role string) string {
	return appName + "\x00" + entity + "\x00" + role
}

// Apply loads access metadata and writes effective Core role and permission records.
func Apply(ctx context.Context, root string, databaseURL string) (Result, error) {
	pool, err := db.OpenRuntimePool(ctx, databaseURL)
	if err != nil {
		return Result{}, err
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("begin access transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	existingRoles, err := listRoleNames(ctx, tx)
	if err != nil {
		return Result{}, err
	}
	plan, err := BuildPlan(root, existingRoles)
	if err != nil {
		return Result{}, err
	}
	if err := applyPlan(ctx, tx, plan); err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("commit access transaction: %w", err)
	}
	return Result{Roles: len(plan.Roles), Permissions: len(plan.Grants)}, nil
}

// ApplyPlan loads access metadata using database roles as known role references.
func ApplyPlan(ctx context.Context, root string, databaseURL string) (Plan, error) {
	pool, err := db.OpenRuntimePool(ctx, databaseURL)
	if err != nil {
		return Plan{}, err
	}
	defer pool.Close()
	existingRoles, err := listRoleNames(ctx, pool)
	if err != nil {
		return Plan{}, err
	}
	return BuildPlan(root, existingRoles)
}

// PlanExport builds a file export plan from live role and permission records.
func PlanExport(ctx context.Context, root string, databaseURL string, target *shape.AppRef, destinationApp string) (ExportPlan, error) {
	if strings.TrimSpace(destinationApp) == "" {
		return ExportPlan{}, fmt.Errorf("destination app is required")
	}
	if err := shape.ValidateMetadataName("destination app", destinationApp); err != nil {
		return ExportPlan{}, err
	}
	metadata, err := project.LoadMetadata(root)
	if err != nil {
		return ExportPlan{}, err
	}
	if !metadataHasApp(metadata, destinationApp) {
		return ExportPlan{}, fmt.Errorf("destination app %q is not loaded", destinationApp)
	}
	if target != nil && !metadataHasEntity(metadata, *target) {
		return ExportPlan{}, fmt.Errorf("Entity target %s/%s is not loaded", target.App, target.Name)
	}

	discovered, err := Discover(root, metadata)
	if err != nil {
		return ExportPlan{}, err
	}
	pool, err := db.OpenRuntimePool(ctx, databaseURL)
	if err != nil {
		return ExportPlan{}, fmt.Errorf("open access export database: %w", err)
	}
	defer pool.Close()

	roles, err := listDatabaseRoles(ctx, pool)
	if err != nil {
		return ExportPlan{}, err
	}
	roleByName := rolesByName(roles)
	var permissions []exportedPermission
	if target != nil {
		permissions, err = listDatabasePermissions(ctx, pool, *target)
		if err != nil {
			return ExportPlan{}, err
		}
	}

	files := []ExportFile{}
	roleFile, err := planRoleExport(root, destinationApp, discovered, roleByName, exportRoleNames(roles, permissions, target != nil))
	if err != nil {
		return ExportPlan{}, err
	}
	if roleFile != nil {
		files = append(files, *roleFile)
	}
	if target != nil {
		policyFile, err := planPolicyExport(root, destinationApp, *target, permissions)
		if err != nil {
			return ExportPlan{}, err
		}
		if policyFile != nil {
			files = append(files, *policyFile)
		}
	}
	return ExportPlan{DestinationApp: destinationApp, Target: target, Files: files}, nil
}

// WriteExportPlan writes the access files described by plan.
func WriteExportPlan(plan ExportPlan) (ExportResult, error) {
	result := ExportResult{}
	for _, file := range plan.Files {
		if len(file.Content) == 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return ExportResult{}, fmt.Errorf("create access directory %s: %w", filepath.Dir(file.Path), err)
		}
		if err := os.WriteFile(file.Path, file.Content, 0o644); err != nil {
			return ExportResult{}, fmt.Errorf("write access file %s: %w", file.Path, err)
		}
		result.FilesWritten++
		result.RolesWritten += file.Roles
		result.PolicyItemsWritten += file.PolicyItems
	}
	return result, nil
}

func applyPlan(ctx context.Context, tx pgx.Tx, plan Plan) error {
	roleIDs, err := upsertRoles(ctx, tx, plan.Roles)
	if err != nil {
		return err
	}
	for _, grant := range plan.Grants {
		roleID, ok := roleIDs[grant.Role]
		if !ok {
			roleID, err = roleIDByName(ctx, tx, grant.Role)
			if err != nil {
				return err
			}
			roleIDs[grant.Role] = roleID
		}
		entityID, err := entityID(ctx, tx, grant.TargetApp, grant.Entity)
		if err != nil {
			return err
		}
		if err := upsertPermission(ctx, tx, entityID, roleID, grant); err != nil {
			return err
		}
	}
	return nil
}

type roleNameQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func listRoleNames(ctx context.Context, queryer roleNameQueryer) ([]string, error) {
	rows, err := queryer.Query(ctx, `SELECT name FROM "role"`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func listDatabaseRoles(ctx context.Context, queryer roleNameQueryer) ([]Role, error) {
	rows, err := queryer.Query(ctx, `SELECT name, label, description FROM "role" ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	defer rows.Close()
	var roles []Role
	for rows.Next() {
		var role Role
		var description sql.NullString
		if err := rows.Scan(&role.Name, &role.Label, &description); err != nil {
			return nil, err
		}
		if description.Valid {
			role.Description = description.String
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func listDatabasePermissions(ctx context.Context, queryer roleNameQueryer, target shape.AppRef) ([]exportedPermission, error) {
	rows, err := queryer.Query(ctx, `
SELECT r.name, r.label, r.description,
	p."read", p."create", p."update", p."delete", p."export", p."print"
FROM "permission" p
JOIN "role" r ON r.id = p.role_id
JOIN entity e ON e.id = p.entity_id
JOIN app a ON a.id = e.app_id
WHERE a.name = $1
	AND e.key = $2
	AND COALESCE(p.retired, false) = false
ORDER BY r.name`, target.App, target.Name)
	if err != nil {
		return nil, fmt.Errorf("list permissions for %s/%s: %w", target.App, target.Name, err)
	}
	defer rows.Close()
	var exported []exportedPermission
	for rows.Next() {
		var item exportedPermission
		var description sql.NullString
		var read, create, update, deleteAction, export, print bool
		if err := rows.Scan(&item.Role.Name, &item.Role.Label, &description, &read, &create, &update, &deleteAction, &export, &print); err != nil {
			return nil, err
		}
		if description.Valid {
			item.Role.Description = description.String
		}
		item.Can = actionsFromValues(map[permissions.Action]bool{
			permissions.ActionRead:   read,
			permissions.ActionCreate: create,
			permissions.ActionUpdate: update,
			permissions.ActionDelete: deleteAction,
			permissions.ActionExport: export,
			permissions.ActionPrint:  print,
		})
		exported = append(exported, item)
	}
	return exported, rows.Err()
}

func planRoleExport(root string, destinationApp string, discovered Plan, dbRoles map[string]Role, roleNames []string) (*ExportFile, error) {
	if len(roleNames) == 0 {
		return nil, nil
	}
	represented := map[string]string{}
	for _, role := range discovered.Roles {
		represented[role.Name] = role.App
	}
	path := filepath.Join(root, filepath.FromSlash(shape.AppRolesPath(destinationApp)))
	projectPath := filepath.ToSlash(shape.AppRolesPath(destinationApp))
	existingRoles, err := existingRolesForExport(root, destinationApp, path)
	if err != nil {
		return nil, err
	}
	before, err := readOptionalFile(path)
	if err != nil {
		return nil, err
	}
	changedRoles, changedCount := upsertExportRoles(existingRoles, dbRoles, represented, destinationApp, roleNames)
	if changedCount == 0 {
		return nil, nil
	}
	content, err := encodeRolesFile(changedRoles)
	if err != nil {
		return nil, fmt.Errorf("encode roles export %s: %w", projectPath, err)
	}
	if bytes.Equal(before, content) {
		return nil, nil
	}
	return &ExportFile{Path: path, ProjectPath: projectPath, Kind: "roles", Roles: changedCount, Content: content}, nil
}

func planPolicyExport(root string, destinationApp string, target shape.AppRef, permissions []exportedPermission) (*ExportFile, error) {
	if len(permissions) == 0 {
		return nil, nil
	}
	path, projectPath := policyExportPath(root, destinationApp, target)
	existing, err := existingPolicyForExport(root, destinationApp, target, path)
	if err != nil {
		return nil, err
	}
	before, err := readOptionalFile(path)
	if err != nil {
		return nil, err
	}
	items, changedCount := upsertExportPolicyItems(existing.Items, permissions)
	if changedCount == 0 {
		return nil, nil
	}
	content, err := encodePolicyFile(items)
	if err != nil {
		return nil, fmt.Errorf("encode policy export %s: %w", projectPath, err)
	}
	if bytes.Equal(before, content) {
		return nil, nil
	}
	return &ExportFile{Path: path, ProjectPath: projectPath, Kind: "policy", PolicyItems: changedCount, Content: content}, nil
}

func existingRolesForExport(root string, destinationApp string, path string) ([]Role, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat roles file %s: %w", rel(root, path), err)
	}
	return loadRolesFile(root, destinationApp, path)
}

func existingPolicyForExport(root string, destinationApp string, target shape.AppRef, path string) (PolicyFile, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return PolicyFile{ContributorApp: destinationApp, TargetApp: target.App, Entity: target.Name, Path: path, ProjectPath: rel(root, path)}, nil
	} else if err != nil {
		return PolicyFile{}, fmt.Errorf("stat access file %s: %w", rel(root, path), err)
	}
	return loadPolicyFile(root, destinationApp, target.App, target.Name, path)
}

func upsertExportRoles(existing []Role, dbRoles map[string]Role, represented map[string]string, destinationApp string, roleNames []string) ([]Role, int) {
	roles := append([]Role(nil), existing...)
	index := map[string]int{}
	for i, role := range roles {
		if _, exists := index[role.Name]; !exists {
			index[role.Name] = i
		}
	}
	changed := 0
	for _, name := range sortedUniqueStrings(roleNames) {
		role, ok := dbRoles[name]
		if !ok {
			continue
		}
		if i, exists := index[name]; exists {
			role.App = destinationApp
			if roles[i].Label != role.Label || roles[i].Description != role.Description {
				roles[i].Label = role.Label
				roles[i].Description = role.Description
				changed++
			}
			continue
		}
		if owner, exists := represented[name]; exists && owner != destinationApp {
			continue
		}
		role.App = destinationApp
		roles = append(roles, role)
		index[name] = len(roles) - 1
		changed++
	}
	return roles, changed
}

func upsertExportPolicyItems(existing []PolicyItem, exported []exportedPermission) ([]PolicyItem, int) {
	items := append([]PolicyItem(nil), existing...)
	index := map[string]int{}
	for i, item := range items {
		if _, exists := index[item.Role]; !exists {
			index[item.Role] = i
		}
	}
	changed := 0
	for _, permission := range exported {
		if i, exists := index[permission.Role.Name]; exists {
			if !sameActions(items[i].Can, permission.Can) {
				items[i].Can = append([]permissions.Action(nil), permission.Can...)
				changed++
			}
			continue
		}
		items = append(items, PolicyItem{Role: permission.Role.Name, Can: append([]permissions.Action(nil), permission.Can...)})
		index[permission.Role.Name] = len(items) - 1
		changed++
	}
	return items, changed
}

func readOptionalFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func encodeRolesFile(roles []Role) ([]byte, error) {
	items := make([]*yaml.Node, 0, len(roles))
	for _, role := range roles {
		content := []*yaml.Node{
			yamlStringNode("name"), yamlStringNode(role.Name),
			yamlStringNode("label"), yamlStringNode(role.Label),
		}
		if strings.TrimSpace(role.Description) != "" {
			content = append(content, yamlStringNode("description"), yamlStringNode(role.Description))
		}
		items = append(items, &yaml.Node{Kind: yaml.MappingNode, Content: content})
	}
	return encodeDocument(yamlStringNode("roles"), &yaml.Node{Kind: yaml.SequenceNode, Content: items})
}

func encodePolicyFile(items []PolicyItem) ([]byte, error) {
	nodes := make([]*yaml.Node, 0, len(items))
	for _, item := range items {
		content := []*yaml.Node{
			yamlStringNode("role"), yamlStringNode(item.Role),
			yamlStringNode("can"), yamlActionSequenceNode(item.Can),
		}
		if item.Override {
			content = append(content, yamlStringNode("override"), yamlBoolNode(true))
		}
		nodes = append(nodes, &yaml.Node{Kind: yaml.MappingNode, Content: content})
	}
	return encodeDocument(yamlStringNode("policy"), &yaml.Node{Kind: yaml.SequenceNode, Content: nodes})
}

func encodeDocument(key *yaml.Node, value *yaml.Node) ([]byte, error) {
	document := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			key, value,
		},
	}
	var buf bytes.Buffer
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

func yamlStringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func yamlBoolNode(value bool) *yaml.Node {
	if value {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
}

func yamlActionSequenceNode(actions []permissions.Action) *yaml.Node {
	nodes := make([]*yaml.Node, 0, len(actions))
	for _, action := range actions {
		nodes = append(nodes, yamlStringNode(string(action)))
	}
	return &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle, Content: nodes}
}

func actionsFromValues(values map[permissions.Action]bool) []permissions.Action {
	actions := []permissions.Action{}
	for _, action := range permissions.SupportedActions() {
		if values[action] {
			actions = append(actions, action)
		}
	}
	return actions
}

func sameActions(left []permissions.Action, right []permissions.Action) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func exportRoleNames(roles []Role, exported []exportedPermission, policyOnly bool) []string {
	if policyOnly {
		names := make([]string, 0, len(exported))
		for _, item := range exported {
			names = append(names, item.Role.Name)
		}
		return names
	}
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	return names
}

func rolesByName(roles []Role) map[string]Role {
	index := map[string]Role{}
	for _, role := range roles {
		index[role.Name] = role
	}
	return index
}

func sortedUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	unique := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

func metadataHasApp(metadata project.Metadata, appName string) bool {
	for _, app := range metadata.Apps {
		if app.Manifest.Name == appName {
			return true
		}
	}
	return false
}

func metadataHasEntity(metadata project.Metadata, target shape.AppRef) bool {
	for _, entity := range metadata.Entities {
		if entity.AppName == target.App && entity.Entity.Name == target.Name {
			return true
		}
	}
	return false
}

func policyExportPath(root string, destinationApp string, target shape.AppRef) (string, string) {
	parts := []string{shape.AppDir(destinationApp), shape.AppAccessDir}
	if target.App != destinationApp {
		parts = append(parts, target.App)
	}
	parts = append(parts, target.Name+".access.yml")
	projectPath := filepath.ToSlash(filepath.Join(parts...))
	return filepath.Join(root, filepath.FromSlash(projectPath)), projectPath
}

func upsertRoles(ctx context.Context, tx pgx.Tx, roles []Role) (map[string]int64, error) {
	ids := map[string]int64{}
	for _, role := range roles {
		var id int64
		if err := tx.QueryRow(ctx, `
INSERT INTO "role" (name, label, description, enabled)
VALUES ($1, $2, $3, true)
ON CONFLICT (name) DO UPDATE
SET label = EXCLUDED.label,
	description = EXCLUDED.description,
	updated_at = now()
RETURNING id`, role.Name, role.Label, nullIfEmpty(role.Description)).Scan(&id); err != nil {
			return nil, fmt.Errorf("upsert role %q: %w", role.Name, err)
		}
		ids[role.Name] = id
	}
	return ids, nil
}

func roleIDByName(ctx context.Context, tx pgx.Tx, role string) (int64, error) {
	var id int64
	if err := tx.QueryRow(ctx, `SELECT id FROM "role" WHERE name = $1`, role).Scan(&id); err != nil {
		return 0, fmt.Errorf("load role %q: %w", role, err)
	}
	return id, nil
}

func entityID(ctx context.Context, tx pgx.Tx, appName string, entity string) (int64, error) {
	var id int64
	if err := tx.QueryRow(ctx, `
SELECT e.id
FROM entity e
JOIN app a ON a.id = e.app_id
WHERE a.name = $1 AND e.key = $2`, appName, entity).Scan(&id); err != nil {
		return 0, fmt.Errorf("load Entity %s/%s: %w", appName, entity, err)
	}
	return id, nil
}

func upsertPermission(ctx context.Context, tx pgx.Tx, entityID int64, roleID int64, grant Grant) error {
	values := actionValues(grant.Can)
	name, err := naming.Random(16)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO "permission" (name, entity_id, role_id, "read", "create", "update", "delete", "export", "print", retired)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, false)
ON CONFLICT (entity_id, role_id) DO UPDATE
SET "read" = EXCLUDED."read",
	"create" = EXCLUDED."create",
	"update" = EXCLUDED."update",
	"delete" = EXCLUDED."delete",
	"export" = EXCLUDED."export",
	"print" = EXCLUDED."print",
	retired = false,
	updated_at = now()`, name, entityID, roleID, values[permissions.ActionRead], values[permissions.ActionCreate], values[permissions.ActionUpdate], values[permissions.ActionDelete], values[permissions.ActionExport], values[permissions.ActionPrint])
	if err != nil {
		return fmt.Errorf("upsert permission %s/%s role %q: %w", grant.TargetApp, grant.Entity, grant.Role, err)
	}
	return nil
}

func actionValues(actions []permissions.Action) map[permissions.Action]bool {
	values := map[permissions.Action]bool{}
	for _, action := range actions {
		values[action] = true
	}
	return values
}

func nullIfEmpty(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func rel(root string, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

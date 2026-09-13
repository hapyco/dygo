package permissions

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/hapyco/dygo/internal/accesspolicy"
	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
)

// Scope is the row and field predicate for one actor, Entity, and action.
// SQL uses PostgreSQL placeholders starting at $1 and the _dygo_record root alias.
type Scope struct {
	Where      string
	Args       []any
	FieldRead  map[string]string
	FieldWrite map[string]string
	Roles      []string
	RoleWhere  map[string]string
}

type scopeGrant struct {
	role   string
	when   *accesspolicy.When
	fields accesspolicy.Fields
}

type scopeQueryer interface {
	db.MetadataQueryer
}

// RecordScope compiles all matching role grants into one permission-aware SQL scope.
func (c Checker) RecordScope(ctx context.Context, request Request) (Scope, error) {
	normalized, resource, err := normalizeRequest(request)
	if err != nil {
		return Scope{}, err
	}
	if resource.Kind != ResourceEntity {
		return Scope{}, permissionError(ErrorInvalidRequest, "record scope requires an Entity resource", decisionDetails(normalized), nil)
	}
	if normalized.Actor.Administrator {
		return Scope{Where: "TRUE", FieldRead: map[string]string{}, FieldWrite: map[string]string{}}, nil
	}
	queryer, ok := c.queryer.(scopeQueryer)
	if !ok {
		return Scope{}, permissionError(ErrorInternal, "permission scope queryer is required", decisionDetails(normalized), nil)
	}
	grants, err := loadScopeGrants(ctx, queryer, normalized, resource)
	if err != nil {
		return Scope{}, err
	}
	if len(grants) == 0 {
		return Scope{Where: "FALSE", FieldRead: map[string]string{}, FieldWrite: map[string]string{}}, nil
	}
	compiler := scopeCompiler{ctx: ctx, metadata: db.NewMetadataReader(queryer), rootApp: resource.App, rootEntity: resource.Name, actorID: normalized.Actor.UserID}
	return compiler.compile(grants)
}

func loadScopeGrants(ctx context.Context, queryer scopeQueryer, request Request, resource Resource) ([]scopeGrant, error) {
	target, targetArgs := permissionTargetSQL(resource)
	args := append([]any{request.Actor.UserID}, targetArgs...)
	action, args, err := permissionActionSQL(request.Action, resource, args)
	if err != nil {
		return nil, err
	}
	rows, err := queryer.Query(ctx, fmt.Sprintf(`
SELECT r.name, p."when", COALESCE(p.field_rules, '{}'::jsonb)
FROM "user" u
JOIN user_role ur ON ur.user_id = u.id
JOIN "role" r ON r.id = ur.role_id AND COALESCE(r.enabled, false) = true
JOIN "permission" p ON p.role_id = r.id
JOIN entity e ON e.id = p.entity_id AND e.retired = false
JOIN app a ON a.id = e.app_id
WHERE u.id = $1
	AND COALESCE(u.enabled, false) = true
	AND %s
	AND COALESCE(p.retired, false) = false
	AND %s
ORDER BY r.name`, target, action), args...)
	if err != nil {
		return nil, permissionError(ErrorInternal, "permission scope query failed", decisionDetails(request), err)
	}
	defer rows.Close()
	var grants []scopeGrant
	for rows.Next() {
		var grant scopeGrant
		var whenJSON, fieldsJSON []byte
		if err := rows.Scan(&grant.role, &whenJSON, &fieldsJSON); err != nil {
			return nil, permissionError(ErrorInternal, "permission scope read failed", decisionDetails(request), err)
		}
		if len(whenJSON) > 0 && string(whenJSON) != "null" {
			grant.when = &accesspolicy.When{}
			if err := json.Unmarshal(whenJSON, grant.when); err != nil {
				return nil, permissionError(ErrorInternal, "permission row condition is invalid", decisionDetails(request), err)
			}
		}
		if err := json.Unmarshal(fieldsJSON, &grant.fields); err != nil {
			return nil, permissionError(ErrorInternal, "permission field rules are invalid", decisionDetails(request), err)
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, permissionError(ErrorInternal, "permission scope read failed", decisionDetails(request), err)
	}
	return grants, nil
}

type scopeCompiler struct {
	ctx        context.Context
	metadata   db.MetadataReader
	rootApp    string
	rootEntity string
	actorID    int64
	args       []any
	alias      int
	cache      map[string]db.MetadataEntityMeta
}

func (c *scopeCompiler) compile(grants []scopeGrant) (Scope, error) {
	root, err := c.entity(c.rootApp, c.rootEntity)
	if err != nil {
		return Scope{}, err
	}
	predicates := make([]string, 0, len(grants))
	grantPredicates := map[string]string{}
	roles := make([]string, 0, len(grants))
	fieldNames := map[string]bool{}
	for _, grant := range grants {
		predicate, err := c.when(root, grant.when)
		if err != nil {
			return Scope{}, permissionError(ErrorInternal, "permission row condition could not be compiled", map[string]any{"role": grant.role, "entity": c.rootEntity}, err)
		}
		predicates = append(predicates, predicate)
		grantPredicates[grant.role] = predicate
		roles = append(roles, grant.role)
		for _, field := range append(append([]string{}, grant.fields.DenyRead...), grant.fields.DenyWrite...) {
			fieldNames[field] = true
		}
	}
	fieldRead := map[string]string{}
	fieldWrite := map[string]string{}
	for field := range fieldNames {
		var read, write []string
		for _, grant := range grants {
			predicate := grantPredicates[grant.role]
			if !slices.Contains(grant.fields.DenyRead, field) {
				read = append(read, predicate)
			}
			if !slices.Contains(grant.fields.DenyRead, field) && !slices.Contains(grant.fields.DenyWrite, field) {
				write = append(write, predicate)
			}
		}
		fieldRead[field] = orPredicate(read)
		fieldWrite[field] = orPredicate(write)
	}
	sort.Strings(roles)
	return Scope{Where: orPredicate(predicates), Args: c.args, FieldRead: fieldRead, FieldWrite: fieldWrite, Roles: roles, RoleWhere: grantPredicates}, nil
}

func (c *scopeCompiler) when(root db.MetadataEntityMeta, when *accesspolicy.When) (string, error) {
	if when == nil {
		return "TRUE", nil
	}
	parts := make([]string, 0, len(when.Conditions))
	for _, condition := range when.Conditions {
		expression, _, _, err := c.path(root, "_dygo_record", condition.Field)
		if err != nil {
			return "", err
		}
		switch {
		case condition.Equals == "actor.user":
			parts = append(parts, expression+" = "+c.arg(c.actorID))
		case condition.In != nil:
			membership, err := c.membership(root, expression, *condition.In)
			if err != nil {
				return "", err
			}
			parts = append(parts, membership)
		default:
			return "", fmt.Errorf("unsupported condition for %s", condition.Field)
		}
	}
	if when.Match == "any" {
		return orPredicate(parts), nil
	}
	return andPredicate(parts), nil
}

func (c *scopeCompiler) membership(root db.MetadataEntityMeta, recordValue string, membership accesspolicy.Membership) (string, error) {
	assignment, err := c.entity(root.App.Name, membership.Entity)
	if err != nil {
		return "", err
	}
	alias := c.nextAlias()
	value, _, _, err := c.path(assignment, alias, "record."+membership.Value)
	if err != nil {
		return "", err
	}
	clauses := []string{value + " = " + recordValue}
	keys := make([]string, 0, len(membership.Where))
	for field := range membership.Where {
		keys = append(keys, field)
	}
	sort.Strings(keys)
	for _, field := range keys {
		if membership.Where[field] != "actor.user" {
			return "", fmt.Errorf("unsupported membership value")
		}
		value, _, _, err := c.path(assignment, alias, "record."+field)
		if err != nil {
			return "", err
		}
		clauses = append(clauses, value+" = "+c.arg(c.actorID))
	}
	if _, ok := db.RecordAddressableFieldByName(db.MetadataFieldsByName(assignment), "retired"); ok {
		clauses = append(clauses, quote(alias)+"."+quote("retired")+" = false")
	}
	return fmt.Sprintf("EXISTS (SELECT 1 FROM %s AS %s WHERE %s)", quote(db.EntityStorageTableName(assignment.App.Name, assignment.Key)), quote(alias), strings.Join(clauses, " AND ")), nil
}

func (c *scopeCompiler) path(root db.MetadataEntityMeta, rootAlias string, raw string) (string, db.MetadataEntityMeta, db.MetadataField, error) {
	parts := strings.Split(strings.TrimPrefix(raw, "record."), ".")
	if len(parts) == 0 || parts[0] == "" {
		return "", db.MetadataEntityMeta{}, db.MetadataField{}, fmt.Errorf("record field path is required")
	}
	current, alias := root, rootAlias
	joins := []string{}
	where := ""
	for index, name := range parts {
		if name == "id" {
			if index != len(parts)-1 {
				return "", db.MetadataEntityMeta{}, db.MetadataField{}, fmt.Errorf("id must end a field path")
			}
			return quote(alias) + "." + quote("id"), current, db.MetadataField{Name: "id", Type: "bigint", Stored: true}, nil
		}
		if name == "owner" {
			if index != len(parts)-1 {
				return "", db.MetadataEntityMeta{}, db.MetadataField{}, fmt.Errorf("owner must end a field path")
			}
			return quote(alias) + "." + quote("owner_id"), current, db.MetadataField{Name: "owner", Type: "link"}, nil
		}
		field, ok := db.RecordAddressableFieldByName(db.MetadataFieldsByName(current), name)
		if !ok || !db.MetadataFieldStored(field) {
			return "", db.MetadataEntityMeta{}, db.MetadataField{}, fmt.Errorf("field %q does not exist on %s/%s", name, current.App.Name, current.Key)
		}
		column := storageColumn(field)
		if index == len(parts)-1 {
			expression := quote(alias) + "." + quote(column)
			if len(joins) == 0 {
				return expression, current, field, nil
			}
			return fmt.Sprintf("(SELECT %s FROM %s WHERE %s LIMIT 1)", expression, strings.Join(joins, " "), where), current, field, nil
		}
		if field.Type != "link" {
			return "", db.MetadataEntityMeta{}, db.MetadataField{}, fmt.Errorf("field %q is not a link", name)
		}
		target, err := db.LinkFieldTargetIdentity(field, current.App.Name)
		if err != nil {
			return "", db.MetadataEntityMeta{}, db.MetadataField{}, err
		}
		next, err := c.entity(target.App, target.Entity)
		if err != nil {
			return "", db.MetadataEntityMeta{}, db.MetadataField{}, err
		}
		nextAlias := c.nextAlias()
		table := quote(db.EntityStorageTableName(next.App.Name, next.Key)) + " AS " + quote(nextAlias)
		if len(joins) == 0 {
			joins = append(joins, table)
			where = quote(nextAlias) + "." + quote("id") + " = " + quote(alias) + "." + quote(column)
		} else {
			joins = append(joins, "JOIN "+table+" ON "+quote(nextAlias)+"."+quote("id")+" = "+quote(alias)+"."+quote(column))
		}
		current, alias = next, nextAlias
	}
	return "", db.MetadataEntityMeta{}, db.MetadataField{}, fmt.Errorf("invalid field path")
}

func (c *scopeCompiler) entity(app, entity string) (db.MetadataEntityMeta, error) {
	if c.cache == nil {
		c.cache = map[string]db.MetadataEntityMeta{}
	}
	key := app + "/" + entity
	if meta, ok := c.cache[key]; ok {
		return meta, nil
	}
	meta, err := c.metadata.GetEntityMetaByIdentity(c.ctx, app, entity)
	if err != nil {
		return db.MetadataEntityMeta{}, err
	}
	c.cache[key] = meta
	return meta, nil
}

func (c *scopeCompiler) arg(value any) string {
	c.args = append(c.args, value)
	return fmt.Sprintf("$%d", len(c.args))
}

func (c *scopeCompiler) nextAlias() string {
	c.alias++
	return fmt.Sprintf("_dygo_access_%d", c.alias)
}

func storageColumn(field db.MetadataField) string {
	column := strings.ReplaceAll(field.Name, "-", "_")
	if definition, ok := fieldtype.DefaultDefinition(field.Type); ok {
		column += definition.Behavior.ColumnSuffix
	}
	return column
}

func quote(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }

func orPredicate(parts []string) string {
	if len(parts) == 0 {
		return "FALSE"
	}
	return "(" + strings.Join(parts, ") OR (") + ")"
}

func andPredicate(parts []string) string {
	if len(parts) == 0 {
		return "TRUE"
	}
	return "(" + strings.Join(parts, ") AND (") + ")"
}

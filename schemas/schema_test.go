package schemas

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/entity/fieldtype"
	entityschema "github.com/hapyco/dygo/internal/entity/schema"
	"github.com/hapyco/dygo/internal/patches"
)

func TestSchemasAreValidJSON(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(schemas) error = %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		data, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", entry.Name(), err)
		}
		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("%s is invalid JSON: %v", entry.Name(), err)
		}
		for _, key := range []string{"$schema", "$id", "title"} {
			value, ok := schema[key].(string)
			if !ok || strings.TrimSpace(value) == "" {
				t.Fatalf("%s missing non-empty %s", entry.Name(), key)
			}
		}
		if value, ok := schema["type"].(string); !ok || value != "object" {
			t.Fatalf("%s type = %#v, want object", entry.Name(), schema["type"])
		}
	}
}

func TestVSCodeSettingsReferenceExistingSchemas(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".vscode", "settings.json"))
	if err != nil {
		t.Fatalf("ReadFile(.vscode/settings.json) error = %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf(".vscode/settings.json is invalid JSON: %v", err)
	}
	rawSchemas, ok := settings["yaml.schemas"].(map[string]any)
	if !ok {
		t.Fatal(".vscode/settings.json missing yaml.schemas object")
	}
	for schemaPath, rawMatches := range rawSchemas {
		cleanSchemaPath := strings.TrimPrefix(schemaPath, "./")
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(cleanSchemaPath))); err != nil {
			t.Fatalf("schema mapping %q does not reference an existing file: %v", schemaPath, err)
		}
		matches, ok := rawMatches.([]any)
		if !ok || len(matches) == 0 {
			t.Fatalf("schema mapping %q has no file matches", schemaPath)
		}
		for _, rawMatch := range matches {
			match, ok := rawMatch.(string)
			if !ok || strings.TrimSpace(match) == "" {
				t.Fatalf("schema mapping %q has invalid match %#v", schemaPath, rawMatch)
			}
		}
	}
}

func TestEntitySchemaContractsMatchRuntime(t *testing.T) {
	schema := readJSONSchema(t, "entity.schema.json")
	defs := schemaObject(t, schema, "$defs")

	assertSameStringSet(t, schemaStringEnum(t, schemaObject(t, defs, "fieldType"), "enum"), fieldtype.DefaultRegistry().Names())
	assertSameStringSet(t, schemaNamingStrategies(t, schemaObject(t, defs, "name")), entityschema.SupportedNamingStrategies())
	assertSameStringSet(t, schemaCheckOperators(t, schemaObject(t, defs, "check")), entityschema.SupportedCheckOperators())
	assertSameStringSet(t, schemaConstraintTypes(t, schemaObject(t, defs, "constraint")), entityschema.SupportedConstraintTypes())
	assertSameStringSet(t, schemaReservedRouteSlugs(t, schemaObject(t, defs, "routeSlug")), catalog.ReservedRootRouteSlugs())

	randomNaming := schemaNamingBranch(t, schemaObject(t, defs, "name"), entityschema.NamingStrategyRandom)
	length := schemaObject(t, schemaObject(t, randomNaming, "properties"), "length")
	assertNumber(t, length["minimum"], entityschema.MinRandomNameLength, "random naming minimum")
	assertNumber(t, length["maximum"], entityschema.MaxRandomNameLength, "random naming maximum")
	assertNumber(t, length["default"], entityschema.DefaultRandomNameLength, "random naming default")
}

func TestKebabNamePatternsMatchRuntime(t *testing.T) {
	appSchema := readJSONSchema(t, "app.schema.json")
	entitySchema := readJSONSchema(t, "entity.schema.json")
	fixtureSchema := readJSONSchema(t, "fixture.schema.json")
	jobSchema := readJSONSchema(t, "job.schema.json")
	patchSchema := readJSONSchema(t, "patch.schema.json")

	assertPattern(t, schemaObject(t, appSchema, "properties", "name"), fieldtype.NamePattern, "app name")
	assertPattern(t, schemaObject(t, schemaObject(t, appSchema, "properties"), "dependencies", "items"), fieldtype.NamePattern, "app dependency")
	assertPattern(t, schemaObject(t, schemaObject(t, entitySchema, "$defs"), "kebabName"), fieldtype.NamePattern, "entity kebabName")
	assertPattern(t, schemaObject(t, fixtureSchema, "properties", "entity"), fieldtype.NamePattern, "fixture entity")
	assertPattern(t, schemaObject(t, schemaObject(t, fixtureSchema, "properties"), "match", "items"), fieldtype.NamePattern, "fixture match")
	assertPattern(t, schemaObject(t, schemaObject(t, fixtureSchema, "$defs"), "record", "propertyNames"), fieldtype.NamePattern, "fixture record field")
	assertPattern(t, schemaObject(t, jobSchema, "properties", "queue"), fieldtype.NamePattern, "job queue")
	assertPattern(t, schemaObject(t, schemaObject(t, patchSchema, "$defs"), "kebabName"), fieldtype.NamePattern, "patch kebabName")
}

func TestJobSchemaContractsMatchRuntime(t *testing.T) {
	schema := readJSONSchema(t, "job.schema.json")

	assertSameStringSet(t, schemaStringEnum(t, schema, "required"), []string{"label", "timeout"})
	retry := schemaObject(t, schema, "properties", "retry")
	assertSameStringSet(t, schemaStringEnum(t, retry, "required"), []string{"attempts"})
	attempts := schemaObject(t, retry, "properties", "attempts")
	assertNumber(t, attempts["minimum"], 2, "retry attempts minimum")
}

func TestPatchSchemaContractsMatchRuntime(t *testing.T) {
	schema := readJSONSchema(t, "patch.schema.json")

	assertSameStringSet(t, schemaStringEnum(t, schemaObject(t, schema, "properties", "phase"), "enum"), patches.SupportedPhases())

	branches := schemaArray(t, schemaObject(t, schemaObject(t, schema, "$defs"), "operation"), "oneOf")
	var operationTypes []string
	for _, rawBranch := range branches {
		branch, ok := rawBranch.(map[string]any)
		if !ok {
			t.Fatalf("patch operation branch is %T, want object", rawBranch)
		}
		operationType := schemaConstString(t, schemaObject(t, schemaObject(t, branch, "properties"), "type"), "patch operation type")
		operationTypes = append(operationTypes, operationType)

		spec, ok := patches.OperationSpecFor(operationType)
		if !ok {
			t.Fatalf("patch schema includes unknown operation type %q", operationType)
		}
		assertSameStringSet(t, schemaStringEnum(t, branch, "required"), spec.Required)
		assertSameStringSet(t, schemaPropertyNames(t, schemaObject(t, branch, "properties")), spec.AllowedFields())
	}
	assertSameStringSet(t, operationTypes, patches.SupportedOperationTypes())
}

func readJSONSchema(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("%s is invalid JSON: %v", path, err)
	}
	return schema
}

func schemaObject(t *testing.T, schema map[string]any, path ...string) map[string]any {
	t.Helper()
	var current any = schema
	for _, segment := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("schema path %s parent is %T, want object", strings.Join(path, "."), current)
		}
		current, ok = object[segment]
		if !ok {
			t.Fatalf("schema path %s missing segment %q", strings.Join(path, "."), segment)
		}
	}
	object, ok := current.(map[string]any)
	if !ok {
		t.Fatalf("schema path %s is %T, want object", strings.Join(path, "."), current)
	}
	return object
}

func schemaArray(t *testing.T, schema map[string]any, key string) []any {
	t.Helper()
	raw, ok := schema[key]
	if !ok {
		t.Fatalf("schema key %q missing", key)
	}
	array, ok := raw.([]any)
	if !ok {
		t.Fatalf("schema key %q is %T, want array", key, raw)
	}
	return array
}

func schemaStringEnum(t *testing.T, schema map[string]any, key string) []string {
	t.Helper()
	values := schemaArray(t, schema, key)
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value, ok := raw.(string)
		if !ok {
			t.Fatalf("schema key %q contains %T, want string", key, raw)
		}
		result = append(result, value)
	}
	return result
}

func schemaPropertyNames(t *testing.T, properties map[string]any) []string {
	t.Helper()
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func schemaNamingStrategies(t *testing.T, naming map[string]any) []string {
	t.Helper()
	var strategies []string
	for _, rawBranch := range schemaArray(t, naming, "oneOf") {
		branch, ok := rawBranch.(map[string]any)
		if !ok {
			t.Fatalf("naming oneOf branch is %T, want object", rawBranch)
		}
		strategies = append(strategies, schemaConstString(t, schemaObject(t, schemaObject(t, branch, "properties"), "strategy"), "name.strategy"))
	}
	return strategies
}

func schemaNamingBranch(t *testing.T, naming map[string]any, strategy string) map[string]any {
	t.Helper()
	for _, rawBranch := range schemaArray(t, naming, "oneOf") {
		branch, ok := rawBranch.(map[string]any)
		if !ok {
			t.Fatalf("naming oneOf branch is %T, want object", rawBranch)
		}
		if schemaConstString(t, schemaObject(t, schemaObject(t, branch, "properties"), "strategy"), "name.strategy") == strategy {
			return branch
		}
	}
	t.Fatalf("naming strategy %q is missing from schema", strategy)
	return nil
}

func schemaCheckOperators(t *testing.T, check map[string]any) []string {
	t.Helper()
	operatorSet := map[string]struct{}{}
	for _, rawBranch := range schemaArray(t, check, "oneOf") {
		branch, ok := rawBranch.(map[string]any)
		if !ok {
			t.Fatalf("check oneOf branch is %T, want object", rawBranch)
		}
		for _, operator := range schemaStringEnum(t, schemaObject(t, schemaObject(t, branch, "properties"), "operator"), "enum") {
			operatorSet[operator] = struct{}{}
		}
	}
	return sortedKeys(operatorSet)
}

func schemaConstraintTypes(t *testing.T, constraint map[string]any) []string {
	t.Helper()
	var types []string
	for _, rawBranch := range schemaArray(t, constraint, "oneOf") {
		branch, ok := rawBranch.(map[string]any)
		if !ok {
			t.Fatalf("constraint oneOf branch is %T, want object", rawBranch)
		}
		types = append(types, schemaConstString(t, schemaObject(t, schemaObject(t, branch, "properties"), "type"), "constraint.type"))
	}
	return types
}

func schemaReservedRouteSlugs(t *testing.T, routeSlug map[string]any) []string {
	t.Helper()
	for _, rawBranch := range schemaArray(t, routeSlug, "allOf") {
		branch, ok := rawBranch.(map[string]any)
		if !ok {
			t.Fatalf("routeSlug allOf branch is %T, want object", rawBranch)
		}
		if notObject, ok := branch["not"].(map[string]any); ok {
			return schemaStringEnum(t, notObject, "enum")
		}
	}
	t.Fatal("routeSlug schema missing reserved slug enum")
	return nil
}

func schemaConstString(t *testing.T, schema map[string]any, label string) string {
	t.Helper()
	value, ok := schema["const"].(string)
	if !ok {
		t.Fatalf("%s const is %#v, want string", label, schema["const"])
	}
	return value
}

func assertPattern(t *testing.T, schema map[string]any, want string, label string) {
	t.Helper()
	got, ok := schema["pattern"].(string)
	if !ok {
		t.Fatalf("%s pattern is %#v, want string", label, schema["pattern"])
	}
	if got != want {
		t.Fatalf("%s pattern = %q, want %q", label, got, want)
	}
}

func assertNumber(t *testing.T, raw any, want int, label string) {
	t.Helper()
	got, ok := raw.(float64)
	if !ok {
		t.Fatalf("%s = %#v, want number", label, raw)
	}
	if got != float64(want) {
		t.Fatalf("%s = %v, want %d", label, got, want)
	}
}

func assertSameStringSet(t *testing.T, got []string, want []string) {
	t.Helper()
	got = append([]string(nil), got...)
	want = append([]string(nil), want...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("string set = %#v, want %#v", got, want)
	}
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

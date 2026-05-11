package schemas

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemasAreValidJSON(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir(schemas) error = %v", err)
	}
	var found int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".schema.json") {
			continue
		}
		found++
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
	if found != 4 {
		t.Fatalf("schema file count = %d, want 4", found)
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
	if len(rawSchemas) != 4 {
		t.Fatalf("yaml.schemas count = %d, want 4", len(rawSchemas))
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

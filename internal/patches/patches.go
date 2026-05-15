// Package patches discovers and validates app-owned patch files.
package patches

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dygo-dev/dygo/internal/app/manifest"
	"gopkg.in/yaml.v3"
)

const (
	Kind    = "patch"
	Version = 1

	PhasePreSync  = "pre-sync"
	PhasePostSync = "post-sync"
)

var operationTypes = map[string]struct{}{
	"rename-field":      {},
	"rename-entity":     {},
	"copy-field":        {},
	"backfill-field":    {},
	"drop-field":        {},
	"change-field-type": {},
	"sql":               {},
}

// Patch is one v1 patch document.
type Patch struct {
	Kind        string
	Version     int
	ID          string
	Phase       string
	Description string
	Operations  []Operation
	Line        int
}

// Operation is one raw patch operation preserved for future planners.
type Operation struct {
	Type   string
	Fields map[string]yaml.Node
	Node   yaml.Node
	Line   int
}

// LoadedPatch is one decoded patch file from an app.
type LoadedPatch struct {
	AppName         string
	AppDir          string
	Path            string
	AppRelativePath string
	Checksum        string
	Patch           Patch
}

// Discover loads and validates patch files from each app's patches path.
func Discover(apps []manifest.LoadedApp) ([]LoadedPatch, error) {
	orderedApps, err := orderApps(apps)
	if err != nil {
		return nil, err
	}

	seen := map[string]string{}
	var patches []LoadedPatch
	for _, app := range orderedApps {
		patchesDir := filepath.Join(app.Dir, app.Manifest.Paths.WithDefaults().Patches)
		entries, err := os.ReadDir(patchesDir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read patches for app %q: %w", app.Manifest.Name, err)
		}
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})
		for _, entry := range entries {
			if entry.IsDir() || !isPatchFilename(entry.Name()) {
				continue
			}
			path := filepath.Join(patchesDir, entry.Name())
			patch, checksum, err := loadFileWithChecksum(path)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
			key := app.Manifest.Name + "\x00" + patch.ID
			if previous, ok := seen[key]; ok {
				return nil, fmt.Errorf("duplicate patch id %q for app %q in %s and %s", patch.ID, app.Manifest.Name, previous, path)
			}
			seen[key] = path

			appRelativePath, err := filepath.Rel(app.Dir, path)
			if err != nil {
				return nil, fmt.Errorf("make patch path relative to app %q: %w", app.Manifest.Name, err)
			}
			patches = append(patches, LoadedPatch{
				AppName:         app.Manifest.Name,
				AppDir:          app.Dir,
				Path:            path,
				AppRelativePath: filepath.ToSlash(appRelativePath),
				Checksum:        checksum,
				Patch:           patch,
			})
		}
	}
	return patches, nil
}

// LoadFile loads and validates one patch file.
func LoadFile(path string) (Patch, error) {
	patch, _, err := loadFileWithChecksum(path)
	return patch, err
}

// Decode decodes one patch document.
func Decode(data []byte) (Patch, error) {
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&root); err != nil {
		if errors.Is(err, io.EOF) {
			return Patch{}, fmt.Errorf("patch file is empty")
		}
		return Patch{}, err
	}
	if isEmptyDocument(root) {
		return Patch{}, fmt.Errorf("patch file is empty")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err == nil {
		if !isEmptyDocument(extra) {
			return Patch{}, fmt.Errorf("patch file must contain a single document")
		}
	} else if !errors.Is(err, io.EOF) {
		return Patch{}, err
	}

	if err := rejectDuplicateKeysNode(&root, "$"); err != nil {
		return Patch{}, err
	}
	document := documentMapping(&root)
	if document == nil {
		return Patch{}, fmt.Errorf("patch document must be a mapping")
	}
	if len(document.Content) == 0 {
		return Patch{}, fmt.Errorf("patch document must be a non-empty mapping")
	}

	patch := Patch{Line: document.Line}
	seen := map[string]bool{}
	for i := 0; i < len(document.Content); i += 2 {
		key := document.Content[i]
		value := document.Content[i+1]
		seen[key.Value] = true
		switch key.Value {
		case "kind":
			kind, err := scalarString(value, "kind")
			if err != nil {
				return Patch{}, err
			}
			patch.Kind = kind
		case "version":
			version, err := scalarInt(value, "version")
			if err != nil {
				return Patch{}, err
			}
			patch.Version = version
		case "id":
			id, err := scalarString(value, "id")
			if err != nil {
				return Patch{}, err
			}
			patch.ID = id
		case "phase":
			phase, err := scalarString(value, "phase")
			if err != nil {
				return Patch{}, err
			}
			patch.Phase = phase
		case "description":
			description, err := scalarString(value, "description")
			if err != nil {
				return Patch{}, err
			}
			patch.Description = description
		case "operations":
			operations, err := decodeOperations(value)
			if err != nil {
				return Patch{}, err
			}
			patch.Operations = operations
		default:
			return Patch{}, fmt.Errorf("unknown patch field %q at line %d", key.Value, key.Line)
		}
	}
	if err := validatePatch(patch, seen); err != nil {
		return Patch{}, err
	}
	return patch, nil
}

func loadFileWithChecksum(path string) (Patch, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Patch{}, "", err
	}
	patch, err := Decode(data)
	if err != nil {
		return Patch{}, "", err
	}
	if err := validatePatchFileName(path, patch.ID); err != nil {
		return Patch{}, "", err
	}
	return patch, checksum(data), nil
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validatePatchFileName(path string, id string) error {
	expectedID := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if id != expectedID {
		return fmt.Errorf("patch id %q must match file name %q", id, expectedID)
	}
	return nil
}

func decodeOperations(node *yaml.Node) ([]Operation, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("patch operations must be a sequence at line %d", node.Line)
	}
	operations := make([]Operation, 0, len(node.Content))
	for index, item := range node.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("patch operation at index %d must be a mapping at line %d", index, item.Line)
		}
		operation := Operation{
			Fields: map[string]yaml.Node{},
			Node:   *item,
			Line:   item.Line,
		}
		for i := 0; i < len(item.Content); i += 2 {
			key := item.Content[i]
			value := item.Content[i+1]
			operation.Fields[key.Value] = *value
			if key.Value == "type" {
				operationType, err := scalarString(value, "operation type")
				if err != nil {
					return nil, err
				}
				operation.Type = operationType
			}
		}
		if strings.TrimSpace(operation.Type) == "" {
			return nil, fmt.Errorf("patch operation at index %d type is required", index)
		}
		if _, ok := operationTypes[operation.Type]; !ok {
			return nil, fmt.Errorf("patch operation at index %d has unknown type %q", index, operation.Type)
		}
		operations = append(operations, operation)
	}
	return operations, nil
}

func validatePatch(patch Patch, seen map[string]bool) error {
	if !seen["kind"] || strings.TrimSpace(patch.Kind) == "" {
		return fmt.Errorf("patch kind is required")
	}
	if patch.Kind != Kind {
		return fmt.Errorf("patch kind must be %q", Kind)
	}
	if !seen["version"] {
		return fmt.Errorf("patch version is required")
	}
	if patch.Version != Version {
		return fmt.Errorf("patch version must be %d", Version)
	}
	if !seen["id"] || strings.TrimSpace(patch.ID) == "" {
		return fmt.Errorf("patch id is required")
	}
	if !seen["phase"] || strings.TrimSpace(patch.Phase) == "" {
		return fmt.Errorf("patch phase is required")
	}
	if patch.Phase != PhasePreSync && patch.Phase != PhasePostSync {
		return fmt.Errorf("patch phase must be %q or %q", PhasePreSync, PhasePostSync)
	}
	if !seen["description"] || strings.TrimSpace(patch.Description) == "" {
		return fmt.Errorf("patch description is required")
	}
	if !seen["operations"] || len(patch.Operations) == 0 {
		return fmt.Errorf("patch operations are required")
	}
	return nil
}

func orderApps(apps []manifest.LoadedApp) ([]manifest.LoadedApp, error) {
	byName := map[string]manifest.LoadedApp{}
	for _, app := range apps {
		name := app.Manifest.Name
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("patch app name is required")
		}
		if previous, ok := byName[name]; ok {
			return nil, fmt.Errorf("duplicate app %q in %s and %s", name, previous.ManifestPath, app.ManifestPath)
		}
		byName[name] = app
	}

	indegree := map[string]int{}
	dependents := map[string][]string{}
	for _, app := range apps {
		name := app.Manifest.Name
		indegree[name] = 0
	}
	for _, app := range apps {
		name := app.Manifest.Name
		seenDependencies := map[string]struct{}{}
		for _, dependency := range app.Manifest.Dependencies {
			if _, ok := byName[dependency]; !ok {
				return nil, fmt.Errorf("app %q depends on unknown app %q", name, dependency)
			}
			if _, ok := seenDependencies[dependency]; ok {
				continue
			}
			seenDependencies[dependency] = struct{}{}
			indegree[name]++
			dependents[dependency] = append(dependents[dependency], name)
		}
	}
	for dependency := range dependents {
		sort.Strings(dependents[dependency])
	}

	var ready []string
	for name, degree := range indegree {
		if degree == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)

	ordered := make([]manifest.LoadedApp, 0, len(apps))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byName[name])
		for _, dependent := range dependents[name] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = append(ready, dependent)
			}
		}
		sort.Strings(ready)
	}
	if len(ordered) != len(apps) {
		var cycle []string
		for name, degree := range indegree {
			if degree > 0 {
				cycle = append(cycle, name)
			}
		}
		sort.Strings(cycle)
		return nil, fmt.Errorf("app dependency cycle among %s", strings.Join(cycle, ", "))
	}
	return ordered, nil
}

func isPatchFilename(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yml" || ext == ".yaml"
}

func documentMapping(root *yaml.Node) *yaml.Node {
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		return valueMapping(root.Content[0])
	}
	return valueMapping(root)
}

func valueMapping(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.MappingNode {
		return node
	}
	return nil
}

func scalarString(node *yaml.Node, name string) (string, error) {
	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("patch %s must be a scalar string at line %d", name, node.Line)
	}
	return node.Value, nil
}

func scalarInt(node *yaml.Node, name string) (int, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
		return 0, fmt.Errorf("patch %s must be an integer at line %d", name, node.Line)
	}
	var value int
	if err := node.Decode(&value); err != nil {
		return 0, fmt.Errorf("decode patch %s: %w", name, err)
	}
	return value, nil
}

func rejectDuplicateKeysNode(node *yaml.Node, location string) error {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for index, child := range node.Content {
			if err := rejectDuplicateKeysNode(child, fmt.Sprintf("%s[%d]", location, index)); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		seen := map[string]int{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			if key.Kind != yaml.ScalarNode {
				return fmt.Errorf("patch mapping key must be scalar at %s line %d", location, key.Line)
			}
			if previous, ok := seen[key.Value]; ok {
				return fmt.Errorf("duplicate patch key %q at %s line %d, previously defined at line %d", key.Value, location, key.Line, previous)
			}
			seen[key.Value] = key.Line
			if err := rejectDuplicateKeysNode(value, location+"."+key.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func isEmptyDocument(node yaml.Node) bool {
	if node.Kind == 0 {
		return true
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) == 0 {
		return true
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		child := node.Content[0]
		return child.Kind == yaml.ScalarNode && child.Tag == "!!null" && child.Value == ""
	}
	return false
}

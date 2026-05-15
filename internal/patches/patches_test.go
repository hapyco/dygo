package patches

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/internal/app/manifest"
)

func TestDecodeValidPatch(t *testing.T) {
	patch, err := Decode([]byte(`kind: patch
version: 1
id: rename-customer-email
phase: pre-sync
description: Rename the legacy customer email field.
operations:
  - type: rename-field
    entity: customer
    from: customer-email
    to: email
  - type: sql
    sql: SELECT 1;
    reason: repair denormalized data
`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if patch.Kind != Kind || patch.Version != Version || patch.ID != "rename-customer-email" || patch.Phase != PhasePreSync {
		t.Fatalf("Decode() patch = %#v", patch)
	}
	if patch.Description == "" {
		t.Fatalf("Decode().Description is empty")
	}
	if len(patch.Operations) != 2 {
		t.Fatalf("Decode().Operations len = %d, want 2", len(patch.Operations))
	}
	if patch.Operations[0].Type != "rename-field" {
		t.Fatalf("Decode().Operations[0].Type = %q, want rename-field", patch.Operations[0].Type)
	}
	if _, ok := patch.Operations[0].Fields["from"]; !ok {
		t.Fatalf("Decode().Operations[0].Fields missing raw from field")
	}
	if patch.Operations[1].Type != "sql" {
		t.Fatalf("Decode().Operations[1].Type = %q, want sql", patch.Operations[1].Type)
	}
	if _, ok := patch.Operations[1].Fields["reason"]; !ok {
		t.Fatalf("Decode().Operations[1].Fields missing raw reason field")
	}
}

func TestDecodeAllowsPostSyncPhaseAndAllV1OperationTypes(t *testing.T) {
	patch, err := Decode([]byte(`kind: patch
version: 1
id: all-operations
phase: post-sync
description: Exercise every v1 patch operation type.
operations:
  - type: rename-field
  - type: rename-entity
  - type: copy-field
  - type: backfill-field
  - type: drop-field
  - type: change-field-type
  - type: sql
`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if patch.Phase != PhasePostSync {
		t.Fatalf("Decode().Phase = %q, want %q", patch.Phase, PhasePostSync)
	}
	if len(patch.Operations) != 7 {
		t.Fatalf("Decode().Operations len = %d, want 7", len(patch.Operations))
	}
}

func TestDecodeRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "empty file",
			body: "",
			want: "patch file is empty",
		},
		{
			name: "empty mapping",
			body: "{}",
			want: "patch document must be a non-empty mapping",
		},
		{
			name: "non mapping document",
			body: "[]",
			want: "patch document must be a mapping",
		},
		{
			name: "multiple documents",
			body: validPatchYAML("first") + "---\n" + validPatchYAML("second"),
			want: "patch file must contain a single document",
		},
		{
			name: "duplicate top level key",
			body: strings.Replace(validPatchYAML("duplicate-kind"), "kind: patch\n", "kind: patch\nkind: patch\n", 1),
			want: `duplicate patch key "kind"`,
		},
		{
			name: "duplicate nested key",
			body: strings.Replace(validPatchYAML("duplicate-operation-type"), "type: rename-field\n", "type: rename-field\n    type: copy-field\n", 1),
			want: `duplicate patch key "type"`,
		},
		{
			name: "unknown top level field",
			body: strings.Replace(validPatchYAML("unknown-field"), "operations:\n", "unknown: true\noperations:\n", 1),
			want: `unknown patch field "unknown"`,
		},
		{
			name: "missing kind",
			body: strings.Replace(validPatchYAML("missing-kind"), "kind: patch\n", "", 1),
			want: "patch kind is required",
		},
		{
			name: "wrong kind",
			body: strings.Replace(validPatchYAML("wrong-kind"), "kind: patch", "kind: migration", 1),
			want: `patch kind must be "patch"`,
		},
		{
			name: "missing version",
			body: strings.Replace(validPatchYAML("missing-version"), "version: 1\n", "", 1),
			want: "patch version is required",
		},
		{
			name: "non integer version",
			body: strings.Replace(validPatchYAML("non-integer-version"), "version: 1", "version: one", 1),
			want: "patch version must be an integer",
		},
		{
			name: "wrong version",
			body: strings.Replace(validPatchYAML("wrong-version"), "version: 1", "version: 2", 1),
			want: "patch version must be 1",
		},
		{
			name: "missing id",
			body: strings.Replace(validPatchYAML("missing-id"), "id: missing-id\n", "", 1),
			want: "patch id is required",
		},
		{
			name: "missing phase",
			body: strings.Replace(validPatchYAML("missing-phase"), "phase: pre-sync\n", "", 1),
			want: "patch phase is required",
		},
		{
			name: "invalid phase",
			body: strings.Replace(validPatchYAML("invalid-phase"), "phase: pre-sync", "phase: before-sync", 1),
			want: `patch phase must be "pre-sync" or "post-sync"`,
		},
		{
			name: "missing description",
			body: strings.Replace(validPatchYAML("missing-description"), "description: Test patch.\n", "", 1),
			want: "patch description is required",
		},
		{
			name: "empty operations",
			body: strings.Replace(validPatchYAML("empty-operations"), "operations:\n  - type: rename-field\n    entity: customer\n", "operations: []\n", 1),
			want: "patch operations are required",
		},
		{
			name: "operations not sequence",
			body: strings.Replace(validPatchYAML("operations-not-sequence"), "operations:\n  - type: rename-field\n    entity: customer\n", "operations: {}\n", 1),
			want: "patch operations must be a sequence",
		},
		{
			name: "operation not mapping",
			body: strings.Replace(validPatchYAML("operation-not-mapping"), "operations:\n  - type: rename-field\n    entity: customer\n", "operations:\n  - rename-field\n", 1),
			want: "patch operation at index 0 must be a mapping",
		},
		{
			name: "missing operation type",
			body: strings.Replace(validPatchYAML("missing-operation-type"), "  - type: rename-field\n", "  - ", 1),
			want: "patch operation at index 0 type is required",
		},
		{
			name: "non scalar operation type",
			body: strings.Replace(validPatchYAML("non-scalar-operation-type"), "type: rename-field", "type: [rename-field]", 1),
			want: "patch operation type must be a scalar string",
		},
		{
			name: "unknown operation type",
			body: strings.Replace(validPatchYAML("unknown-operation-type"), "type: rename-field", "type: rename-column", 1),
			want: `patch operation at index 0 has unknown type "rename-column"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode([]byte(tt.body))
			requireErrorContains(t, err, tt.want)
		})
	}
}

func TestLoadFileRequiresIDToMatchFilename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "expected-id.yml")
	writeFile(t, path, validPatchYAML("actual-id"))

	_, err := LoadFile(path)
	requireErrorContains(t, err, `patch id "actual-id" must match file name "expected-id"`)
}

func TestLoadFileReturnsPatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "load-me.yaml")
	writeFile(t, path, validPatchYAML("load-me"))

	patch, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if patch.ID != "load-me" {
		t.Fatalf("LoadFile().ID = %q, want load-me", patch.ID)
	}
}

func TestDiscoverLoadsAndOrdersPatches(t *testing.T) {
	root := t.TempDir()
	core := testLoadedApp(t, root, "core", nil, "")
	sales := testLoadedApp(t, root, "sales", []string{"core"}, "metadata/patches")
	zed := testLoadedApp(t, root, "zed", []string{"core"}, "")

	writeFile(t, filepath.Join(core.Dir, "patches", "001-core.yml"), validPatchYAML("001-core"))
	writeFile(t, filepath.Join(sales.Dir, "metadata", "patches", "002-sales.yaml"), validPatchYAML("002-sales"))
	writeFile(t, filepath.Join(sales.Dir, "metadata", "patches", "001-sales.yml"), validPatchYAML("001-sales"))
	writeFile(t, filepath.Join(sales.Dir, "metadata", "patches", "nested", "000-ignored.yml"), validPatchYAML("000-ignored"))
	writeFile(t, filepath.Join(sales.Dir, "metadata", "patches", "README.md"), "ignored")
	writeFile(t, filepath.Join(zed.Dir, "patches", "001-zed.yaml"), validPatchYAML("001-zed"))

	loaded, err := Discover([]manifest.LoadedApp{zed, sales, core})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	got := make([]string, 0, len(loaded))
	for _, patch := range loaded {
		got = append(got, patch.AppName+":"+patch.Patch.ID)
	}
	want := []string{"core:001-core", "sales:001-sales", "sales:002-sales", "zed:001-zed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Discover() order = %v, want %v", got, want)
	}

	if loaded[1].AppRelativePath != "metadata/patches/001-sales.yml" {
		t.Fatalf("Discover()[1].AppRelativePath = %q, want metadata/patches/001-sales.yml", loaded[1].AppRelativePath)
	}
	if loaded[1].AppDir != sales.Dir {
		t.Fatalf("Discover()[1].AppDir = %q, want %q", loaded[1].AppDir, sales.Dir)
	}

	bytes, err := os.ReadFile(loaded[1].Path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sum := sha256.Sum256(bytes)
	wantChecksum := hex.EncodeToString(sum[:])
	if loaded[1].Checksum != wantChecksum {
		t.Fatalf("Discover()[1].Checksum = %q, want %q", loaded[1].Checksum, wantChecksum)
	}
}

func TestDiscoverAllowsMissingPatchDirectory(t *testing.T) {
	root := t.TempDir()
	app := testLoadedApp(t, root, "sales", nil, "")

	loaded, err := Discover([]manifest.LoadedApp{app})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("Discover() len = %d, want 0", len(loaded))
	}
}

func TestDiscoverAllowsSamePatchIDAcrossDifferentApps(t *testing.T) {
	root := t.TempDir()
	billing := testLoadedApp(t, root, "billing", nil, "")
	sales := testLoadedApp(t, root, "sales", nil, "")
	writeFile(t, filepath.Join(billing.Dir, "patches", "001-shared.yml"), validPatchYAML("001-shared"))
	writeFile(t, filepath.Join(sales.Dir, "patches", "001-shared.yml"), validPatchYAML("001-shared"))

	loaded, err := Discover([]manifest.LoadedApp{sales, billing})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("Discover() len = %d, want 2", len(loaded))
	}
}

func TestDiscoverRejectsDuplicatePatchIDWithinApp(t *testing.T) {
	root := t.TempDir()
	app := testLoadedApp(t, root, "sales", nil, "")
	writeFile(t, filepath.Join(app.Dir, "patches", "001-duplicate.yml"), validPatchYAML("001-duplicate"))
	writeFile(t, filepath.Join(app.Dir, "patches", "001-duplicate.yaml"), validPatchYAML("001-duplicate"))

	_, err := Discover([]manifest.LoadedApp{app})
	requireErrorContains(t, err, `duplicate patch id "001-duplicate" for app "sales"`)
}

func TestDiscoverRejectsFilenameIDMismatch(t *testing.T) {
	root := t.TempDir()
	app := testLoadedApp(t, root, "sales", nil, "")
	writeFile(t, filepath.Join(app.Dir, "patches", "001-expected.yml"), validPatchYAML("001-actual"))

	_, err := Discover([]manifest.LoadedApp{app})
	requireErrorContains(t, err, `patch id "001-actual" must match file name "001-expected"`)
}

func TestDiscoverRejectsInvalidAppDependencies(t *testing.T) {
	t.Run("missing dependency", func(t *testing.T) {
		root := t.TempDir()
		app := testLoadedApp(t, root, "sales", []string{"core"}, "")

		_, err := Discover([]manifest.LoadedApp{app})
		requireErrorContains(t, err, `app "sales" depends on unknown app "core"`)
	})

	t.Run("dependency cycle", func(t *testing.T) {
		root := t.TempDir()
		core := testLoadedApp(t, root, "core", []string{"sales"}, "")
		sales := testLoadedApp(t, root, "sales", []string{"core"}, "")

		_, err := Discover([]manifest.LoadedApp{core, sales})
		requireErrorContains(t, err, "app dependency cycle among core, sales")
	})
}

func TestChecksumUsesExactBytes(t *testing.T) {
	withoutNewline := checksum([]byte("kind: patch"))
	withNewline := checksum([]byte("kind: patch\n"))
	if withoutNewline == withNewline {
		t.Fatalf("checksum() did not account for exact file bytes")
	}

	sum := sha256.Sum256([]byte("kind: patch"))
	want := hex.EncodeToString(sum[:])
	if withoutNewline != want {
		t.Fatalf("checksum() = %q, want %q", withoutNewline, want)
	}
}

func testLoadedApp(t *testing.T, root string, name string, dependencies []string, patchesPath string) manifest.LoadedApp {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	paths := manifest.DefaultPaths()
	if patchesPath != "" {
		paths.Patches = patchesPath
	}
	return manifest.LoadedApp{
		Dir:          dir,
		ManifestPath: filepath.Join(dir, manifest.Filename),
		Manifest: manifest.Manifest{
			Name:         name,
			Label:        name,
			Version:      "0.1.0",
			Dependencies: dependencies,
			Paths:        paths,
		},
	}
}

func validPatchYAML(id string) string {
	return fmt.Sprintf(`kind: patch
version: 1
id: %s
phase: pre-sync
description: Test patch.
operations:
  - type: rename-field
    entity: customer
`, id)
}

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func requireErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want containing %q", err.Error(), want)
	}
}

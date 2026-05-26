package secrets

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

func TestStoreLifecycle(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	paths, err := store.Init(false)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if _, err := os.Stat(paths.MasterKeyFile); err != nil {
		t.Fatalf("Stat(.dygo/secrets/master.key) error = %v", err)
	}

	ciphertext, err := os.ReadFile(paths.SecretFile)
	if err != nil {
		t.Fatalf("ReadFile(secret) error = %v", err)
	}
	if !strings.Contains(string(ciphertext), "-----BEGIN AGE ENCRYPTED FILE-----") {
		t.Fatalf("encrypted file is not age armor:\n%s", ciphertext)
	}

	if err := store.Set(EnvironmentDevelopment, "DATABASE_URL", "postgres://local"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	secret, err := store.Get(EnvironmentDevelopment, "DATABASE_URL")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if secret.Value != "postgres://local" {
		t.Fatalf("Get().Value = %q, want %q", secret.Value, "postgres://local")
	}

	entries, err := store.List(EnvironmentDevelopment)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "DATABASE_URL" {
		t.Fatalf("List() = %#v, want one DATABASE_URL entry", entries)
	}

	configPath := filepath.Join(root, "config", "app.yaml")
	if err := os.WriteFile(configPath, []byte("env:\n  DATABASE_URL:\n    secret: DATABASE_URL\ndatabase:\n  url:\n    secret: DATABASE_URL\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	if err := store.Validate(EnvironmentDevelopment); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if err := store.Remove(EnvironmentDevelopment, "DATABASE_URL"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := store.Get(EnvironmentDevelopment, "DATABASE_URL"); err == nil {
		t.Fatal("Get() error = nil after Remove(), want error")
	}
}

func TestStoreValidationFailures(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	if _, err := ParseEnvironment("dev"); err == nil {
		t.Fatal("ParseEnvironment(dev) error = nil, want error")
	}
	if err := ValidateSecretName("database..url"); err == nil {
		t.Fatal("ValidateSecretName(database..url) error = nil, want error")
	}

	if _, err := store.Init(false); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := store.Set(EnvironmentDevelopment, "database..url", "value"); err == nil {
		t.Fatal("Set(invalid name) error = nil, want error")
	}

	configPath := filepath.Join(root, "config", "app.yaml")
	if err := os.WriteFile(configPath, []byte("database:\n  url:\n    secret: DATABASE_URL\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	if err := store.Validate(EnvironmentDevelopment); err == nil {
		t.Fatal("Validate() error = nil for missing manifest secret, want error")
	}
}

func TestStoreResolvesNestedPlainYAMLSecrets(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)
	if _, err := store.Init(false); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := store.SavePlaintext(EnvironmentDevelopment, []byte("database:\n  url: postgres://nested\n")); err != nil {
		t.Fatalf("SavePlaintext(nested) error = %v", err)
	}

	secret, err := store.Get(EnvironmentDevelopment, "database.url")
	if err != nil {
		t.Fatalf("Get(database.url) error = %v", err)
	}
	if secret.Value != "postgres://nested" {
		t.Fatalf("Get(database.url).Value = %q, want nested value", secret.Value)
	}

	configPath := filepath.Join(root, "config", "app.yaml")
	if err := os.WriteFile(configPath, []byte("database:\n  url:\n    secret: database.url\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	if err := store.Validate(EnvironmentDevelopment); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadWithUnusableMasterKeyFails(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	paths, err := store.Init(false)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := store.Set(EnvironmentDevelopment, "DATABASE_URL", "postgres://local"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	wrongIdentity, err := age.GenerateHybridIdentity()
	if err != nil {
		t.Fatalf("GenerateHybridIdentity() error = %v", err)
	}
	if err := os.WriteFile(paths.MasterKeyFile, []byte(wrongIdentity.String()+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(key) error = %v", err)
	}

	if _, err := store.Load(EnvironmentDevelopment); err == nil {
		t.Fatal("Load() error = nil with wrong identity, want error")
	}
}

func TestRotateKeyPreservesAllEnvironments(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	if _, err := store.Init(false); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := store.Set(EnvironmentDevelopment, "DATABASE_URL", "postgres://development"); err != nil {
		t.Fatalf("Set(development) error = %v", err)
	}
	if err := store.Set(EnvironmentStaging, "DATABASE_URL", "postgres://staging"); err != nil {
		t.Fatalf("Set(staging) error = %v", err)
	}
	if err := store.Set(EnvironmentProduction, "DATABASE_URL", "postgres://production"); err != nil {
		t.Fatalf("Set(production) error = %v", err)
	}

	if _, err := store.RotateKey(); err != nil {
		t.Fatalf("RotateKey() error = %v", err)
	}
	assertNoRotationArtifacts(t, store)

	for _, tt := range []struct {
		env  Environment
		want string
	}{
		{env: EnvironmentDevelopment, want: "postgres://development"},
		{env: EnvironmentStaging, want: "postgres://staging"},
		{env: EnvironmentProduction, want: "postgres://production"},
	} {
		secret, err := store.Get(tt.env, "DATABASE_URL")
		if err != nil {
			t.Fatalf("Get(%s) error = %v", tt.env, err)
		}
		if secret.Value != tt.want {
			t.Fatalf("Get(%s).Value = %q, want %q", tt.env, secret.Value, tt.want)
		}
	}
}

func TestRotateKeyFailureBeforeMasterReplacementLeavesOldState(t *testing.T) {
	store := newRotatableStore(t)
	oldMaster, oldSecrets := readRotationState(t, store)
	store.fileOps.rename = func(oldPath string, newPath string) error {
		if strings.HasSuffix(oldPath, "development.yml.age.dual.next") {
			return errors.New("injected dual replacement failure")
		}
		return os.Rename(oldPath, newPath)
	}

	_, err := store.RotateKey()
	if err == nil {
		t.Fatal("RotateKey() error = nil, want dual replacement failure")
	}
	assertRotationErrorRedacted(t, err)
	assertRotationState(t, store, oldMaster, oldSecrets)
	assertRotatedSecrets(t, store)
}

func newRotatableStore(t *testing.T) Store {
	t.Helper()

	store := NewStore(t.TempDir())
	if _, err := store.Init(false); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	for _, tt := range []struct {
		env   Environment
		value string
	}{
		{env: EnvironmentDevelopment, value: "postgres://development-secret"},
		{env: EnvironmentStaging, value: "postgres://staging-secret"},
		{env: EnvironmentProduction, value: "postgres://production-secret"},
	} {
		if err := store.Set(tt.env, "DATABASE_URL", tt.value); err != nil {
			t.Fatalf("Set(%s DATABASE_URL) error = %v", tt.env, err)
		}
	}
	return store
}

func readRotationState(t *testing.T, store Store) ([]byte, map[Environment][]byte) {
	t.Helper()

	master, err := os.ReadFile(store.Paths(EnvironmentDevelopment).MasterKeyFile)
	if err != nil {
		t.Fatalf("ReadFile(.dygo/secrets/master.key) error = %v", err)
	}
	files := make(map[Environment][]byte)
	for _, env := range SupportedEnvironments() {
		data, err := os.ReadFile(store.Paths(env).SecretFile)
		if err != nil {
			t.Fatalf("ReadFile(%s secrets) error = %v", env, err)
		}
		files[env] = data
	}
	return master, files
}

func assertRotationState(t *testing.T, store Store, wantMaster []byte, wantSecrets map[Environment][]byte) {
	t.Helper()

	gotMaster, gotSecrets := readRotationState(t, store)
	if !bytes.Equal(gotMaster, wantMaster) {
		t.Fatal(".dygo/secrets/master.key changed after failed rotation")
	}
	for _, env := range SupportedEnvironments() {
		if !bytes.Equal(gotSecrets[env], wantSecrets[env]) {
			t.Fatalf("%s secrets changed after failed rotation", env)
		}
	}
}

func assertRotatedSecrets(t *testing.T, store Store) {
	t.Helper()

	for _, tt := range []struct {
		env  Environment
		want string
	}{
		{env: EnvironmentDevelopment, want: "postgres://development-secret"},
		{env: EnvironmentStaging, want: "postgres://staging-secret"},
		{env: EnvironmentProduction, want: "postgres://production-secret"},
	} {
		secret, err := store.Get(tt.env, "DATABASE_URL")
		if err != nil {
			t.Fatalf("Get(%s DATABASE_URL) error = %v", tt.env, err)
		}
		if secret.Value != tt.want {
			t.Fatalf("Get(%s DATABASE_URL).Value = %q, want %q", tt.env, secret.Value, tt.want)
		}
	}
}

func assertNoRotationArtifacts(t *testing.T, store Store) {
	t.Helper()

	rotateDir := filepath.Join(store.Paths(EnvironmentDevelopment).TempDir, "rotate-key")
	if _, err := os.Stat(rotateDir); err == nil {
		t.Fatalf("rotation artifact directory %s still exists", rotateDir)
	} else if !os.IsNotExist(err) {
		t.Fatalf("Stat(%s) error = %v", rotateDir, err)
	}
}

func assertRotationErrorRedacted(t *testing.T, err error) {
	t.Helper()

	message := err.Error()
	for _, leaked := range []string{"postgres://", "development-secret", "staging-secret", "production-secret", "AGE-SECRET-KEY"} {
		if strings.Contains(message, leaked) {
			t.Fatalf("RotateKey() error leaked %q: %s", leaked, message)
		}
	}
}

func TestDecodeDocumentRejectsDuplicateSecretNames(t *testing.T) {
	plaintext := []byte("database:\n  url: first\n  url: second\n")

	if _, err := DecodeDocument(plaintext, EnvironmentDevelopment); err == nil {
		t.Fatal("DecodeDocument() error = nil for duplicate secret names, want error")
	}
}

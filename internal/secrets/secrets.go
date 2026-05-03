package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
	"gopkg.in/yaml.v3"
)

const (
	documentVersion = 1

	EnvironmentDevelopment Environment = "development"
	EnvironmentStaging     Environment = "staging"
	EnvironmentProduction  Environment = "production"
)

var secretNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// Environment names a supported dygo runtime environment.
type Environment string

// Secret contains one secret value and its metadata.
type Secret struct {
	Value     string `yaml:"value"`
	UpdatedAt string `yaml:"updated_at"`
}

// Document is the decrypted YAML payload encrypted on disk.
type Document struct {
	Version     int               `yaml:"version"`
	Environment Environment       `yaml:"environment"`
	Secrets     map[string]Secret `yaml:"secrets"`
}

// Entry is a sorted secret listing item.
type Entry struct {
	Name   string
	Secret Secret
}

// Paths contains the filesystem locations used for one environment.
type Paths struct {
	SecretFile    string
	RecipientFile string
	KeyFile       string
	TempDir       string
}

// Store manages encrypted dygo secret files below a repository root.
type Store struct {
	root  string
	clock func() time.Time
}

// NewStore returns a Store rooted at the provided directory.
func NewStore(root string) Store {
	if root == "" {
		root = "."
	}
	return Store{
		root: root,
		clock: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// WithClock returns a copy of the store using the provided clock.
func (s Store) WithClock(clock func() time.Time) Store {
	if clock == nil {
		return s
	}
	s.clock = clock
	return s
}

// ParseEnvironment validates and returns a supported environment.
func ParseEnvironment(value string) (Environment, error) {
	env := Environment(value)
	switch env {
	case EnvironmentDevelopment, EnvironmentStaging, EnvironmentProduction:
		return env, nil
	default:
		return "", fmt.Errorf("unsupported environment %q: use development, staging, or production", value)
	}
}

// ValidateSecretName checks the public identifier used to reference a secret.
func ValidateSecretName(name string) error {
	if !secretNamePattern.MatchString(name) {
		return fmt.Errorf("invalid secret name %q: use uppercase letters, numbers, and underscores, starting with a letter", name)
	}
	return nil
}

// Paths returns the filesystem paths for env.
func (s Store) Paths(env Environment) Paths {
	envName := string(env)
	return Paths{
		SecretFile:    filepath.Join(s.root, "configs", "secrets", envName+".age.yaml"),
		RecipientFile: filepath.Join(s.root, "configs", "secrets", "recipients", envName+".txt"),
		KeyFile:       filepath.Join(s.root, ".dygo", "secrets", "keys", envName+".agekey"),
		TempDir:       filepath.Join(s.root, ".dygo", "secrets", "tmp"),
	}
}

// Init creates a new key, recipient, and empty encrypted secret document.
func (s Store) Init(env Environment, force bool) (Paths, error) {
	if _, err := ParseEnvironment(string(env)); err != nil {
		return Paths{}, err
	}

	paths := s.Paths(env)
	if !force {
		for _, path := range []string{paths.SecretFile, paths.RecipientFile, paths.KeyFile} {
			if exists(path) {
				return Paths{}, fmt.Errorf("%s already exists; rerun with --force to replace it", path)
			}
		}
	}

	identity, err := age.GenerateHybridIdentity()
	if err != nil {
		return Paths{}, fmt.Errorf("generate age identity: %w", err)
	}

	keyData := fmt.Sprintf("# dygo %s secrets identity\n# public recipient: %s\n%s\n", env, identity.Recipient(), identity)
	recipientData := fmt.Sprintf("# dygo %s secrets recipient\n%s\n", env, identity.Recipient())

	doc := NewDocument(env)
	plaintext, err := EncodeDocument(doc)
	if err != nil {
		return Paths{}, err
	}
	ciphertext, err := encryptArmored(plaintext, []age.Recipient{identity.Recipient()})
	if err != nil {
		return Paths{}, err
	}

	if err := writeFileAtomic(paths.KeyFile, []byte(keyData), 0o600); err != nil {
		return Paths{}, fmt.Errorf("write identity key: %w", err)
	}
	if err := writeFileAtomic(paths.RecipientFile, []byte(recipientData), 0o644); err != nil {
		return Paths{}, fmt.Errorf("write recipient file: %w", err)
	}
	if err := writeFileAtomic(paths.SecretFile, ciphertext, 0o644); err != nil {
		return Paths{}, fmt.Errorf("write encrypted secrets file: %w", err)
	}

	return paths, nil
}

// RotateKey replaces the local identity and committed recipient for env.
func (s Store) RotateKey(env Environment, force bool) (Paths, error) {
	var doc Document
	var err error
	if force {
		doc, err = s.Load(env)
		if err != nil {
			doc = NewDocument(env)
		}
	} else {
		doc, err = s.Load(env)
		if err != nil {
			return Paths{}, fmt.Errorf("load existing secrets before rotating key: %w", err)
		}
	}

	paths := s.Paths(env)
	identity, err := age.GenerateHybridIdentity()
	if err != nil {
		return Paths{}, fmt.Errorf("generate age identity: %w", err)
	}

	keyData := fmt.Sprintf("# dygo %s secrets identity\n# public recipient: %s\n%s\n", env, identity.Recipient(), identity)
	recipientData := fmt.Sprintf("# dygo %s secrets recipient\n%s\n", env, identity.Recipient())
	plaintext, err := EncodeDocument(doc)
	if err != nil {
		return Paths{}, err
	}
	ciphertext, err := encryptArmored(plaintext, []age.Recipient{identity.Recipient()})
	if err != nil {
		return Paths{}, err
	}

	if err := writeFileAtomic(paths.KeyFile, []byte(keyData), 0o600); err != nil {
		return Paths{}, fmt.Errorf("write identity key: %w", err)
	}
	if err := writeFileAtomic(paths.RecipientFile, []byte(recipientData), 0o644); err != nil {
		return Paths{}, fmt.Errorf("write recipient file: %w", err)
	}
	if err := writeFileAtomic(paths.SecretFile, ciphertext, 0o644); err != nil {
		return Paths{}, fmt.Errorf("write encrypted secrets file: %w", err)
	}

	return paths, nil
}

// NewDocument returns an empty secret document for env.
func NewDocument(env Environment) Document {
	return Document{
		Version:     documentVersion,
		Environment: env,
		Secrets:     map[string]Secret{},
	}
}

// Load decrypts and validates the document for env.
func (s Store) Load(env Environment) (Document, error) {
	paths := s.Paths(env)

	ciphertext, err := os.ReadFile(paths.SecretFile)
	if err != nil {
		return Document{}, fmt.Errorf("read encrypted secrets file: %w", err)
	}
	identities, err := loadIdentities(paths.KeyFile)
	if err != nil {
		return Document{}, err
	}
	plaintext, err := decryptArmored(ciphertext, identities)
	if err != nil {
		return Document{}, fmt.Errorf("decrypt secrets for %s: %w", env, err)
	}
	doc, err := DecodeDocument(plaintext, env)
	if err != nil {
		return Document{}, err
	}
	return doc, nil
}

// Save validates and encrypts doc for env.
func (s Store) Save(env Environment, doc Document) error {
	if err := doc.Validate(env); err != nil {
		return err
	}

	paths := s.Paths(env)
	recipients, err := loadRecipients(paths.RecipientFile)
	if err != nil {
		return err
	}
	plaintext, err := EncodeDocument(doc)
	if err != nil {
		return err
	}
	ciphertext, err := encryptArmored(plaintext, recipients)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(paths.SecretFile, ciphertext, 0o644); err != nil {
		return fmt.Errorf("write encrypted secrets file: %w", err)
	}
	return nil
}

// Plaintext returns the decrypted YAML document bytes for env.
func (s Store) Plaintext(env Environment) ([]byte, error) {
	doc, err := s.Load(env)
	if err != nil {
		return nil, err
	}
	return EncodeDocument(doc)
}

// SavePlaintext validates, parses, and encrypts plaintext YAML for env.
func (s Store) SavePlaintext(env Environment, plaintext []byte) error {
	doc, err := DecodeDocument(plaintext, env)
	if err != nil {
		return err
	}
	return s.Save(env, doc)
}

// Set stores one secret value.
func (s Store) Set(env Environment, name, value string) error {
	if err := ValidateSecretName(name); err != nil {
		return err
	}
	doc, err := s.Load(env)
	if err != nil {
		return err
	}
	doc.Secrets[name] = Secret{
		Value:     value,
		UpdatedAt: s.clock().Format(time.RFC3339),
	}
	return s.Save(env, doc)
}

// Get returns one raw secret.
func (s Store) Get(env Environment, name string) (Secret, error) {
	if err := ValidateSecretName(name); err != nil {
		return Secret{}, err
	}
	doc, err := s.Load(env)
	if err != nil {
		return Secret{}, err
	}
	secret, ok := doc.Secrets[name]
	if !ok {
		return Secret{}, fmt.Errorf("secret %q is not defined for %s", name, env)
	}
	return secret, nil
}

// Remove deletes one secret.
func (s Store) Remove(env Environment, name string) error {
	if err := ValidateSecretName(name); err != nil {
		return err
	}
	doc, err := s.Load(env)
	if err != nil {
		return err
	}
	if _, ok := doc.Secrets[name]; !ok {
		return fmt.Errorf("secret %q is not defined for %s", name, env)
	}
	delete(doc.Secrets, name)
	return s.Save(env, doc)
}

// List returns secret entries sorted by name.
func (s Store) List(env Environment) ([]Entry, error) {
	doc, err := s.Load(env)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(doc.Secrets))
	for name, secret := range doc.Secrets {
		entries = append(entries, Entry{Name: name, Secret: secret})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// Validate checks decryptability, schema, and config manifest secret references.
func (s Store) Validate(env Environment) error {
	doc, err := s.Load(env)
	if err != nil {
		return err
	}

	references, err := FindManifestReferences(filepath.Join(s.root, "configs"))
	if err != nil {
		return err
	}

	var problems []string
	for _, ref := range references {
		if _, ok := doc.Secrets[ref.SecretName]; !ok {
			problems = append(problems, fmt.Sprintf("%s references missing secret %q", ref.Path, ref.SecretName))
		}
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

// Validate checks a decrypted document against its expected environment.
func (d Document) Validate(env Environment) error {
	if _, err := ParseEnvironment(string(env)); err != nil {
		return err
	}
	if d.Version != documentVersion {
		return fmt.Errorf("invalid secrets document version %d: want %d", d.Version, documentVersion)
	}
	if d.Environment != env {
		return fmt.Errorf("invalid secrets document environment %q: want %q", d.Environment, env)
	}
	if d.Secrets == nil {
		return fmt.Errorf("secrets map is required")
	}
	for name, secret := range d.Secrets {
		if err := ValidateSecretName(name); err != nil {
			return err
		}
		if secret.UpdatedAt != "" {
			if _, err := time.Parse(time.RFC3339, secret.UpdatedAt); err != nil {
				return fmt.Errorf("secret %q has invalid updated_at: %w", name, err)
			}
		}
	}
	return nil
}

// EncodeDocument returns canonical YAML bytes for doc.
func EncodeDocument(doc Document) ([]byte, error) {
	if doc.Secrets == nil {
		doc.Secrets = map[string]Secret{}
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode secrets document: %w", err)
	}
	return out, nil
}

// DecodeDocument parses and validates plaintext YAML for env.
func DecodeDocument(data []byte, env Environment) (Document, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Document{}, fmt.Errorf("secrets document is empty")
	}
	if err := rejectDuplicateSecretNames(data); err != nil {
		return Document{}, err
	}

	var doc Document
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return Document{}, fmt.Errorf("decode secrets document: %w", err)
	}
	if doc.Secrets == nil {
		doc.Secrets = map[string]Secret{}
	}
	if err := doc.Validate(env); err != nil {
		return Document{}, err
	}
	return doc, nil
}

// Redact masks a secret value for human-facing output.
func Redact(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return "************" + value[len(value)-4:]
}

// ValidationError reports multiple validation problems together.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "secrets validation failed: " + strings.Join(e.Problems, "; ")
}

// ManifestReference is one config manifest secret reference.
type ManifestReference struct {
	Path       string
	SecretName string
}

// FindManifestReferences scans YAML configs for env entries with secret references.
func FindManifestReferences(configRoot string) ([]ManifestReference, error) {
	if !exists(configRoot) {
		return nil, nil
	}

	var references []ManifestReference
	err := filepath.WalkDir(configRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != configRoot && filepath.Base(path) == "secrets" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read config manifest %s: %w", path, err)
		}
		var node yaml.Node
		if err := yaml.Unmarshal(data, &node); err != nil {
			return fmt.Errorf("parse config manifest %s: %w", path, err)
		}
		collectManifestReferences(path, &node, &references)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return references, nil
}

func collectManifestReferences(path string, node *yaml.Node, references *[]ManifestReference) {
	if node == nil {
		return
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		collectManifestReferences(path, node.Content[0], references)
		return
	}
	if node.Kind != yaml.MappingNode {
		for _, child := range node.Content {
			collectManifestReferences(path, child, references)
		}
		return
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]
		if key.Value == "env" && value.Kind == yaml.MappingNode {
			collectEnvSecretReferences(path, value, references)
			continue
		}
		collectManifestReferences(path, value, references)
	}
}

func collectEnvSecretReferences(path string, envNode *yaml.Node, references *[]ManifestReference) {
	for i := 0; i+1 < len(envNode.Content); i += 2 {
		value := envNode.Content[i+1]
		if value.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(value.Content); j += 2 {
			key := value.Content[j]
			secret := value.Content[j+1]
			if key.Value == "secret" && secret.Kind == yaml.ScalarNode && secret.Value != "" {
				*references = append(*references, ManifestReference{
					Path:       path,
					SecretName: secret.Value,
				})
			}
		}
	}
}

func rejectDuplicateSecretNames(data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse secrets document: %w", err)
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		root = *root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("secrets document must be a mapping")
	}

	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "secrets" {
			continue
		}
		secretsNode := root.Content[i+1]
		if secretsNode.Kind != yaml.MappingNode {
			return nil
		}
		seen := map[string]struct{}{}
		for j := 0; j+1 < len(secretsNode.Content); j += 2 {
			name := secretsNode.Content[j].Value
			if _, ok := seen[name]; ok {
				return fmt.Errorf("duplicate secret name %q", name)
			}
			seen[name] = struct{}{}
		}
	}
	return nil
}

func loadRecipients(path string) ([]age.Recipient, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read recipient file: %w", err)
	}
	defer file.Close()

	recipients, err := age.ParseRecipients(file)
	if err != nil {
		return nil, fmt.Errorf("parse recipient file: %w", err)
	}
	if len(recipients) == 0 {
		return nil, fmt.Errorf("recipient file %s does not contain any recipients", path)
	}
	return recipients, nil
}

func loadIdentities(path string) ([]age.Identity, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read identity key: %w", err)
	}
	defer file.Close()

	identities, err := age.ParseIdentities(file)
	if err != nil {
		return nil, fmt.Errorf("parse identity key: %w", err)
	}
	if len(identities) == 0 {
		return nil, fmt.Errorf("identity key %s does not contain any identities", path)
	}
	return identities, nil
}

func encryptArmored(plaintext []byte, recipients []age.Recipient) ([]byte, error) {
	var encrypted bytes.Buffer
	armorWriter := armor.NewWriter(&encrypted)
	ageWriter, err := age.Encrypt(armorWriter, recipients...)
	if err != nil {
		armorWriter.Close()
		return nil, fmt.Errorf("create encrypted file: %w", err)
	}
	if _, err := ageWriter.Write(plaintext); err != nil {
		ageWriter.Close()
		armorWriter.Close()
		return nil, fmt.Errorf("encrypt plaintext: %w", err)
	}
	if err := ageWriter.Close(); err != nil {
		armorWriter.Close()
		return nil, fmt.Errorf("finish encryption: %w", err)
	}
	if err := armorWriter.Close(); err != nil {
		return nil, fmt.Errorf("finish armor encoding: %w", err)
	}
	return encrypted.Bytes(), nil
}

func decryptArmored(ciphertext []byte, identities []age.Identity) ([]byte, error) {
	armorReader := armor.NewReader(bytes.NewReader(ciphertext))
	plaintextReader, err := age.Decrypt(armorReader, identities...)
	if err != nil {
		return nil, err
	}
	plaintext, err := io.ReadAll(plaintextReader)
	if err != nil {
		return nil, fmt.Errorf("read decrypted plaintext: %w", err)
	}
	return plaintext, nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}

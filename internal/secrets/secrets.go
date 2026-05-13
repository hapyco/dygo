package secrets

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
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

// Paths contains the filesystem locations used by encrypted secrets.
type Paths struct {
	SecretFile    string
	MasterKeyFile string
	TempDir       string
}

// Store manages encrypted dygo secret files below a repository root.
type Store struct {
	root    string
	clock   func() time.Time
	fileOps fileOperations
}

type fileOperations struct {
	writeFileAtomic func(path string, data []byte, perm os.FileMode) error
	rename          func(oldPath string, newPath string) error
	remove          func(path string) error
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

// SupportedEnvironments returns the environments managed by dygo secrets.
func SupportedEnvironments() []Environment {
	return []Environment{
		EnvironmentDevelopment,
		EnvironmentStaging,
		EnvironmentProduction,
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
		MasterKeyFile: filepath.Join(s.root, "master.key"),
		TempDir:       filepath.Join(s.root, ".dygo", "secrets", "tmp"),
	}
}

// Init creates a root master key and encrypted secret files for all environments.
func (s Store) Init(force bool) (Paths, error) {
	paths := s.Paths(EnvironmentDevelopment)
	envs := SupportedEnvironments()

	if !force {
		masterExists := exists(paths.MasterKeyFile)
		var identity *age.HybridIdentity
		var keyData []byte
		var err error
		if masterExists {
			identity, err = loadMasterIdentity(paths.MasterKeyFile)
			if err != nil {
				return Paths{}, err
			}
		} else {
			identity, keyData, err = generateMasterKeyData()
			if err != nil {
				return Paths{}, err
			}
		}

		docs := make(map[Environment]Document, len(envs))
		for _, env := range envs {
			envPaths := s.Paths(env)
			if exists(envPaths.SecretFile) {
				if !masterExists {
					return Paths{}, fmt.Errorf("%s already exists; rerun with --force to migrate it to master.key", envPaths.SecretFile)
				}
				continue
			}
			docs[env] = NewDocument(env)
		}

		encrypted, err := encryptDocuments(docs, identity.Recipient())
		if err != nil {
			return Paths{}, err
		}
		if !masterExists {
			if err := writeFileAtomic(paths.MasterKeyFile, keyData, 0o600); err != nil {
				return Paths{}, fmt.Errorf("write master key: %w", err)
			}
		}
		for _, env := range envs {
			ciphertext, ok := encrypted[env]
			if !ok {
				continue
			}
			if err := writeFileAtomic(s.Paths(env).SecretFile, ciphertext, 0o644); err != nil {
				return Paths{}, fmt.Errorf("write encrypted %s secrets file: %w", env, err)
			}
		}
		return paths, nil
	}

	docs := make(map[Environment]Document, len(envs))
	for _, env := range envs {
		doc, err := s.loadForRewrite(env)
		if err != nil {
			if !force || exists(s.Paths(env).SecretFile) {
				return Paths{}, fmt.Errorf("load existing %s secrets: %w", env, err)
			}
			doc = NewDocument(env)
		}
		docs[env] = doc
	}

	identity, keyData, err := generateMasterKeyData()
	if err != nil {
		return Paths{}, err
	}
	encrypted, err := encryptDocuments(docs, identity.Recipient())
	if err != nil {
		return Paths{}, err
	}

	if err := writeFileAtomic(paths.MasterKeyFile, keyData, 0o600); err != nil {
		return Paths{}, fmt.Errorf("write master key: %w", err)
	}
	for _, env := range envs {
		if err := writeFileAtomic(s.Paths(env).SecretFile, encrypted[env], 0o644); err != nil {
			return Paths{}, fmt.Errorf("write encrypted %s secrets file: %w", env, err)
		}
	}

	return paths, nil
}

// RotateKey replaces the root master key and re-encrypts all environments.
func (s Store) RotateKey() (Paths, error) {
	envs := SupportedEnvironments()
	paths := s.Paths(EnvironmentDevelopment)

	oldIdentity, err := loadMasterIdentity(paths.MasterKeyFile)
	if err != nil {
		return Paths{}, err
	}
	docs := make(map[Environment]Document, len(envs))
	for _, env := range envs {
		doc, err := s.loadWithIdentity(env, oldIdentity)
		if err != nil {
			return Paths{}, fmt.Errorf("load existing %s secrets before rotating key: %w", env, err)
		}
		docs[env] = doc
	}

	identity, keyData, err := generateMasterKeyData()
	if err != nil {
		return Paths{}, err
	}
	dualEncrypted, err := encryptDocumentsForRecipients(docs, []age.Recipient{oldIdentity.Recipient(), identity.Recipient()})
	if err != nil {
		return Paths{}, err
	}
	finalEncrypted, err := encryptDocuments(docs, identity.Recipient())
	if err != nil {
		return Paths{}, err
	}

	rotation := s.rotationPaths(envs)
	if err := s.stageRotationFiles(rotation, keyData, dualEncrypted, finalEncrypted); err != nil {
		_ = s.cleanupRotationArtifacts(rotation)
		return Paths{}, err
	}
	if err := s.verifyStagedRotation(rotation, docs, oldIdentity, identity); err != nil {
		_ = s.cleanupRotationArtifacts(rotation)
		return Paths{}, err
	}
	if err := s.backupRotationFiles(rotation); err != nil {
		_ = s.cleanupRotationArtifacts(rotation)
		return Paths{}, err
	}

	for _, env := range envs {
		if err := s.rename(rotation.DualNext[env], s.Paths(env).SecretFile); err != nil {
			rollbackErr := s.restoreRotationBackups(rotation, true)
			cleanupErr := s.cleanupRotationArtifacts(rotation)
			return Paths{}, combineRotationErrors(fmt.Errorf("replace %s secrets with dual-recipient file: %w", env, err), rollbackErr, cleanupErr)
		}
	}

	if err := s.rename(rotation.MasterNext, paths.MasterKeyFile); err != nil {
		rollbackErr := s.restoreRotationBackups(rotation, true)
		cleanupErr := s.cleanupRotationArtifacts(rotation)
		return Paths{}, combineRotationErrors(fmt.Errorf("replace master key: %w", err), rollbackErr, cleanupErr)
	}

	for _, env := range envs {
		if err := s.rename(rotation.FinalNext[env], s.Paths(env).SecretFile); err != nil {
			cleanupErr := s.cleanupRotationArtifacts(rotation)
			return Paths{}, combineRotationErrors(fmt.Errorf("finalize rotated %s secrets: %w; rotation is recoverable with the new master.key", env, err), nil, cleanupErr)
		}
	}

	for _, env := range envs {
		if _, err := s.Load(env); err != nil {
			cleanupErr := s.cleanupRotationArtifacts(rotation)
			return Paths{}, combineRotationErrors(fmt.Errorf("verify rotated %s secrets with new master.key: %w", env, err), nil, cleanupErr)
		}
	}
	if err := s.cleanupRotationArtifacts(rotation); err != nil {
		return Paths{}, fmt.Errorf("secrets key rotation completed but cleanup failed: %w", err)
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

	identity, err := loadMasterIdentity(paths.MasterKeyFile)
	if err != nil {
		return Document{}, err
	}
	return s.loadWithIdentity(env, identity)
}

func (s Store) loadWithIdentity(env Environment, identity age.Identity) (Document, error) {
	paths := s.Paths(env)

	ciphertext, err := os.ReadFile(paths.SecretFile)
	if err != nil {
		return Document{}, fmt.Errorf("read encrypted secrets file: %w", err)
	}
	plaintext, err := decryptArmored(ciphertext, []age.Identity{identity})
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
	identity, err := loadMasterIdentity(paths.MasterKeyFile)
	if err != nil {
		return err
	}
	plaintext, err := EncodeDocument(doc)
	if err != nil {
		return err
	}
	ciphertext, err := encryptArmored(plaintext, []age.Recipient{identity.Recipient()})
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

// FindManifestReferences scans YAML configs for secret references.
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
		if key.Value == "secret" && value.Kind == yaml.ScalarNode && value.Value != "" {
			*references = append(*references, ManifestReference{
				Path:       path,
				SecretName: value.Value,
			})
			continue
		}
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

func loadMasterIdentity(path string) (*age.HybridIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read master key: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		identity, err := age.ParseHybridIdentity(line)
		if err != nil {
			return nil, fmt.Errorf("parse master key: %w", err)
		}
		return identity, nil
	}
	return nil, fmt.Errorf("master key %s does not contain an age identity", path)
}

func generateMasterKeyData() (*age.HybridIdentity, []byte, error) {
	identity, err := age.GenerateHybridIdentity()
	if err != nil {
		return nil, nil, fmt.Errorf("generate age identity: %w", err)
	}
	data := fmt.Sprintf("# dygo master key\n# public recipient: %s\n%s\n", identity.Recipient(), identity)
	return identity, []byte(data), nil
}

func encryptDocuments(docs map[Environment]Document, recipient age.Recipient) (map[Environment][]byte, error) {
	return encryptDocumentsForRecipients(docs, []age.Recipient{recipient})
}

func encryptDocumentsForRecipients(docs map[Environment]Document, recipients []age.Recipient) (map[Environment][]byte, error) {
	encrypted := make(map[Environment][]byte, len(docs))
	for _, env := range SupportedEnvironments() {
		doc, ok := docs[env]
		if !ok {
			continue
		}
		plaintext, err := EncodeDocument(doc)
		if err != nil {
			return nil, err
		}
		ciphertext, err := encryptArmored(plaintext, recipients)
		if err != nil {
			return nil, fmt.Errorf("encrypt %s secrets: %w", env, err)
		}
		encrypted[env] = ciphertext
	}
	return encrypted, nil
}

type rotationFileSet struct {
	MasterNext     string
	MasterRollback string
	DualNext       map[Environment]string
	FinalNext      map[Environment]string
	SecretRollback map[Environment]string
}

func (s Store) rotationPaths(envs []Environment) rotationFileSet {
	base := filepath.Join(s.Paths(EnvironmentDevelopment).TempDir, "rotate-key")
	paths := rotationFileSet{
		MasterNext:     filepath.Join(base, "master.key.next"),
		MasterRollback: filepath.Join(base, "master.key.rollback"),
		DualNext:       make(map[Environment]string, len(envs)),
		FinalNext:      make(map[Environment]string, len(envs)),
		SecretRollback: make(map[Environment]string, len(envs)),
	}
	for _, env := range envs {
		name := string(env) + ".age.yaml"
		paths.DualNext[env] = filepath.Join(base, name+".dual.next")
		paths.FinalNext[env] = filepath.Join(base, name+".final.next")
		paths.SecretRollback[env] = filepath.Join(base, name+".rollback")
	}
	return paths
}

func (s Store) stageRotationFiles(paths rotationFileSet, keyData []byte, dualEncrypted, finalEncrypted map[Environment][]byte) error {
	if err := s.writeFileAtomic(paths.MasterNext, keyData, 0o600); err != nil {
		return fmt.Errorf("stage rotated master key: %w", err)
	}
	for _, env := range SupportedEnvironments() {
		if err := s.writeFileAtomic(paths.DualNext[env], dualEncrypted[env], 0o644); err != nil {
			return fmt.Errorf("stage dual-recipient %s secrets: %w", env, err)
		}
		if err := s.writeFileAtomic(paths.FinalNext[env], finalEncrypted[env], 0o644); err != nil {
			return fmt.Errorf("stage final %s secrets: %w", env, err)
		}
	}
	return nil
}

func (s Store) verifyStagedRotation(paths rotationFileSet, docs map[Environment]Document, oldIdentity, newIdentity *age.HybridIdentity) error {
	stagedIdentity, err := loadMasterIdentity(paths.MasterNext)
	if err != nil {
		return fmt.Errorf("verify staged master key: %w", err)
	}
	if stagedIdentity.Recipient().String() != newIdentity.Recipient().String() {
		return fmt.Errorf("verify staged master key: staged recipient does not match generated recipient")
	}
	for _, env := range SupportedEnvironments() {
		if err := verifyEncryptedDocumentFile(paths.DualNext[env], env, docs[env], []age.Identity{oldIdentity}); err != nil {
			return fmt.Errorf("verify staged dual-recipient %s secrets with old key: %w", env, err)
		}
		if err := verifyEncryptedDocumentFile(paths.DualNext[env], env, docs[env], []age.Identity{newIdentity}); err != nil {
			return fmt.Errorf("verify staged dual-recipient %s secrets with new key: %w", env, err)
		}
		if err := verifyEncryptedDocumentFile(paths.FinalNext[env], env, docs[env], []age.Identity{newIdentity}); err != nil {
			return fmt.Errorf("verify staged final %s secrets with new key: %w", env, err)
		}
	}
	return nil
}

func verifyEncryptedDocumentFile(path string, env Environment, want Document, identities []age.Identity) error {
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read staged encrypted secrets file: %w", err)
	}
	plaintext, err := decryptArmored(ciphertext, identities)
	if err != nil {
		return err
	}
	got, err := DecodeDocument(plaintext, env)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("staged secrets document changed unexpectedly")
	}
	return nil
}

func (s Store) backupRotationFiles(paths rotationFileSet) error {
	if err := s.copyExistingFile(s.Paths(EnvironmentDevelopment).MasterKeyFile, paths.MasterRollback, 0o600); err != nil {
		return fmt.Errorf("backup master key: %w", err)
	}
	for _, env := range SupportedEnvironments() {
		if err := s.copyExistingFile(s.Paths(env).SecretFile, paths.SecretRollback[env], 0o644); err != nil {
			return fmt.Errorf("backup %s secrets: %w", env, err)
		}
	}
	return nil
}

func (s Store) copyExistingFile(source string, target string, perm os.FileMode) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return s.writeFileAtomic(target, data, perm)
}

func (s Store) restoreRotationBackups(paths rotationFileSet, restoreMaster bool) error {
	var problems []string
	for _, env := range SupportedEnvironments() {
		backup := paths.SecretRollback[env]
		if !exists(backup) {
			continue
		}
		if err := s.rename(backup, s.Paths(env).SecretFile); err != nil {
			problems = append(problems, fmt.Sprintf("restore %s secrets: %v", env, err))
		}
	}
	if restoreMaster && exists(paths.MasterRollback) {
		if err := s.rename(paths.MasterRollback, s.Paths(EnvironmentDevelopment).MasterKeyFile); err != nil {
			problems = append(problems, fmt.Sprintf("restore master key: %v", err))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (s Store) cleanupRotationArtifacts(paths rotationFileSet) error {
	var problems []string
	for _, path := range paths.all() {
		if err := s.remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			problems = append(problems, fmt.Sprintf("remove %s: %v", path, err))
		}
	}
	rotateDir := filepath.Dir(paths.MasterNext)
	if err := s.remove(rotateDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		problems = append(problems, fmt.Sprintf("remove %s: %v", rotateDir, err))
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func (paths rotationFileSet) all() []string {
	files := []string{paths.MasterNext, paths.MasterRollback}
	for _, env := range SupportedEnvironments() {
		files = append(files, paths.DualNext[env], paths.FinalNext[env], paths.SecretRollback[env])
	}
	return files
}

func combineRotationErrors(primary error, rollbackErr error, cleanupErr error) error {
	if rollbackErr != nil {
		primary = fmt.Errorf("%w; rollback failed: %v", primary, rollbackErr)
	}
	if cleanupErr != nil {
		primary = fmt.Errorf("%w; cleanup failed: %v", primary, cleanupErr)
	}
	return primary
}

func (s Store) writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if s.fileOps.writeFileAtomic != nil {
		return s.fileOps.writeFileAtomic(path, data, perm)
	}
	return writeFileAtomic(path, data, perm)
}

func (s Store) rename(oldPath string, newPath string) error {
	if s.fileOps.rename != nil {
		return s.fileOps.rename(oldPath, newPath)
	}
	return os.Rename(oldPath, newPath)
}

func (s Store) remove(path string) error {
	if s.fileOps.remove != nil {
		return s.fileOps.remove(path)
	}
	return os.Remove(path)
}

func (s Store) loadForRewrite(env Environment) (Document, error) {
	paths := s.Paths(env)
	if !exists(paths.SecretFile) {
		return NewDocument(env), nil
	}

	if exists(paths.MasterKeyFile) {
		doc, err := s.Load(env)
		if err == nil {
			return doc, nil
		}
	}

	legacyKeyFile := filepath.Join(s.root, ".dygo", "secrets", "keys", string(env)+".agekey")
	if exists(legacyKeyFile) {
		doc, err := loadLegacyDocument(paths.SecretFile, legacyKeyFile, env)
		if err == nil {
			return doc, nil
		}
		return Document{}, err
	}

	return Document{}, fmt.Errorf("encrypted secrets file exists but no usable master.key or legacy environment key was found")
}

func loadLegacyDocument(secretFile string, keyFile string, env Environment) (Document, error) {
	ciphertext, err := os.ReadFile(secretFile)
	if err != nil {
		return Document{}, fmt.Errorf("read encrypted secrets file: %w", err)
	}
	identities, err := loadIdentities(keyFile)
	if err != nil {
		return Document{}, err
	}
	plaintext, err := decryptArmored(ciphertext, identities)
	if err != nil {
		return Document{}, fmt.Errorf("decrypt legacy secrets for %s: %w", env, err)
	}
	doc, err := DecodeDocument(plaintext, env)
	if err != nil {
		return Document{}, err
	}
	return doc, nil
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

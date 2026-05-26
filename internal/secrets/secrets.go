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
	"sort"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/yamlmeta"
	"gopkg.in/yaml.v3"
)

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentStaging     Environment = "staging"
	EnvironmentProduction  Environment = "production"
)

// Environment names a supported dygo runtime environment.
type Environment string

// Secret contains one resolved secret value.
type Secret struct {
	Value string
}

// Document is the decrypted YAML payload encrypted on disk.
type Document struct {
	Values map[string]any
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
	return Store{root: root}
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

// SupportedEnvironments returns the environments managed by dygo secret.
func SupportedEnvironments() []Environment {
	return []Environment{
		EnvironmentDevelopment,
		EnvironmentStaging,
		EnvironmentProduction,
	}
}

// ValidateSecretName checks the public path used to reference a secret.
func ValidateSecretName(name string) error {
	if name == "" || strings.TrimSpace(name) != name {
		return fmt.Errorf("invalid secret name %q: use a non-empty root key or dot-separated path", name)
	}
	for _, segment := range strings.Split(name, ".") {
		if segment == "" {
			return fmt.Errorf("invalid secret name %q: path segments cannot be empty", name)
		}
	}
	return nil
}

// Paths returns the filesystem paths for env.
func (s Store) Paths(env Environment) Paths {
	envName := string(env)
	return Paths{
		SecretFile:    filepath.Join(s.root, filepath.FromSlash(shape.ConfigSecretsDir), envName+".yml.age"),
		MasterKeyFile: filepath.Join(s.root, filepath.FromSlash(shape.LocalSecretKeyFile)),
		TempDir:       filepath.Join(s.root, filepath.FromSlash(shape.LocalSecretsTempDir)),
	}
}

// Init creates a root master key and encrypted secret files for all environments.
func (s Store) Init() (Paths, error) {
	paths := s.Paths(EnvironmentDevelopment)
	envs := SupportedEnvironments()

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
				return Paths{}, fmt.Errorf("%s already exists but %s is missing; restore the key or remove the stale encrypted file", envPaths.SecretFile, paths.MasterKeyFile)
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
		Values: map[string]any{},
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
	if err := doc.Validate(); err != nil {
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
	doc.Set(name, value)
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
	value, ok, err := doc.SecretValue(name)
	if err != nil {
		return Secret{}, err
	}
	if !ok {
		return Secret{}, fmt.Errorf("secret %q is not defined for %s", name, env)
	}
	return Secret{Value: value}, nil
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
	if ok := doc.Remove(name); !ok {
		return fmt.Errorf("secret %q is not defined for %s", name, env)
	}
	return s.Save(env, doc)
}

// List returns secret entries sorted by name.
func (s Store) List(env Environment) ([]Entry, error) {
	doc, err := s.Load(env)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0)
	for name, secret := range doc.Flatten() {
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

	references, err := FindProjectReferences(s.root)
	if err != nil {
		return err
	}

	var problems []string
	for _, ref := range references {
		_, ok, err := doc.SecretValue(ref.SecretName)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s references non-scalar secret %q", ref.Path, ref.SecretName))
			continue
		}
		if !ok {
			problems = append(problems, fmt.Sprintf("%s references missing secret %q", ref.Path, ref.SecretName))
		}
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

// Validate checks a decrypted plain YAML document.
func (d Document) Validate() error {
	if d.Values == nil {
		return fmt.Errorf("secrets document must be a mapping")
	}
	return nil
}

// EncodeDocument returns canonical YAML bytes for doc.
func EncodeDocument(doc Document) ([]byte, error) {
	if doc.Values == nil {
		doc.Values = map[string]any{}
	}
	out, err := yaml.Marshal(doc.Values)
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
	root, err := yamlmeta.Parse(data, "parse secrets document")
	if err != nil {
		return Document{}, err
	}
	if err := yamlmeta.RejectDuplicateKeys(&root, func(duplicate yamlmeta.DuplicateKey) error {
		return fmt.Errorf("duplicate secret key %q", secretDuplicatePath(duplicate.Location))
	}); err != nil {
		return Document{}, err
	}
	values, err := decodeRootMapping(&root)
	if err != nil {
		return Document{}, err
	}
	return Document{Values: values}, nil
}

// Set stores a scalar value at a root key or dot-separated path.
func (d *Document) Set(path string, value string) {
	if d.Values == nil {
		d.Values = map[string]any{}
	}
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		d.Values[path] = value
		return
	}
	current := d.Values
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

// SecretValue resolves a root key or dot-separated path to a scalar string value.
func (d Document) SecretValue(path string) (string, bool, error) {
	if err := ValidateSecretName(path); err != nil {
		return "", false, err
	}
	value, ok := d.Lookup(path)
	if !ok {
		return "", false, nil
	}
	scalar, ok := scalarSecretValue(value)
	if !ok {
		return "", true, fmt.Errorf("secret %q must resolve to a scalar value", path)
	}
	return scalar, true, nil
}

// Lookup resolves an exact root key first, then a dot-separated nested path.
func (d Document) Lookup(path string) (any, bool) {
	if d.Values == nil {
		return nil, false
	}
	if value, ok := d.Values[path]; ok {
		return value, true
	}
	parts := strings.Split(path, ".")
	var current any = d.Values
	for _, part := range parts {
		mapping, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = mapping[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// Remove deletes a root key or dot-separated path.
func (d *Document) Remove(path string) bool {
	if d.Values == nil {
		return false
	}
	if _, ok := d.Values[path]; ok {
		delete(d.Values, path)
		return true
	}
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		return false
	}
	current := d.Values
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	last := parts[len(parts)-1]
	if _, ok := current[last]; !ok {
		return false
	}
	delete(current, last)
	return true
}

// Flatten returns scalar secrets by their root keys and dot-separated paths.
func (d Document) Flatten() map[string]Secret {
	out := map[string]Secret{}
	flattenSecrets(out, "", d.Values)
	return out
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

// FindProjectReferences scans root dygo.yml and config YAML files for secret references.
func FindProjectReferences(root string) ([]ManifestReference, error) {
	var references []ManifestReference
	projectConfigReferences, err := FindManifestFileReferences(filepath.Join(root, filepath.FromSlash(shape.ProjectConfigFile)))
	if err != nil {
		return nil, err
	}
	references = append(references, projectConfigReferences...)

	configReferences, err := FindManifestReferences(filepath.Join(root, filepath.FromSlash(shape.ConfigDir)))
	if err != nil {
		return nil, err
	}
	references = append(references, configReferences...)
	return references, nil
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

		fileReferences, err := FindManifestFileReferences(path)
		if err != nil {
			return err
		}
		references = append(references, fileReferences...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return references, nil
}

// FindManifestFileReferences scans one YAML manifest for secret references.
func FindManifestFileReferences(path string) ([]ManifestReference, error) {
	if !exists(path) {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config manifest %s: %w", path, err)
	}
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parse config manifest %s: %w", path, err)
	}
	var references []ManifestReference
	collectManifestReferences(path, &node, &references)
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

func decodeRootMapping(root *yaml.Node) (map[string]any, error) {
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return nil, fmt.Errorf("secrets document is empty")
		}
		root = root.Content[0]
	}
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("secrets document must be a mapping")
	}
	value, err := decodeYAMLNode(root)
	if err != nil {
		return nil, err
	}
	mapping, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("secrets document must be a mapping")
	}
	return mapping, nil
}

func decodeYAMLNode(node *yaml.Node) (any, error) {
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil, nil
		}
		return decodeYAMLNode(node.Content[0])
	case yaml.MappingNode:
		mapping := make(map[string]any, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Value == "" {
				return nil, fmt.Errorf("secrets document mapping keys must be non-empty strings")
			}
			value, err := decodeYAMLNode(node.Content[i+1])
			if err != nil {
				return nil, err
			}
			mapping[key.Value] = value
		}
		return mapping, nil
	case yaml.SequenceNode:
		values := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := decodeYAMLNode(child)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	case yaml.ScalarNode:
		var value any
		if err := node.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode secrets scalar: %w", err)
		}
		return value, nil
	default:
		return nil, nil
	}
}

func scalarSecretValue(value any) (string, bool) {
	switch v := value.(type) {
	case nil:
		return "", true
	case string:
		return v, true
	case bool:
		return fmt.Sprint(v), true
	case int:
		return fmt.Sprint(v), true
	case int64:
		return fmt.Sprint(v), true
	case uint64:
		return fmt.Sprint(v), true
	case float64:
		return fmt.Sprint(v), true
	default:
		return "", false
	}
}

func flattenSecrets(out map[string]Secret, prefix string, value any) {
	mapping, ok := value.(map[string]any)
	if !ok {
		if scalar, scalarOK := scalarSecretValue(value); scalarOK && prefix != "" {
			out[prefix] = Secret{Value: scalar}
		}
		return
	}
	for key, child := range mapping {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		flattenSecrets(out, path, child)
	}
}

func secretDuplicatePath(location string) string {
	location = strings.TrimPrefix(location, "$.")
	return strings.TrimPrefix(location, "$")
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
		name := string(env) + ".yml.age"
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

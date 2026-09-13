// Package recordsecret owns Record encryption and its environment key ring.
package recordsecret

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"filippo.io/age"
	"github.com/hapyco/dygo/internal/secrets"
)

const configKey = "_record_encryption"

var ErrUnavailable = errors.New("Record encryption key is unavailable")
var ErrInvalid = errors.New("Record secret ciphertext is invalid")

type Ring struct {
	Active   string            `json:"active"`
	Keys     map[string]string `json:"keys"`
	Rotating bool              `json:"rotating,omitempty"`
}
type envelope struct {
	Version int    `json:"v"`
	Key     string `json:"key"`
	Data    string `json:"data"`
}
type payload struct {
	Binding string `json:"binding"`
	Value   string `json:"value"`
}
type contextKey struct{}
type operationKey struct{}
type operationCache struct {
	once sync.Once
	ring Ring
	err  error
}
type Provider func() (Ring, error)

func WithProvider(ctx context.Context, provider Provider) context.Context {
	return context.WithValue(ctx, contextKey{}, provider)
}
func WithStore(ctx context.Context, store secrets.Store, env secrets.Environment) context.Context {
	return WithProvider(ctx, func() (Ring, error) { return Load(store, env) })
}

// WithOperation creates a per-request or per-mutation key cache. The provider
// still resolves the encrypted credential store at each operation boundary, so
// a key rotation is observed by the next operation without rereading it for
// every secret field in one mutation.
func WithOperation(ctx context.Context) context.Context {
	if _, ok := ctx.Value(operationKey{}).(*operationCache); ok {
		return ctx
	}
	return context.WithValue(ctx, operationKey{}, &operationCache{})
}

func FromContext(ctx context.Context) (Ring, error) {
	p, ok := ctx.Value(contextKey{}).(Provider)
	if !ok {
		return Ring{}, ErrUnavailable
	}
	if cache, ok := ctx.Value(operationKey{}).(*operationCache); ok {
		cache.once.Do(func() {
			cache.ring, cache.err = p()
		})
		return cache.ring, cache.err
	}
	return p()
}
func Load(store secrets.Store, env secrets.Environment) (Ring, error) {
	doc, err := store.Load(env)
	if err != nil {
		return Ring{}, ErrUnavailable
	}
	value, ok := doc.Values[configKey]
	if !ok {
		return Ring{}, ErrUnavailable
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return Ring{}, ErrUnavailable
	}
	var ring Ring
	if json.Unmarshal(raw, &ring) != nil || ring.validate() != nil {
		return Ring{}, ErrUnavailable
	}
	return ring, nil
}
func (r Ring) validate() error {
	if r.Active == "" || r.Keys[r.Active] == "" {
		return ErrUnavailable
	}
	for id, key := range r.Keys {
		identity, err := age.ParseX25519Identity(key)
		if err != nil || keyID(identity) != id {
			return ErrUnavailable
		}
	}
	return nil
}
func save(store secrets.Store, env secrets.Environment, ring Ring) error {
	doc, err := store.Load(env)
	if err != nil {
		return ErrUnavailable
	}
	data, err := json.Marshal(ring)
	if err != nil {
		return ErrUnavailable
	}
	var value map[string]any
	if json.Unmarshal(data, &value) != nil {
		return ErrUnavailable
	}
	doc.Values[configKey] = value
	if err := store.Save(env, doc); err != nil {
		return fmt.Errorf("save Record encryption keys: %w", err)
	}
	return nil
}
func keyID(identity *age.X25519Identity) string {
	sum := sha256.Sum256([]byte(identity.Recipient().String()))
	return hex.EncodeToString(sum[:])
}
func newKey(ring *Ring) error {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return ErrUnavailable
	}
	if ring.Keys == nil {
		ring.Keys = map[string]string{}
	}
	ring.Active = keyID(identity)
	ring.Keys[ring.Active] = identity.String()
	return nil
}

// Init preserves existing keys, including malformed configuration which needs repair.
func Init(store secrets.Store, env secrets.Environment) error {
	doc, err := store.Load(env)
	if err != nil {
		return ErrUnavailable
	}
	if _, exists := doc.Values[configKey]; exists {
		_, err = Load(store, env)
		return err
	}
	ring := Ring{}
	if err = newKey(&ring); err != nil {
		return err
	}
	return save(store, env, ring)
}

// BeginRotation resumes an incomplete rotation without generating another key.
func BeginRotation(store secrets.Store, env secrets.Environment) (Ring, error) {
	ring, err := Load(store, env)
	if err != nil {
		return Ring{}, err
	}
	if ring.Rotating {
		return ring, nil
	}
	if err = newKey(&ring); err != nil {
		return Ring{}, err
	}
	ring.Rotating = true
	return ring, save(store, env, ring)
}
func FinishRotation(store secrets.Store, env secrets.Environment, ring Ring) error {
	ring.Rotating = false
	return save(store, env, ring)
}
func (r Ring) Encrypt(binding, value string) (string, error) {
	if value == "" {
		return "", errors.New("secret must not be empty")
	}
	identity, err := age.ParseX25519Identity(r.Keys[r.Active])
	if err != nil {
		return "", ErrUnavailable
	}
	raw, _ := json.Marshal(payload{Binding: binding, Value: value})
	var encrypted bytes.Buffer
	writer, err := age.Encrypt(&encrypted, identity.Recipient())
	if err != nil {
		return "", ErrUnavailable
	}
	if _, err = writer.Write(raw); err != nil {
		return "", ErrUnavailable
	}
	if writer.Close() != nil {
		return "", ErrUnavailable
	}
	data, _ := json.Marshal(envelope{Version: 1, Key: r.Active, Data: base64.StdEncoding.EncodeToString(encrypted.Bytes())})
	return string(data), nil
}
func CipherKey(ciphertext string) (string, error) {
	var e envelope
	if json.Unmarshal([]byte(ciphertext), &e) != nil || e.Version != 1 || e.Key == "" || e.Data == "" {
		return "", ErrInvalid
	}
	return e.Key, nil
}
func (r Ring) Decrypt(binding, ciphertext string) (string, error) {
	id, err := CipherKey(ciphertext)
	if err != nil {
		return "", err
	}
	identity, err := age.ParseX25519Identity(r.Keys[id])
	if err != nil {
		return "", ErrUnavailable
	}
	var e envelope
	_ = json.Unmarshal([]byte(ciphertext), &e)
	data, err := base64.StdEncoding.DecodeString(e.Data)
	if err != nil {
		return "", ErrInvalid
	}
	reader, err := age.Decrypt(bytes.NewReader(data), identity)
	if err != nil {
		return "", ErrInvalid
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", ErrInvalid
	}
	var value payload
	if json.Unmarshal(raw, &value) != nil || value.Binding != binding || value.Value == "" {
		return "", ErrInvalid
	}
	return value.Value, nil
}

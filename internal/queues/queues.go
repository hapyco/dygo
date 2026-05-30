// Package queues loads dygo queue configuration.
package queues

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/hapyco/dygo/internal/yamlmeta"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultName is the queue used when Job metadata does not name one.
	DefaultName = "default"
	// DefaultConcurrency is the generated concurrency for the default queue.
	DefaultConcurrency = 4
)

// Config is a project's queue registry.
type Config struct {
	Queues []Queue `yaml:"queues"`
}

// Queue describes one registered queue.
type Queue struct {
	Name        string `yaml:"name"`
	Concurrency int    `yaml:"concurrency"`
}

// Default returns the generated default queue configuration.
func Default() Config {
	return Config{Queues: []Queue{{Name: DefaultName, Concurrency: DefaultConcurrency}}}
}

// Load reads config/queues.yml from root.
func Load(root string) (Config, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	return LoadFile(filepath.Join(root, filepath.FromSlash(shape.ConfigQueuesFile)))
}

// LoadFile reads one queue config file.
func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read queue config %s: %w", path, err)
	}
	cfg, err := Decode(data)
	if err != nil {
		return Config{}, fmt.Errorf("load queue config %s: %w", path, err)
	}
	return cfg, nil
}

// Decode decodes and validates one queue config document.
func Decode(data []byte) (Config, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return Config{}, err
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil && err != io.EOF {
		return Config{}, fmt.Errorf("decode queue config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks queue names and concurrency.
func (c Config) Validate() error {
	var problems []string

	seen := map[string]struct{}{}
	hasDefault := false
	for index, queue := range c.Queues {
		name := strings.TrimSpace(queue.Name)
		label := fmt.Sprintf("queues[%d]", index)
		if name == "" {
			problems = append(problems, label+".name is required")
		} else if !fieldtype.IsName(name) {
			problems = append(problems, fmt.Sprintf("%s.name %q must be kebab-case", label, queue.Name))
		} else {
			if _, ok := seen[name]; ok {
				problems = append(problems, fmt.Sprintf("duplicate queue %q", name))
			}
			seen[name] = struct{}{}
			if name == DefaultName {
				hasDefault = true
			}
		}
		if queue.Concurrency < 1 {
			problems = append(problems, fmt.Sprintf("%s.concurrency must be greater than 0", label))
		}
	}
	if !hasDefault {
		problems = append(problems, "queues must include default queue")
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

// Has reports whether name is registered.
func (c Config) Has(name string) bool {
	name = strings.TrimSpace(name)
	for _, queue := range c.Queues {
		if queue.Name == name {
			return true
		}
	}
	return false
}

// Names returns registered queue names in stable order.
func (c Config) Names() []string {
	names := make([]string, 0, len(c.Queues))
	for _, queue := range c.Queues {
		names = append(names, queue.Name)
	}
	sort.Strings(names)
	return names
}

// ValidationError reports one or more queue config problems.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "queue config validation failed: " + strings.Join(e.Problems, "; ")
}

func rejectDuplicateKeys(data []byte) error {
	root, err := yamlmeta.Parse(data, "parse queue config")
	if err != nil {
		return err
	}
	return yamlmeta.RejectDuplicateKeys(&root, func(duplicate yamlmeta.DuplicateKey) error {
		return fmt.Errorf("duplicate queue config key %q at line %d, previously defined at line %d", duplicate.Location, duplicate.Line, duplicate.PreviousLine)
	})
}

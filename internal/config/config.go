package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultServerHost = "127.0.0.1"
	defaultServerPort = 6790

	// FilePath is the project-relative dygo runtime config path.
	FilePath = "configs/dygo.yaml"
)

// Config contains dygo runtime settings.
type Config struct {
	Server Server
}

// Server contains HTTP server settings.
type Server struct {
	Host string
	Port int
}

// Load reads, decodes, and validates the dygo project config from root.
func Load(root string) (Config, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	return LoadFile(filepath.Join(root, filepath.FromSlash(FilePath)))
}

// LoadFile reads, decodes, and validates one dygo config file.
func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read dygo config %s: %w", path, err)
	}
	cfg, err := Decode(data)
	if err != nil {
		return Config{}, fmt.Errorf("load dygo config %s: %w", path, err)
	}
	return cfg, nil
}

// Decode decodes and validates one dygo config document.
func Decode(data []byte) (Config, error) {
	if err := rejectDuplicateKeys(data); err != nil {
		return Config{}, err
	}

	cfg := Default()
	var raw rawConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil && err != io.EOF {
		return Config{}, fmt.Errorf("decode dygo config: %w", err)
	}
	raw.apply(&cfg)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Default returns the built-in dygo configuration.
func Default() Config {
	return Config{
		Server: Server{
			Host: defaultServerHost,
			Port: defaultServerPort,
		},
	}
}

// Validate checks a resolved dygo config.
func (c Config) Validate() error {
	var problems []string
	if strings.TrimSpace(c.Server.Host) == "" {
		problems = append(problems, "server.host is required")
	}
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		problems = append(problems, fmt.Sprintf("server.port must be between 1 and 65535, got %d", c.Server.Port))
	}
	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

// Address returns the host:port pair used by the server.
func (s Server) Address() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// ValidationError reports one or more config validation problems.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "dygo config validation failed: " + strings.Join(e.Problems, "; ")
}

type rawConfig struct {
	Server *rawServer `yaml:"server,omitempty"`
}

type rawServer struct {
	Host *string `yaml:"host,omitempty"`
	Port *int    `yaml:"port,omitempty"`
}

func (r rawConfig) apply(cfg *Config) {
	if r.Server == nil {
		return
	}
	if r.Server.Host != nil {
		cfg.Server.Host = *r.Server.Host
	}
	if r.Server.Port != nil {
		cfg.Server.Port = *r.Server.Port
	}
}

func rejectDuplicateKeys(data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse dygo config: %w", err)
	}
	return rejectDuplicateKeysNode(&root, "$")
}

func rejectDuplicateKeysNode(node *yaml.Node, location string) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			if err := rejectDuplicateKeysNode(child, location); err != nil {
				return err
			}
		}
		return nil
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]int{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			keyLocation := location + "." + key.Value
			if previousLine, ok := seen[key.Value]; ok {
				return fmt.Errorf("duplicate config key %q at line %d, previously defined at line %d", keyLocation, key.Line, previousLine)
			}
			seen[key.Value] = key.Line
			if err := rejectDuplicateKeysNode(value, keyLocation); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range node.Content {
		if err := rejectDuplicateKeysNode(child, location); err != nil {
			return err
		}
	}
	return nil
}

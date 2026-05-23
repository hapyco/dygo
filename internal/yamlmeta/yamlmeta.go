// Package yamlmeta contains shared helpers for authored YAML metadata.
package yamlmeta

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DuplicateKey describes one duplicate mapping key found in a YAML document.
type DuplicateKey struct {
	Key          string
	Location     string
	Line         int
	PreviousLine int
}

// Parse decodes data into a YAML syntax tree for pre-struct validation.
func Parse(data []byte, context string) (yaml.Node, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return yaml.Node{}, fmt.Errorf("%s: %w", context, err)
	}
	return root, nil
}

// RejectDuplicateKeys walks node and reports duplicate mapping keys.
func RejectDuplicateKeys(node *yaml.Node, duplicate func(DuplicateKey) error) error {
	if node == nil {
		return nil
	}
	return rejectDuplicateKeysNode(node, "$", duplicate)
}

func rejectDuplicateKeysNode(node *yaml.Node, location string, duplicate func(DuplicateKey) error) error {
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := rejectDuplicateKeysNode(child, location, duplicate); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for index, child := range node.Content {
			if err := rejectDuplicateKeysNode(child, fmt.Sprintf("%s[%d]", location, index), duplicate); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		seen := map[string]int{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			keyLocation := location + "." + key.Value
			if previous, ok := seen[key.Value]; ok {
				return duplicate(DuplicateKey{Key: key.Value, Location: keyLocation, Line: key.Line, PreviousLine: previous})
			}
			seen[key.Value] = key.Line
			if err := rejectDuplicateKeysNode(value, keyLocation, duplicate); err != nil {
				return err
			}
		}
	}
	return nil
}

// DocumentMapping returns the document's top-level mapping, if present.
func DocumentMapping(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return DocumentMapping(node.Content[0])
	}
	return ValueMapping(node)
}

// ValueMapping returns node when it is a mapping.
func ValueMapping(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.MappingNode {
		return node
	}
	return nil
}

// ScalarString returns a trimmed string scalar.
func ScalarString(node *yaml.Node, name string) (string, error) {
	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("%s must be a string at line %d", name, node.Line)
	}
	return strings.TrimSpace(node.Value), nil
}

// ScalarStringSequence returns a sequence of trimmed string scalars.
func ScalarStringSequence(node *yaml.Node, name string) ([]string, error) {
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s must be a sequence at line %d", name, node.Line)
	}
	values := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		value, err := ScalarString(item, name)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

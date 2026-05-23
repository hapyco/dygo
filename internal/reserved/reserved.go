// Package reserved exposes framework-owned reserved words.
package reserved

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed words.yml
var wordsYAML []byte

type wordSet struct {
	Slugs    []string `yaml:"slugs"`
	Fields   []string `yaml:"fields"`
	Queries  []string `yaml:"queries"`
	Entities []string `yaml:"entities"`
}

var defaultWords = mustLoadWords(wordsYAML)

// IsSlug reports whether value is reserved in root route slug space.
func IsSlug(value string) bool {
	return contains(defaultWords.Slugs, value)
}

// IsField reports whether value is reserved in authored field-name space.
func IsField(value string) bool {
	return contains(defaultWords.Fields, value)
}

// IsQuery reports whether value is reserved in Record list query-param space.
func IsQuery(value string) bool {
	return contains(defaultWords.Queries, value)
}

// IsEntity reports whether value is reserved in Entity-name space.
func IsEntity(value string) bool {
	return contains(defaultWords.Entities, value)
}

// Slugs returns reserved root route slugs in stable order.
func Slugs() []string {
	return sortedCopy(defaultWords.Slugs)
}

// Fields returns reserved field names in stable order.
func Fields() []string {
	return sortedCopy(defaultWords.Fields)
}

// Queries returns reserved query parameters in stable order.
func Queries() []string {
	return sortedCopy(defaultWords.Queries)
}

// Entities returns reserved Entity names in stable order.
func Entities() []string {
	return sortedCopy(defaultWords.Entities)
}

func mustLoadWords(data []byte) wordSet {
	words, err := loadWords(data)
	if err != nil {
		panic(err)
	}
	return words
}

func loadWords(data []byte) (wordSet, error) {
	var words wordSet
	if err := yaml.Unmarshal(data, &words); err != nil {
		return wordSet{}, fmt.Errorf("load reserved words: %w", err)
	}
	normalizeCategory(&words.Slugs)
	normalizeCategory(&words.Fields)
	normalizeCategory(&words.Queries)
	normalizeCategory(&words.Entities)
	return words, nil
}

func normalizeCategory(values *[]string) {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(*values))
	for _, value := range *values {
		word := normalize(value)
		if word == "" || seen[word] {
			continue
		}
		seen[word] = true
		normalized = append(normalized, word)
	}
	sort.Strings(normalized)
	*values = normalized
}

func contains(values []string, value string) bool {
	word := normalize(value)
	if word == "" {
		return false
	}
	index := sort.SearchStrings(values, word)
	return index < len(values) && values[index] == word
}

func sortedCopy(values []string) []string {
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

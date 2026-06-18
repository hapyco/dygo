// Package schema loads and validates dygo Entity metadata.
package schema

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/internal/reserved"
	"github.com/hapyco/dygo/internal/yamlmeta"
	"gopkg.in/yaml.v3"
)

// Entity describes one dygo business object definition.
type Entity struct {
	Line         int          `yaml:"-"`
	Name         string       `yaml:"-"`
	Label        string       `yaml:"label"`
	Description  string       `yaml:"description,omitempty"`
	Icon         string       `yaml:"icon,omitempty"`
	IsSingle     bool         `yaml:"is-single,omitempty"`
	IsSystem     bool         `yaml:"is-system,omitempty"`
	IsCollection bool         `yaml:"-"`
	Route        Route        `yaml:"route,omitempty"`
	Naming       Naming       `yaml:"name,omitempty"`
	Fields       []Field      `yaml:"fields"`
	Indexes      []Index      `yaml:"indexes,omitempty"`
	Constraints  []Constraint `yaml:"constraints,omitempty"`
}

// Route describes the user-facing Studio route metadata for an Entity.
type Route struct {
	Line int    `yaml:"-"`
	Slug string `yaml:"slug,omitempty"`
}

// Naming describes how dygo assigns the system-owned Record name.
type Naming struct {
	Line     int    `yaml:"-"`
	Strategy string `yaml:"strategy,omitempty"`
	Label    string `yaml:"label,omitempty"`
	Length   int    `yaml:"length,omitempty"`
	Pattern  string `yaml:"pattern,omitempty"`
	Format   string `yaml:"format,omitempty"`
}

const (
	NamingStrategyManual = "manual"
	NamingStrategyRandom = "random"
	NamingStrategySeries = "series"
	NamingStrategyFormat = "format"

	DefaultRandomNameLength = 16
	CollectionRowNameLength = 16
	MinRandomNameLength     = 8
	MaxRandomNameLength     = 64
)

var (
	namingStrategies = []string{
		NamingStrategyManual,
		NamingStrategyRandom,
		NamingStrategySeries,
		NamingStrategyFormat,
	}
	checkOperators  = []string{"eq", "neq", "gt", "gte", "lt", "lte", "in", "not-in"}
	constraintTypes = []string{"unique", "check"}
)

// SupportedNamingStrategies returns the authored Entity naming strategies in stable order.
func SupportedNamingStrategies() []string {
	return append([]string(nil), namingStrategies...)
}

// SupportedCheckOperators returns authored check operators in stable order.
func SupportedCheckOperators() []string {
	return append([]string(nil), checkOperators...)
}

// SupportedConstraintTypes returns authored Entity constraint types in stable order.
func SupportedConstraintTypes() []string {
	return append([]string(nil), constraintTypes...)
}

// Field describes one field inside an Entity.
type Field struct {
	Line     int               `yaml:"-"`
	Name     string            `yaml:"name"`
	Label    string            `yaml:"label"`
	Type     string            `yaml:"type"`
	Required bool              `yaml:"required,omitempty"`
	Unique   bool              `yaml:"unique,omitempty"`
	Index    bool              `yaml:"index,omitempty"`
	Default  yaml.Node         `yaml:"default,omitempty"`
	Check    *Check            `yaml:"check,omitempty"`
	Fetch    *Fetch            `yaml:"fetch,omitempty"`
	Options  fieldtype.Options `yaml:"options,omitempty"`
}

// Check describes one single-field structured value check.
type Check struct {
	Operator string    `yaml:"operator"`
	Value    yaml.Node `yaml:"value,omitempty"`
}

// Fetch describes a value copied from a linked Record path.
type Fetch struct {
	From string `yaml:"from" json:"from"`
}

// Index describes a non-unique Entity-level database index.
type Index struct {
	Line   int      `yaml:"-"`
	Name   string   `yaml:"name,omitempty"`
	Fields []string `yaml:"fields"`
}

// EffectiveName returns the configured or deterministic metadata name for the index.
func (i Index) EffectiveName(entity Entity) string {
	if strings.TrimSpace(i.Name) != "" {
		return i.Name
	}
	parts := []string{entity.Name}
	parts = append(parts, i.Fields...)
	parts = append(parts, "idx")
	return strings.Join(parts, "-")
}

// Constraint describes an Entity-level database constraint.
type Constraint struct {
	Line     int       `yaml:"-"`
	Name     string    `yaml:"name,omitempty"`
	Type     string    `yaml:"type"`
	Fields   []string  `yaml:"fields,omitempty"`
	Field    string    `yaml:"field,omitempty"`
	Operator string    `yaml:"operator,omitempty"`
	Value    yaml.Node `yaml:"value,omitempty"`
}

// EffectiveName returns the configured or deterministic metadata name for the constraint.
func (c Constraint) EffectiveName(entity Entity) string {
	if strings.TrimSpace(c.Name) != "" {
		return c.Name
	}
	switch c.Type {
	case "unique":
		parts := []string{entity.Name}
		parts = append(parts, c.Fields...)
		parts = append(parts, "key")
		return strings.Join(parts, "-")
	case "check":
		return strings.Join([]string{entity.Name, c.Field, c.Operator, "check"}, "-")
	default:
		return strings.Join([]string{entity.Name, "constraint"}, "-")
	}
}

// ValidationError reports one or more Entity validation problems.
type ValidationError struct {
	Problems []string
}

func (e ValidationError) Error() string {
	return "entity schema validation failed: " + strings.Join(e.Problems, "; ")
}

// LoadFile reads, decodes, and validates one Entity metadata file.
func LoadFile(path string, registry fieldtype.Registry) (Entity, error) {
	return LoadFileWithOptions(path, registry, LoadOptions{})
}

// LoadOptions controls path-aware Entity loading behavior.
type LoadOptions struct {
	IsCollection bool
}

// LoadFileWithOptions reads, decodes, and validates one Entity metadata file with load-time context.
func LoadFileWithOptions(path string, registry fieldtype.Registry, options LoadOptions) (Entity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Entity{}, fmt.Errorf("read entity schema %s: %w", path, err)
	}
	entity, err := DecodeWithOptions(data, registry, DecodeOptions{IsCollection: options.IsCollection})
	if err != nil {
		return Entity{}, fmt.Errorf("load entity schema %s: %w", path, err)
	}
	name, err := entityNameFromPath(path)
	if err != nil {
		return Entity{}, fmt.Errorf("load entity schema %s: %w", path, err)
	}
	entity.Name = name
	return entity, nil
}

// Decode decodes and validates one Entity metadata document.
func Decode(data []byte, registry fieldtype.Registry) (Entity, error) {
	return DecodeWithOptions(data, registry, DecodeOptions{})
}

// DecodeOptions controls Entity decoding behavior.
type DecodeOptions struct {
	IsCollection bool
}

// DecodeWithOptions decodes and validates one Entity metadata document with caller-supplied context.
func DecodeWithOptions(data []byte, registry fieldtype.Registry, options DecodeOptions) (Entity, error) {
	source, err := inspectSource(data)
	if err != nil {
		return Entity{}, err
	}

	var entity Entity
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&entity); err != nil {
		return Entity{}, fmt.Errorf("decode entity schema: %w", err)
	}
	source.apply(&entity)
	entity.IsCollection = options.IsCollection
	if err := entity.Validate(registry); err != nil {
		return Entity{}, err
	}
	return entity, nil
}

// Validate validates an Entity against a field type registry.
func (e Entity) Validate(registry fieldtype.Registry) error {
	var problems []string

	if strings.TrimSpace(e.Name) != "" && !fieldtype.IsName(e.Name) {
		problems = append(problems, withLine(e.Line, fmt.Sprintf("name %q must be kebab-case", e.Name)))
	}
	if strings.TrimSpace(e.Label) == "" {
		problems = append(problems, withLine(e.Line, "label is required"))
	}
	if strings.TrimSpace(e.Route.Slug) != "" && !fieldtype.IsName(e.Route.Slug) {
		line := e.Route.Line
		if line == 0 {
			line = e.Line
		}
		problems = append(problems, withLine(line, fmt.Sprintf("route slug %q must be kebab-case", e.Route.Slug)))
	}
	if len(e.Fields) == 0 {
		problems = append(problems, withLine(e.Line, "at least one field is required"))
	}

	seenFields := map[string]struct{}{}
	fields := map[string]Field{}
	fieldTypes := map[string]fieldtype.Definition{}
	for _, field := range e.Fields {
		validateField(field, registry, seenFields, &problems)
		if e.IsCollection && isCollectionSystemFieldName(field.Name) {
			problems = append(problems, withLine(field.Line, fmt.Sprintf("collection field %q is reserved for framework collection row storage", field.Name)))
		}
		if e.IsCollection && field.Type == "collection" {
			problems = append(problems, withLine(field.Line, "collection Entities cannot define collection fields in v1"))
		}
		if field.Name != "" {
			seenFields[field.Name] = struct{}{}
			fields[field.Name] = field
			if definition, ok := registry.Get(field.Type); ok {
				fieldTypes[field.Name] = definition
				if e.IsSingle {
					validateSingleFieldDefault(field, definition, &problems)
				}
			}
		}
	}
	validateIndexes(e, fields, fieldTypes, &problems)
	validateConstraints(e, fields, fieldTypes, &problems)
	if e.IsCollection && hasExplicitNaming(e.Naming) {
		line := e.Naming.Line
		if line == 0 {
			line = e.Line
		}
		problems = append(problems, withLine(line, "collection Entities do not support explicit name configuration"))
	} else if e.IsSingle && hasExplicitNaming(e.Naming) {
		line := e.Naming.Line
		if line == 0 {
			line = e.Line
		}
		problems = append(problems, withLine(line, "single Entities do not support explicit name configuration"))
	} else if !e.IsSingle && !e.IsCollection && !hasExplicitNaming(e.Naming) {
		problems = append(problems, withLine(e.Line, "Entity must define name. Use name.strategy: random for generated names."))
	}
	validateNaming(e, fields, fieldTypes, &problems)

	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

// EffectiveRouteSlug returns the explicit route slug or the default Entity name.
func (e Entity) EffectiveRouteSlug() string {
	if strings.TrimSpace(e.Route.Slug) != "" {
		return e.Route.Slug
	}
	return e.Name
}

// EffectiveNaming returns explicit Entity naming or the v1 default.
func (e Entity) EffectiveNaming() Naming {
	if e.IsCollection {
		return CollectionRowNaming()
	}
	naming := e.Naming
	if strings.TrimSpace(naming.Strategy) == "" {
		naming.Strategy = NamingStrategyRandom
	}
	if naming.Strategy == NamingStrategyRandom && naming.Length == 0 {
		naming.Length = DefaultRandomNameLength
	}
	return naming
}

// CollectionRowNaming returns the framework-owned naming plan for collection rows.
func CollectionRowNaming() Naming {
	return Naming{Strategy: NamingStrategyRandom, Length: CollectionRowNameLength}
}

func isCollectionSystemFieldName(name string) bool {
	switch name {
	case "ordinal", "parent-entity-id", "parent-record-id", "parent-field-id":
		return true
	default:
		return false
	}
}

func hasExplicitNaming(naming Naming) bool {
	return naming.Line != 0 ||
		strings.TrimSpace(naming.Strategy) != "" ||
		strings.TrimSpace(naming.Label) != "" ||
		naming.Length != 0 ||
		strings.TrimSpace(naming.Pattern) != "" ||
		strings.TrimSpace(naming.Format) != ""
}

func validateSingleFieldDefault(field Field, definition fieldtype.Definition, problems *[]string) {
	if !field.Required || !definition.Behavior.Stored {
		return
	}
	if field.Default.Kind == 0 {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("single Entity required field %q must define a non-null default", field.Name)))
		return
	}
	if field.Default.Kind != yaml.ScalarNode {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("single Entity required field %q default must be scalar", field.Name)))
		return
	}
	if field.Default.Tag == "!!null" {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("single Entity required field %q default must not be null", field.Name)))
	}
}

func validateNaming(entity Entity, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, problems *[]string) {
	if field, ok := fields["name"]; ok {
		*problems = append(*problems, withLine(field.Line, `field "name" is reserved; configure Record names with top-level name metadata`))
	}
	if !hasExplicitNaming(entity.Naming) {
		return
	}
	naming := entity.Naming
	line := naming.Line
	if line == 0 {
		line = entity.Line
	}
	if strings.TrimSpace(naming.Strategy) == "" {
		*problems = append(*problems, withLine(line, "name strategy is required"))
		return
	}
	if naming.Strategy == NamingStrategyRandom && naming.Length == 0 {
		naming.Length = DefaultRandomNameLength
	}

	switch naming.Strategy {
	case NamingStrategyManual:
		validateManualNaming(naming, line, problems)
	case NamingStrategyRandom:
		validateRandomNaming(naming, line, problems)
	case NamingStrategySeries:
		validateSeriesNaming(naming, line, problems)
	case NamingStrategyFormat:
		validateFormatNaming(naming, fields, fieldTypes, line, problems)
	default:
		*problems = append(*problems, withLine(line, fmt.Sprintf("naming strategy %q is not supported", naming.Strategy)))
	}

}

func validateManualNaming(naming Naming, line int, problems *[]string) {
	if naming.Length != 0 || strings.TrimSpace(naming.Pattern) != "" || strings.TrimSpace(naming.Format) != "" {
		*problems = append(*problems, withLine(line, "manual name supports label only"))
	}
}

func validateRandomNaming(naming Naming, line int, problems *[]string) {
	if strings.TrimSpace(naming.Label) != "" || strings.TrimSpace(naming.Pattern) != "" || strings.TrimSpace(naming.Format) != "" {
		*problems = append(*problems, withLine(line, "random naming supports length only"))
	}
	if naming.Length < MinRandomNameLength || naming.Length > MaxRandomNameLength {
		*problems = append(*problems, withLine(line, fmt.Sprintf("random naming length must be between %d and %d", MinRandomNameLength, MaxRandomNameLength)))
	}
}

func validateSeriesNaming(naming Naming, line int, problems *[]string) {
	if strings.TrimSpace(naming.Label) != "" || naming.Length != 0 || strings.TrimSpace(naming.Format) != "" {
		*problems = append(*problems, withLine(line, "series naming supports pattern only"))
	}
	if strings.TrimSpace(naming.Pattern) == "" {
		*problems = append(*problems, withLine(line, "series naming pattern is required"))
		return
	}
	if err := validateSeriesPattern(naming.Pattern); err != nil {
		*problems = append(*problems, withLine(line, err.Error()))
	}
}

func validateFormatNaming(naming Naming, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, line int, problems *[]string) {
	if strings.TrimSpace(naming.Label) != "" || naming.Length != 0 || strings.TrimSpace(naming.Pattern) != "" {
		*problems = append(*problems, withLine(line, "format naming supports format only"))
	}
	if strings.TrimSpace(naming.Format) == "" {
		*problems = append(*problems, withLine(line, "format naming format is required"))
		return
	}
	tokens, err := namingFormatTokens(naming.Format)
	if err != nil {
		*problems = append(*problems, withLine(line, err.Error()))
		return
	}
	if len(tokens) == 0 {
		*problems = append(*problems, withLine(line, "format naming must include at least one field token"))
		return
	}
	for _, token := range tokens {
		field, ok := fields[token]
		if !ok {
			*problems = append(*problems, withLine(line, fmt.Sprintf("format naming references unknown field %q", token)))
			continue
		}
		definition, ok := fieldTypes[token]
		if !ok {
			*problems = append(*problems, withLine(field.Line, fmt.Sprintf("format naming field %q type %q is unknown", token, field.Type)))
			continue
		}
		if !field.Required {
			*problems = append(*problems, withLine(field.Line, fmt.Sprintf("format naming field %q must be required", token)))
		}
		if !definition.Behavior.NameRenderable {
			*problems = append(*problems, withLine(field.Line, fmt.Sprintf("format naming field %q type %q cannot be used for naming", token, field.Type)))
		}
		if !definition.Behavior.Stored {
			*problems = append(*problems, withLine(field.Line, fmt.Sprintf("format naming field %q type %q is not stored", token, field.Type)))
		}
		if definition.Behavior.WriteOnly {
			*problems = append(*problems, withLine(field.Line, fmt.Sprintf("format naming field %q type %q is write-only", token, field.Type)))
		}
	}
}

func namingFormatTokens(format string) ([]string, error) {
	var tokens []string
	for i := 0; i < len(format); {
		switch format[i] {
		case '{':
			end := strings.IndexByte(format[i+1:], '}')
			if end < 0 {
				return nil, fmt.Errorf("format naming format has an unclosed token")
			}
			token := format[i+1 : i+1+end]
			if strings.Contains(token, "{") {
				return nil, fmt.Errorf("format naming format has a nested token")
			}
			if !fieldtype.IsName(token) {
				return nil, fmt.Errorf("format naming token %q must be a field name", "{"+token+"}")
			}
			tokens = append(tokens, token)
			i += end + 2
		case '}':
			return nil, fmt.Errorf("format naming format has an unopened token")
		default:
			i++
		}
	}
	return tokens, nil
}

func validateSeriesPattern(pattern string) error {
	counterTokens := 0
	for i := 0; i < len(pattern); i++ {
		if pattern[i] != '{' {
			continue
		}
		end := strings.IndexByte(pattern[i+1:], '}')
		if end < 0 {
			return fmt.Errorf("series naming pattern has an unclosed token")
		}
		token := pattern[i+1 : i+1+end]
		if token == "YY" || token == "YYYY" || token == "MM" {
			i += end + 1
			continue
		}
		if strings.Trim(token, "#") == "" && len(token) > 0 {
			counterTokens++
			i += end + 1
			continue
		}
		return fmt.Errorf("series naming pattern token %q is not supported", "{"+token+"}")
	}
	if strings.Contains(pattern, "}") && !strings.Contains(pattern, "{") {
		return fmt.Errorf("series naming pattern has an unopened token")
	}
	if counterTokens != 1 {
		return fmt.Errorf("series naming pattern must include exactly one hash counter token")
	}
	return nil
}

func validateIndexes(entity Entity, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, problems *[]string) {
	seenNames := map[string]struct{}{}
	seenDefinitions := map[string]struct{}{}
	for _, index := range entity.Indexes {
		if strings.TrimSpace(index.Name) != "" && !fieldtype.IsName(index.Name) {
			*problems = append(*problems, withLine(index.Line, fmt.Sprintf("index name %q must be kebab-case", index.Name)))
		}
		if len(index.Fields) == 0 {
			*problems = append(*problems, withLine(index.Line, "index fields are required"))
		}
		validateFieldReferences(index.Line, "index", index.Fields, fields, fieldTypes, func(definition fieldtype.Definition) bool {
			return definition.AllowIndex
		}, "indexed", problems)

		name := index.EffectiveName(entity)
		if strings.TrimSpace(name) != "" {
			if _, ok := seenNames[name]; ok {
				*problems = append(*problems, withLine(index.Line, fmt.Sprintf("duplicate index name %q", name)))
			}
			seenNames[name] = struct{}{}
		}
		definitionKey := strings.Join(index.Fields, "\x00")
		if definitionKey != "" {
			if _, ok := seenDefinitions[definitionKey]; ok {
				*problems = append(*problems, withLine(index.Line, fmt.Sprintf("duplicate index fields %q", strings.Join(index.Fields, ", "))))
			}
			seenDefinitions[definitionKey] = struct{}{}
		}
	}
}

func validateConstraints(entity Entity, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, problems *[]string) {
	seenNames := map[string]struct{}{}
	seenDefinitions := map[string]struct{}{}
	for _, constraint := range entity.Constraints {
		if strings.TrimSpace(constraint.Name) != "" && !fieldtype.IsName(constraint.Name) {
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("constraint name %q must be kebab-case", constraint.Name)))
		}
		if strings.TrimSpace(constraint.Type) == "" {
			*problems = append(*problems, withLine(constraint.Line, "constraint type is required"))
			continue
		}
		switch constraint.Type {
		case "unique":
			validateUniqueConstraint(constraint, fields, fieldTypes, seenDefinitions, problems)
		case "check":
			validateCheckConstraint(constraint, fields, fieldTypes, seenDefinitions, problems)
		default:
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("constraint type %q is not supported", constraint.Type)))
		}

		name := constraint.EffectiveName(entity)
		if strings.TrimSpace(name) != "" {
			if _, ok := seenNames[name]; ok {
				*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("duplicate constraint name %q", name)))
			}
			seenNames[name] = struct{}{}
		}
	}
}

func validateUniqueConstraint(constraint Constraint, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, seenDefinitions map[string]struct{}, problems *[]string) {
	if len(constraint.Fields) < 2 {
		*problems = append(*problems, withLine(constraint.Line, "unique constraint requires at least two fields"))
	}
	if strings.TrimSpace(constraint.Field) != "" {
		*problems = append(*problems, withLine(constraint.Line, "unique constraint uses fields, not field"))
	}
	if strings.TrimSpace(constraint.Operator) != "" || constraint.Value.Kind != 0 {
		*problems = append(*problems, withLine(constraint.Line, "unique constraint does not support operator or value"))
	}
	validateFieldReferences(constraint.Line, "unique constraint", constraint.Fields, fields, fieldTypes, func(definition fieldtype.Definition) bool {
		return definition.AllowUnique
	}, "unique", problems)

	key := constraint.Type + "\x00" + strings.Join(constraint.Fields, "\x00")
	if key != constraint.Type+"\x00" {
		if _, ok := seenDefinitions[key]; ok {
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("duplicate unique constraint fields %q", strings.Join(constraint.Fields, ", "))))
		}
		seenDefinitions[key] = struct{}{}
	}
}

func validateCheckConstraint(constraint Constraint, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, seenDefinitions map[string]struct{}, problems *[]string) {
	if len(constraint.Fields) > 0 {
		*problems = append(*problems, withLine(constraint.Line, "check constraint uses field, not fields"))
	}
	if strings.TrimSpace(constraint.Field) == "" {
		*problems = append(*problems, withLine(constraint.Line, "check constraint field is required"))
	} else if !fieldtype.IsName(constraint.Field) {
		*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("check constraint field %q must be kebab-case", constraint.Field)))
	} else {
		field, ok := fields[constraint.Field]
		if !ok {
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("check constraint references unknown field %q", constraint.Field)))
		} else if definition, ok := fieldTypes[constraint.Field]; ok && !definition.Behavior.Checkable {
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("check constraint field %q type %q is not supported", constraint.Field, field.Type)))
		} else if _, ok := fieldTypes[constraint.Field]; !ok {
			*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("check constraint field %q type %q is unknown", constraint.Field, field.Type)))
		}
	}
	validateCheckRule(constraint.Line, "check constraint", constraint.Operator, constraint.Value, problems)

	key := strings.Join([]string{constraint.Type, constraint.Field, constraint.Operator, checkValueKey(constraint.Value)}, "\x00")
	if _, ok := seenDefinitions[key]; ok {
		*problems = append(*problems, withLine(constraint.Line, fmt.Sprintf("duplicate check constraint on field %q", constraint.Field)))
	}
	seenDefinitions[key] = struct{}{}
}

func validateFieldReferences(line int, owner string, refs []string, fields map[string]Field, fieldTypes map[string]fieldtype.Definition, allowed func(fieldtype.Definition) bool, capability string, problems *[]string) {
	seen := map[string]struct{}{}
	for _, ref := range refs {
		if strings.TrimSpace(ref) == "" {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s field names must not be empty", owner)))
			continue
		}
		if !fieldtype.IsName(ref) {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s field %q must be kebab-case", owner, ref)))
			continue
		}
		if _, ok := seen[ref]; ok {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s has duplicate field %q", owner, ref)))
			continue
		}
		seen[ref] = struct{}{}
		field, ok := fields[ref]
		if !ok {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s references unknown field %q", owner, ref)))
			continue
		}
		definition, ok := fieldTypes[ref]
		if !ok {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s field %q type %q is unknown", owner, ref, field.Type)))
			continue
		}
		if !allowed(definition) {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s field %q type %q cannot be %s", owner, ref, field.Type, capability)))
		}
	}
}

func isCheckOperator(value string) bool {
	for _, operator := range checkOperators {
		if value == operator {
			return true
		}
	}
	return false
}

func validateCheckRule(line int, owner string, operator string, value yaml.Node, problems *[]string) {
	if !isCheckOperator(operator) {
		*problems = append(*problems, withLine(line, fmt.Sprintf("%s operator %q is not supported", owner, operator)))
	}
	validateCheckValue(line, owner, operator, value, problems)
}

func validateCheckValue(line int, owner string, operator string, value yaml.Node, problems *[]string) {
	if operator == "in" || operator == "not-in" {
		if value.Kind == 0 {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s value is required", owner)))
			return
		}
		if value.Kind != yaml.SequenceNode || len(value.Content) == 0 {
			*problems = append(*problems, withLine(line, fmt.Sprintf("%s value must be a non-empty list for in and not-in", owner)))
			return
		}
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				*problems = append(*problems, withLine(line, fmt.Sprintf("%s list values must be scalar", owner)))
				return
			}
			if item.Tag == "!!null" {
				*problems = append(*problems, withLine(line, fmt.Sprintf("%s list values must not be null", owner)))
				return
			}
		}
		return
	}
	if value.Kind == 0 {
		*problems = append(*problems, withLine(line, fmt.Sprintf("%s value is required", owner)))
		return
	}
	if value.Kind != yaml.ScalarNode {
		*problems = append(*problems, withLine(line, fmt.Sprintf("%s value must be scalar", owner)))
		return
	}
	if value.Tag == "!!null" {
		*problems = append(*problems, withLine(line, fmt.Sprintf("%s value must not be null", owner)))
	}
}

func checkValueKey(node yaml.Node) string {
	if node.Kind == 0 {
		return ""
	}
	if node.Kind == yaml.SequenceNode {
		values := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			values = append(values, item.Tag+"="+item.Value)
		}
		return strings.Join(values, ",")
	}
	return node.Tag + "=" + node.Value
}

func validateField(field Field, registry fieldtype.Registry, seenFields map[string]struct{}, problems *[]string) {
	fieldLabel := field.Name
	if fieldLabel == "" {
		fieldLabel = "<missing>"
	}

	if strings.TrimSpace(field.Name) == "" {
		*problems = append(*problems, withLine(field.Line, "field name is required"))
	} else if !fieldtype.IsName(field.Name) {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q name must be kebab-case", field.Name)))
	} else if field.Name != "name" && reserved.IsField(field.Name) {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q is reserved", field.Name)))
	}
	if _, ok := seenFields[field.Name]; ok {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("duplicate field %q", field.Name)))
	}
	if strings.TrimSpace(field.Label) == "" {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q label is required", fieldLabel)))
	}
	if strings.TrimSpace(field.Type) == "" {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type is required", fieldLabel)))
		return
	}
	if !fieldtype.IsName(field.Type) {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q must be kebab-case", fieldLabel, field.Type)))
		return
	}

	definition, ok := registry.Get(field.Type)
	if !ok {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q uses unknown type %q", fieldLabel, field.Type)))
		return
	}
	if field.Required && !definition.AllowRequired {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q cannot be required", fieldLabel, field.Type)))
	}
	if field.Unique && !definition.AllowUnique {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q cannot be unique", fieldLabel, field.Type)))
	}
	if field.Index && !definition.AllowIndex {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q cannot be indexed", fieldLabel, field.Type)))
	}
	if field.Default.Kind != 0 && !definition.AllowDefault {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q does not support default values", fieldLabel, field.Type)))
	}
	if field.Check != nil {
		if !definition.Behavior.Checkable {
			*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q type %q does not support checks", fieldLabel, field.Type)))
		}
		validateCheckRule(field.Line, fmt.Sprintf("field %q check", fieldLabel), field.Check.Operator, field.Check.Value, problems)
	}
	validateFetch(field, problems)
	if err := definition.Validate(field.Options); err != nil {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q options invalid for type %q: %v", fieldLabel, field.Type, err)))
	}
}

func validateFetch(field Field, problems *[]string) {
	if field.Fetch == nil {
		return
	}
	path := strings.TrimSpace(field.Fetch.From)
	if path == "" {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q fetch.from is required", field.Name)))
		return
	}
	segments := strings.Split(path, ".")
	if len(segments) < 2 {
		*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q fetch.from must include a link field and target field", field.Name)))
		return
	}
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q fetch.from contains an empty path segment", field.Name)))
			return
		}
		if !fieldtype.IsName(segment) {
			*problems = append(*problems, withLine(field.Line, fmt.Sprintf("field %q fetch.from segment %q must be kebab-case", field.Name, segment)))
		}
	}
}

type sourceMap struct {
	entityLine      int
	routeLine       int
	namingLine      int
	fieldLines      []int
	indexLines      []int
	constraintLines []int
}

func (m sourceMap) apply(entity *Entity) {
	entity.Line = m.entityLine
	entity.Route.Line = m.routeLine
	entity.Naming.Line = m.namingLine
	for i := range entity.Fields {
		if i >= len(m.fieldLines) {
			break
		}
		entity.Fields[i].Line = m.fieldLines[i]
	}
	for i := range entity.Indexes {
		if i >= len(m.indexLines) {
			break
		}
		entity.Indexes[i].Line = m.indexLines[i]
	}
	for i := range entity.Constraints {
		if i >= len(m.constraintLines) {
			break
		}
		entity.Constraints[i].Line = m.constraintLines[i]
	}
}

func inspectSource(data []byte) (sourceMap, error) {
	root, err := yamlmeta.Parse(data, "parse entity schema")
	if err != nil {
		return sourceMap{}, err
	}
	if err := yamlmeta.RejectDuplicateKeys(&root, func(duplicate yamlmeta.DuplicateKey) error {
		return fmt.Errorf("duplicate key %q at %s line %d", duplicate.Key, strings.TrimSuffix(duplicate.Location, "."+duplicate.Key), duplicate.Line)
	}); err != nil {
		return sourceMap{}, err
	}
	return buildSourceMap(&root), nil
}

func entityNameFromPath(path string) (string, error) {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if strings.HasSuffix(base, ".entity.yml") {
		name = strings.TrimSuffix(base, ".entity.yml")
	}
	if name == "entity" {
		parent := filepath.Base(filepath.Dir(path))
		if parent != "" && parent != "." && parent != "entities" && parent != "_collections" {
			name = parent
		}
	}
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("entity filename must not be empty")
	}
	if !fieldtype.IsName(name) {
		return "", fmt.Errorf("entity filename %q must be kebab-case", filepath.Base(path))
	}
	return name, nil
}

func buildSourceMap(root *yaml.Node) sourceMap {
	mapping := yamlmeta.DocumentMapping(root)
	if mapping == nil {
		return sourceMap{}
	}

	source := sourceMap{entityLine: mapping.Line}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		value := mapping.Content[i+1]
		switch key.Value {
		case "route":
			if value.Kind == yaml.MappingNode {
				source.routeLine = value.Line
			}
		case "name":
			source.namingLine = value.Line
		case "fields":
			if value.Kind != yaml.SequenceNode {
				continue
			}
			for _, item := range value.Content {
				source.fieldLines = append(source.fieldLines, item.Line)
			}
		case "indexes":
			if value.Kind != yaml.SequenceNode {
				continue
			}
			for _, item := range value.Content {
				source.indexLines = append(source.indexLines, item.Line)
			}
		case "constraints":
			if value.Kind != yaml.SequenceNode {
				continue
			}
			for _, item := range value.Content {
				source.constraintLines = append(source.constraintLines, item.Line)
			}
		}
	}
	return source
}

func withLine(line int, message string) string {
	if line == 0 {
		return message
	}
	return fmt.Sprintf("line %d: %s", line, message)
}

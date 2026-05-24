package naming

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dygo-dev/dygo/internal/entity/schema"
)

var seriesTokenPattern = regexp.MustCompile(`\{(YY|YYYY|MM|#+)\}`)

// ValueResolver provides values for manual and format naming tokens.
type ValueResolver interface {
	Value(ctx context.Context, token string) (string, error)
}

// ValueResolverFunc adapts a function into a ValueResolver.
type ValueResolverFunc func(ctx context.Context, token string) (string, error)

// Value resolves one naming token.
func (f ValueResolverFunc) Value(ctx context.Context, token string) (string, error) {
	if f == nil {
		return "", fmt.Errorf("naming value %q is not available", token)
	}
	return f(ctx, token)
}

// MapResolver resolves naming tokens from an in-memory map.
type MapResolver map[string]string

// Value resolves one naming token from the map.
func (r MapResolver) Value(_ context.Context, token string) (string, error) {
	value, ok := r[token]
	if !ok {
		return "", fmt.Errorf("naming value %q is not available", token)
	}
	return value, nil
}

// SeriesCounter increments and returns the next value for a rendered series key.
type SeriesCounter interface {
	Next(ctx context.Context, key string, pattern string) (int64, error)
}

// SeriesCounterFunc adapts a function into a SeriesCounter.
type SeriesCounterFunc func(ctx context.Context, key string, pattern string) (int64, error)

// Next increments and returns the next series value.
func (f SeriesCounterFunc) Next(ctx context.Context, key string, pattern string) (int64, error) {
	if f == nil {
		return 0, fmt.Errorf("series naming requires a series counter")
	}
	return f(ctx, key, pattern)
}

// Options configures strategy behavior that needs runtime services.
type Options struct {
	Entity string
	Now    func() time.Time
	Random func(length int) (string, error)
	Series SeriesCounter
}

// Generate renders one system Record name from Entity naming metadata.
func Generate(ctx context.Context, plan schema.Naming, resolver ValueResolver, options Options) (string, error) {
	plan = normalizePlan(plan)
	switch plan.Strategy {
	case schema.NamingStrategyManual:
		return resolveValue(ctx, resolver, "name")
	case schema.NamingStrategyRandom:
		random := options.Random
		if random == nil {
			random = Random
		}
		name, err := random(plan.Length)
		if err != nil {
			return "", fmt.Errorf("generate random name: %w", err)
		}
		return name, nil
	case schema.NamingStrategySeries:
		return generateSeriesName(ctx, plan, options)
	case schema.NamingStrategyFormat:
		return RenderFormat(plan.Format, func(token string) (string, error) {
			return resolveValue(ctx, resolver, token)
		})
	default:
		return "", fmt.Errorf("naming strategy %q is not supported", plan.Strategy)
	}
}

// GenerateDeterministic renders a name for strategies that do not mutate state or use randomness.
func GenerateDeterministic(ctx context.Context, plan schema.Naming, resolver ValueResolver) (string, error) {
	plan = normalizePlan(plan)
	switch plan.Strategy {
	case schema.NamingStrategyRandom, schema.NamingStrategySeries:
		return "", fmt.Errorf("naming strategy %q is not deterministic", plan.Strategy)
	default:
		return Generate(ctx, plan, resolver, Options{})
	}
}

func normalizePlan(plan schema.Naming) schema.Naming {
	plan.Strategy = strings.TrimSpace(plan.Strategy)
	if plan.Strategy == "" {
		plan.Strategy = schema.NamingStrategyRandom
	}
	if plan.Strategy == schema.NamingStrategyRandom && plan.Length == 0 {
		plan.Length = schema.DefaultRandomNameLength
	}
	return plan
}

func resolveValue(ctx context.Context, resolver ValueResolver, token string) (string, error) {
	if resolver == nil {
		return "", fmt.Errorf("naming value %q is not available", token)
	}
	return resolver.Value(ctx, token)
}

func generateSeriesName(ctx context.Context, plan schema.Naming, options Options) (string, error) {
	if strings.TrimSpace(options.Entity) == "" {
		return "", fmt.Errorf("series naming requires entity")
	}
	rendered, counterWidth, key, err := RenderSeriesPattern(options.Entity, plan.Pattern, namingNow(options))
	if err != nil {
		return "", err
	}
	if options.Series == nil {
		return "", fmt.Errorf("series naming requires a series counter")
	}
	current, err := options.Series.Next(ctx, key, plan.Pattern)
	if err != nil {
		return "", err
	}
	return strings.Replace(rendered, "{#}", fmt.Sprintf("%0*d", counterWidth, current), 1), nil
}

func namingNow(options Options) time.Time {
	if options.Now != nil {
		return options.Now()
	}
	return time.Now().UTC()
}

// RenderFormat replaces {field} tokens by calling resolve for each token.
func RenderFormat(format string, resolve func(token string) (string, error)) (string, error) {
	if resolve == nil {
		return "", fmt.Errorf("format resolver is required")
	}
	var rendered strings.Builder
	for i := 0; i < len(format); {
		switch format[i] {
		case '{':
			end := strings.IndexByte(format[i+1:], '}')
			if end < 0 {
				return "", fmt.Errorf("format has an unclosed token")
			}
			token := format[i+1 : i+1+end]
			value, err := resolve(token)
			if err != nil {
				return "", err
			}
			rendered.WriteString(value)
			i += end + 2
		case '}':
			return "", fmt.Errorf("format has an unopened token")
		default:
			rendered.WriteByte(format[i])
			i++
		}
	}
	return rendered.String(), nil
}

// RenderSeriesPattern renders date tokens and returns the stable counter key.
func RenderSeriesPattern(entity string, pattern string, now time.Time) (rendered string, counterWidth int, key string, err error) {
	counterTokens := 0
	rendered = seriesTokenPattern.ReplaceAllStringFunc(pattern, func(token string) string {
		name := strings.Trim(token, "{}")
		switch name {
		case "YY":
			return now.Format("06")
		case "YYYY":
			return now.Format("2006")
		case "MM":
			return now.Format("01")
		default:
			counterTokens++
			counterWidth = len(name)
			return "{#}"
		}
	})
	if counterTokens != 1 {
		return "", 0, "", fmt.Errorf("series pattern must include exactly one counter token")
	}
	key = entity + ":" + strings.Replace(rendered, "{#}", "{"+strings.Repeat("#", counterWidth)+"}", 1)
	return rendered, counterWidth, key, nil
}

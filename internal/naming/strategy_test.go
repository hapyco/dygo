package naming

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hapyco/dygo/internal/entity/schema"
)

func TestGenerateNameStrategies(t *testing.T) {
	ctx := context.Background()
	fixedNow := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		plan     schema.Naming
		resolver ValueResolver
		options  Options
		want     string
	}{
		{
			name: "random",
			plan: schema.Naming{Strategy: schema.NamingStrategyRandom, Length: 10},
			options: Options{
				Random: func(length int) (string, error) {
					return strings.Repeat("x", length), nil
				},
			},
			want: "xxxxxxxxxx",
		},
		{
			name:     "manual",
			plan:     schema.Naming{Strategy: schema.NamingStrategyManual},
			resolver: MapResolver{"name": "A-001"},
			want:     "A-001",
		},
		{
			name:     "format",
			plan:     schema.Naming{Strategy: schema.NamingStrategyFormat, Format: "{entity}.{field-name}"},
			resolver: MapResolver{"entity": "core.user", "field-name": "email"},
			want:     "core.user.email",
		},
		{
			name: "series",
			plan: schema.Naming{Strategy: schema.NamingStrategySeries, Pattern: "SINV-{YYYY}-{MM}-{#####}"},
			options: Options{
				Entity: "sales-invoice",
				Now: func() time.Time {
					return fixedNow
				},
				Series: SeriesCounterFunc(func(_ context.Context, key string, pattern string) (int64, error) {
					if key != "sales-invoice:SINV-2026-05-{#####}" {
						t.Fatalf("series key = %q, want entity-scoped rendered key", key)
					}
					if pattern != "SINV-{YYYY}-{MM}-{#####}" {
						t.Fatalf("series pattern = %q, want original pattern", pattern)
					}
					return 42, nil
				}),
			},
			want: "SINV-2026-05-00042",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Generate(ctx, tt.plan, tt.resolver, tt.options)
			if err != nil {
				t.Fatalf("Generate() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("Generate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateDeterministicRejectsStatefulStrategies(t *testing.T) {
	tests := []schema.Naming{
		{Strategy: schema.NamingStrategyRandom, Length: 16},
		{Strategy: schema.NamingStrategySeries, Pattern: "A-{###}"},
	}
	for _, plan := range tests {
		t.Run(plan.Strategy, func(t *testing.T) {
			_, err := GenerateDeterministic(context.Background(), plan, MapResolver{})
			if err == nil {
				t.Fatal("GenerateDeterministic() error = nil, want stateful strategy error")
			}
			if !strings.Contains(err.Error(), "not deterministic") {
				t.Fatalf("GenerateDeterministic() error = %q, want deterministic context", err.Error())
			}
		})
	}
}

func TestGeneratePropagatesResolverErrors(t *testing.T) {
	wantErr := errors.New("missing value")
	_, err := Generate(context.Background(), schema.Naming{
		Strategy: schema.NamingStrategyFormat,
		Format:   "{missing}",
	}, ValueResolverFunc(func(context.Context, string) (string, error) {
		return "", wantErr
	}), Options{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Generate() error = %v, want %v", err, wantErr)
	}
}

func TestRenderSeriesPattern(t *testing.T) {
	rendered, width, key, err := RenderSeriesPattern("sales-invoice", "SINV-{YYYY}-{MM}-{#####}", time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("RenderSeriesPattern() error = %v, want nil", err)
	}
	if rendered != "SINV-2026-05-{#}" {
		t.Fatalf("rendered = %q, want placeholder-rendered pattern", rendered)
	}
	if width != 5 {
		t.Fatalf("width = %d, want 5", width)
	}
	if key != "sales-invoice:SINV-2026-05-{#####}" {
		t.Fatalf("key = %q, want entity scoped key", key)
	}
}

func TestRandom(t *testing.T) {
	name, err := Random(16)
	if err != nil {
		t.Fatalf("Random() error = %v, want nil", err)
	}
	if len(name) != 16 {
		t.Fatalf("Random() length = %d, want 16", len(name))
	}
}

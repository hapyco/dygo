package corevalues

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/hapyco/dygo/internal/entity/fieldtype"
	"github.com/hapyco/dygo/internal/entity/schema"
	"github.com/hapyco/dygo/internal/shape"
)

func TestCoreSelectValuesMatchEntityMetadata(t *testing.T) {
	tests := []struct {
		entity string
		field  string
		want   []string
	}{
		{entity: "app", field: "status", want: AppStatuses()},
		{entity: "session", field: "status", want: SessionStatuses()},
		{entity: "activity", field: "kind", want: ActivityKinds()},
		{entity: "activity", field: "operation", want: ActivityOperations()},
		{entity: "activity", field: "status", want: ActivityStatuses()},
		{entity: "log", field: "type", want: LogTypes()},
		{entity: "log", field: "source", want: LogSources()},
	}

	for _, tt := range tests {
		t.Run(tt.entity+"."+tt.field, func(t *testing.T) {
			entity := loadCoreEntity(t, tt.entity)
			field, ok := findField(entity, tt.field)
			if !ok {
				t.Fatalf("field %q not found on Entity %q", tt.field, tt.entity)
			}
			if field.Type != "select" {
				t.Fatalf("field %q type = %q, want select", tt.field, field.Type)
			}
			if !reflect.DeepEqual(field.Options.Values, tt.want) {
				t.Fatalf("field %q values = %#v, want %#v", tt.field, field.Options.Values, tt.want)
			}
		})
	}
}

func loadCoreEntity(t *testing.T, name string) schema.Entity {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "apps", "core", filepath.FromSlash(shape.EntityMetadataPath(name)))
	entity, err := schema.LoadFile(path, fieldtype.DefaultRegistry())
	if err != nil {
		t.Fatalf("LoadFile(%s) error = %v", path, err)
	}
	return entity
}

func findField(entity schema.Entity, name string) (schema.Field, bool) {
	for _, field := range entity.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return schema.Field{}, false
}

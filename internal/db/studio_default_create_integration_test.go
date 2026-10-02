package db

import (
	"context"
	"errors"
	"testing"

	"github.com/hapyco/dygo/internal/entity/schema"
	"gopkg.in/yaml.v3"
)

func TestPostgresStudioDefaultCreatePreservesFieldPermissionsAndHooks(t *testing.T) {
	pool, metadata := auditDatabase(t)
	entity := auditEntity("studio-default",
		schema.Field{Name: "status", Label: "Status", Type: "text", Required: true, Default: stringDefault("draft")},
		schema.Field{Name: "amount", Label: "Amount", Type: "int", Default: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "5"}},
		schema.Field{Name: "enabled", Label: "Enabled", Type: "boolean", Default: yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}},
	)
	entity.Entity.Naming = schema.Naming{Strategy: schema.NamingStrategyRandom, Length: 16}
	metadata.Entities = append(metadata.Entities, entity)
	syncAuditMetadata(t, pool, metadata)
	ctx := context.Background()
	var hookInputs []RecordInput
	hooks := NewRecordHookRegistry()
	if err := hooks.RegisterEntity("audit", "studio-default", RecordBeforeCreate, "observe-submitted-values", func(_ context.Context, h RecordHookContext) error {
		hookInputs = append(hookInputs, cloneRecordInput(h.Input))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	store := NewRecordStoreWithHooks(pool, hooks).WithScope(RecordScope{
		Where: "TRUE", FieldWrite: map[string]string{"status": "FALSE"},
	})
	_, err := store.CreateRecordByIdentity(ctx, "audit", "studio-default", recordInput(map[string]string{"status": `"draft"`}))
	var recordErr RecordError
	if !errors.As(err, &recordErr) || recordErr.Code != RecordErrorPermissionDenied {
		t.Fatalf("explicit protected default must be denied, got %v", err)
	}
	record, err := store.CreateRecordByIdentity(ctx, "audit", "studio-default", RecordInput{})
	if err != nil {
		t.Fatal(err)
	}
	if record["status"] != "draft" || record["amount"] != int32(5) || record["enabled"] != true {
		t.Fatalf("database defaults not preserved: %#v", record)
	}
	if len(hookInputs[len(hookInputs)-1]) != 0 {
		t.Fatalf("omitted defaults must retain empty-create hook semantics: %#v", hookInputs)
	}
	record, err = store.CreateRecordByIdentity(ctx, "audit", "studio-default", recordInput(map[string]string{"amount": "0", "enabled": "false"}))
	if err != nil {
		t.Fatal(err)
	}
	if record["amount"] != int32(0) || record["enabled"] != false || record["status"] != "draft" {
		t.Fatalf("explicit zero/false overwritten: %#v", record)
	}
	last := hookInputs[len(hookInputs)-1]
	if string(last["amount"]) != "0" || string(last["enabled"]) != "false" {
		t.Fatalf("hooks lost explicit inputs: %#v", last)
	}
}

func TestPostgresStudioRetainsDefaultedFormatNamingInputs(t *testing.T) {
	pool, metadata := auditDatabase(t)
	entity := auditEntity("studio-format", schema.Field{Name: "prefix", Label: "Prefix", Type: "text", Required: true, Default: stringDefault("INV")})
	entity.Entity.Naming = schema.Naming{Strategy: schema.NamingStrategyFormat, Format: "{prefix}"}
	metadata.Entities = append(metadata.Entities, entity)
	syncAuditMetadata(t, pool, metadata)
	store := NewRecordStoreWithHookPolicy(pool, RecordMutationHooksNone)
	record, err := store.CreateRecordByIdentity(context.Background(), "audit", "studio-format", recordInput(map[string]string{"prefix": `"INV"`}))
	if err != nil {
		t.Fatal(err)
	}
	if record["name"] != "INV" {
		t.Fatalf("defaulted naming token changed: %#v", record)
	}
}

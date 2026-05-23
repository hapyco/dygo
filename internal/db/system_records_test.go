package db

import (
	"context"
	"strings"
	"testing"
)

func TestSystemRecordWriterMutationStorePolicies(t *testing.T) {
	fullHooks := NewRecordHookRegistry()
	writer := SystemRecordWriter{store: NewRecordStoreWithHooks(&fakeRecordQueryer{}, fullHooks)}

	tests := []struct {
		name      string
		policy    SystemMutationPolicy
		wantHooks bool
		wantFull  bool
	}{
		{name: "bootstrap", policy: SystemMutationBootstrap},
		{name: "silent", policy: SystemMutationSilent},
		{name: "framework", policy: SystemMutationFramework, wantHooks: true},
		{name: "full", policy: SystemMutationFull, wantHooks: true, wantFull: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := writer.mutationStore("core", "user", tt.policy)
			if err != nil {
				t.Fatalf("mutationStore() error = %v, want nil", err)
			}
			if gotHooks := store.hooks != nil; gotHooks != tt.wantHooks {
				t.Fatalf("mutationStore() hooks present = %t, want %t", gotHooks, tt.wantHooks)
			}
			if tt.wantFull && store.hooks != fullHooks {
				t.Fatal("mutationStore(full) did not keep configured hooks")
			}
			if !tt.wantFull && store.hooks == fullHooks {
				t.Fatal("mutationStore() unexpectedly kept full hooks")
			}
		})
	}
}

func TestSystemRecordWriterRejectsInvalidPolicy(t *testing.T) {
	err := NewSystemRecordWriter(&fakeRecordQueryer{}).InsertByIdentity(context.Background(), "core", "user", RecordInput{}, SystemMutationPolicy("nope"))
	if err == nil || !strings.Contains(err.Error(), "system mutation policy is invalid") {
		t.Fatalf("InsertByIdentity(invalid policy) error = %v, want invalid policy", err)
	}
}

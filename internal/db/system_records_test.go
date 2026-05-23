package db

import (
	"context"
	"strings"
	"testing"
)

func TestSystemRecordWriterRejectsInvalidPolicy(t *testing.T) {
	err := NewSystemRecordWriter(&fakeRecordQueryer{}).InsertByIdentity(context.Background(), "core", "user", RecordInput{}, SystemMutationPolicy("nope"))
	if err == nil || !strings.Contains(err.Error(), "system mutation policy is invalid") {
		t.Fatalf("InsertByIdentity(invalid policy) error = %v, want invalid policy", err)
	}
}

package db

import (
	"strings"
	"testing"
)

func TestPGDumpConnectionRemovesPassword(t *testing.T) {
	got, password := pgDumpConnection("postgres://user:secret@127.0.0.1:5432/dygo?sslmode=disable")
	if password != "secret" {
		t.Fatalf("password = %q, want secret", password)
	}
	if strings.Contains(got, "secret") {
		t.Fatalf("connection string %q leaked password", got)
	}
	if !strings.Contains(got, "user@127.0.0.1") {
		t.Fatalf("connection string = %q, want user without password", got)
	}
}

package auth

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	t.Parallel()

	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword() error = %v, want nil", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("HashPassword() returned plaintext, want hash")
	}
	if err := ComparePassword(hash, "correct horse battery staple"); err != nil {
		t.Fatalf("ComparePassword() error = %v, want nil", err)
	}
	if err := ComparePassword(hash, "wrong"); err == nil {
		t.Fatal("ComparePassword() error = nil, want mismatch")
	}
}

func TestHashPasswordRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := HashPassword("  "); !errors.Is(err, ErrPasswordEmpty) {
		t.Fatalf("HashPassword(empty) error = %v, want ErrPasswordEmpty", err)
	}
	if _, err := HashPassword(strings.Repeat("a", 73)); !errors.Is(err, bcrypt.ErrPasswordTooLong) {
		t.Fatalf("HashPassword(too long) error = %v, want bcrypt.ErrPasswordTooLong", err)
	}
}

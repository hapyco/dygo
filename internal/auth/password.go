// Package auth contains shared authentication primitives.
package auth

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrPasswordEmpty reports an empty plaintext password.
	ErrPasswordEmpty = errors.New("password must not be empty")
)

// ValidatePassword validates one plaintext password before hashing.
func ValidatePassword(plaintext string) error {
	if strings.TrimSpace(plaintext) == "" {
		return ErrPasswordEmpty
	}
	if len([]byte(plaintext)) > 72 {
		return fmt.Errorf("hash password: %w", bcrypt.ErrPasswordTooLong)
	}
	return nil
}

// HashPassword hashes one plaintext password for storage.
func HashPassword(plaintext string) (string, error) {
	if err := ValidatePassword(plaintext); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// ComparePassword verifies plaintext against a stored bcrypt hash.
func ComparePassword(hash string, plaintext string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
}

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

// HashPassword hashes one plaintext password for storage.
func HashPassword(plaintext string) (string, error) {
	if strings.TrimSpace(plaintext) == "" {
		return "", ErrPasswordEmpty
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

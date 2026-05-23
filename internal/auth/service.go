package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dygo-dev/dygo/internal/corevalues"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	// ErrorInvalidRequest reports malformed auth input.
	ErrorInvalidRequest = "invalid_request"
	// ErrorInvalidCredentials reports failed credential authentication.
	ErrorInvalidCredentials = "invalid_credentials"
	// ErrorUnauthenticated reports a missing or invalid session.
	ErrorUnauthenticated = "unauthenticated"
	// ErrorAlreadyExists reports a bootstrap resource that already exists.
	ErrorAlreadyExists = "already_exists"
	// ErrorSchemaNotReady reports missing Core auth schema.
	ErrorSchemaNotReady = "schema_not_ready"
	// ErrorInternal reports an unexpected auth runtime failure.
	ErrorInternal = "internal_error"
)

const (
	defaultSessionTTL = 7 * 24 * time.Hour
	sessionTokenBytes = 32
)

// Queryer is the database behavior needed by the auth service.
type Queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// Service authenticates users and sessions from Core records.
type Service struct {
	queryer Queryer

	Now            func() time.Time
	TokenGenerator func() (string, error)
	SessionTTL     time.Duration
	SessionWriter  SessionWriter
	AdminWriter    AdminWriter
}

// User is the current authenticated Core user.
type User struct {
	ID            int64  `json:"id"`
	Email         string `json:"email"`
	FullName      string `json:"full-name"`
	Enabled       bool   `json:"enabled"`
	Administrator bool   `json:"administrator"`
}

// LoginRequest contains user credentials.
type LoginRequest struct {
	Email      string `json:"email"`
	Username   string `json:"username,omitempty"`
	Identifier string `json:"identifier,omitempty"`
	Password   string `json:"password"`
	Remember   bool   `json:"remember,omitempty"`
}

// LoginResult contains the authenticated user and raw session token.
type LoginResult struct {
	User      User
	Token     string
	ExpiresAt time.Time
}

// SessionInput is one authenticated login session to persist.
type SessionInput struct {
	UserID      int64
	TokenDigest string
	Status      string
	StartedAt   time.Time
	ExpiresAt   time.Time
	LastSeenAt  time.Time
}

// SessionWriter persists login sessions through the caller's framework mutation path.
type SessionWriter interface {
	CreateSession(context.Context, SessionInput) error
}

// AdminInput is the normalized first administrator account to persist.
type AdminInput struct {
	Email    string
	FullName string
	Password string
}

// AdminWriter persists the first administrator through the caller's framework mutation path.
type AdminWriter interface {
	SaveAdmin(context.Context, AdminInput) (User, error)
}

// SetupAdminInput contains first Administrator account details.
type SetupAdminInput struct {
	Email    string
	FullName string
	Password string
}

// Error reports stable auth runtime failures.
type Error struct {
	Code    string
	Message string
	Details map[string]any
	Err     error
}

func (e Error) Error() string {
	return e.Message
}

// Unwrap returns the underlying error.
func (e Error) Unwrap() error {
	return e.Err
}

// NewService returns an auth service backed by queryer.
func NewService(queryer Queryer) Service {
	return Service{queryer: queryer}
}

// Login verifies credentials, creates an active session, and returns the raw token.
func (s Service) Login(ctx context.Context, request LoginRequest) (LoginResult, error) {
	if err := s.requireQueryer(); err != nil {
		return LoginResult{}, err
	}
	identifier := firstNonEmpty(request.Email, request.Username, request.Identifier)
	email, err := normalizeEmail(identifier)
	if err != nil {
		return LoginResult{}, err
	}
	if strings.TrimSpace(request.Password) == "" {
		return LoginResult{}, authError(ErrorInvalidRequest, "password is required", nil, nil)
	}

	var user User
	var passwordHash sql.NullString
	err = s.queryer.QueryRow(ctx, `
SELECT id, email, full_name, COALESCE(enabled, false), COALESCE(administrator, false), password_hash
FROM "user"
WHERE lower(email) = lower($1)
LIMIT 1`, email).Scan(&user.ID, &user.Email, &user.FullName, &user.Enabled, &user.Administrator, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return LoginResult{}, invalidCredentials()
	}
	if err != nil {
		return LoginResult{}, classifyAuthDBError("load user credentials", err)
	}
	if !user.Enabled || !passwordHash.Valid || strings.TrimSpace(passwordHash.String) == "" {
		return LoginResult{}, invalidCredentials()
	}
	if err := ComparePassword(passwordHash.String, request.Password); err != nil {
		return LoginResult{}, invalidCredentials()
	}

	token, err := s.generateToken()
	if err != nil {
		return LoginResult{}, authError(ErrorInternal, "generate session token failed", nil, err)
	}
	now := s.now()
	expiresAt := now.Add(s.ttl())
	if err := s.createSession(ctx, SessionInput{
		UserID:      user.ID,
		TokenDigest: SessionTokenDigest(token),
		Status:      corevalues.SessionStatusActive,
		StartedAt:   now,
		ExpiresAt:   expiresAt,
		LastSeenAt:  now,
	}); err != nil {
		return LoginResult{}, classifyAuthDBError("create session", err)
	}

	return LoginResult{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// CurrentUser resolves a raw session token to an authenticated user.
func (s Service) CurrentUser(ctx context.Context, token string) (User, error) {
	if err := s.requireQueryer(); err != nil {
		return User{}, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return User{}, unauthenticated()
	}

	var sessionID int64
	var user User
	err := s.queryer.QueryRow(ctx, `
SELECT s.id, u.id, u.email, u.full_name, COALESCE(u.enabled, false), COALESCE(u.administrator, false)
FROM "session" s
JOIN "user" u ON u.id = s.user_id
WHERE s.token_digest = $1
	AND s.status = $2
	AND (s.expires_at IS NULL OR s.expires_at > now())
	AND COALESCE(u.enabled, false) = true
LIMIT 1`, SessionTokenDigest(token), corevalues.SessionStatusActive).Scan(&sessionID, &user.ID, &user.Email, &user.FullName, &user.Enabled, &user.Administrator)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, unauthenticated()
	}
	if err != nil {
		return User{}, classifyAuthDBError("load current session", err)
	}
	if _, err := s.queryer.Exec(ctx, `UPDATE "session" SET last_seen_at = now(), updated_at = now() WHERE id = $1`, sessionID); err != nil {
		return User{}, classifyAuthDBError("touch current session", err)
	}
	return user, nil
}

// Logout revokes a raw session token.
func (s Service) Logout(ctx context.Context, token string) error {
	if err := s.requireQueryer(); err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return unauthenticated()
	}
	if _, err := s.queryer.Exec(ctx, `
UPDATE "session"
SET status = $2, updated_at = now()
WHERE token_digest = $1 AND status = $3`, SessionTokenDigest(token), corevalues.SessionStatusRevoked, corevalues.SessionStatusActive); err != nil {
		return classifyAuthDBError("revoke session", err)
	}
	return nil
}

// SetupAdmin creates or promotes the special Administrator account.
func (s Service) SetupAdmin(ctx context.Context, input SetupAdminInput) (User, error) {
	if err := s.requireQueryer(); err != nil {
		return User{}, err
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return User{}, err
	}
	fullName := strings.TrimSpace(input.FullName)
	if fullName == "" {
		return User{}, authError(ErrorInvalidRequest, "full name is required", nil, nil)
	}
	if err := ValidatePassword(input.Password); err != nil {
		return User{}, authError(ErrorInvalidRequest, "password is invalid", nil, err)
	}

	var adminExists bool
	if err := s.queryer.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM "user" WHERE COALESCE(administrator, false) = true LIMIT 1)`).Scan(&adminExists); err != nil {
		return User{}, classifyAuthDBError("check administrator account", err)
	}
	if adminExists {
		return User{}, authError(ErrorAlreadyExists, "administrator account already exists", nil, nil)
	}
	if s.AdminWriter == nil {
		return User{}, authError(ErrorInternal, "auth admin writer is required", nil, nil)
	}
	user, err := s.AdminWriter.SaveAdmin(ctx, AdminInput{Email: email, FullName: fullName, Password: input.Password})
	if err != nil {
		return User{}, classifyAuthDBError("save administrator account", err)
	}
	return user, nil
}

// SessionTokenDigest returns the storage digest for a raw session token.
func SessionTokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GenerateSessionToken returns one high-entropy URL-safe session token.
func GenerateSessionToken() (string, error) {
	data := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("generate random session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// IsError reports whether err is an auth Error.
func IsError(err error) bool {
	var authErr Error
	return errors.As(err, &authErr)
}

func (s Service) requireQueryer() error {
	if s.queryer == nil {
		return authError(ErrorInternal, "auth queryer is required", nil, nil)
	}
	return nil
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s Service) ttl() time.Duration {
	if s.SessionTTL > 0 {
		return s.SessionTTL
	}
	return defaultSessionTTL
}

func (s Service) generateToken() (string, error) {
	if s.TokenGenerator != nil {
		return s.TokenGenerator()
	}
	return GenerateSessionToken()
}

func (s Service) createSession(ctx context.Context, input SessionInput) error {
	if s.SessionWriter == nil {
		return authError(ErrorInternal, "auth session writer is required", nil, nil)
	}
	return s.SessionWriter.CreateSession(ctx, input)
}

func normalizeEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	if email == "" {
		return "", authError(ErrorInvalidRequest, "email is required", nil, nil)
	}
	if strings.ContainsAny(email, " \t\r\n") || !strings.Contains(email, "@") {
		return "", authError(ErrorInvalidRequest, "email is invalid", nil, nil)
	}
	return email, nil
}

func invalidCredentials() error {
	return authError(ErrorInvalidCredentials, "invalid email or password", nil, nil)
}

func unauthenticated() error {
	return authError(ErrorUnauthenticated, "authentication required", nil, nil)
}

func classifyAuthDBError(message string, err error) error {
	if err == nil {
		return authError(ErrorInternal, message, nil, nil)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "42P01", "42703":
			return authError(ErrorSchemaNotReady, "auth schema is not ready; run dygo migrate", nil, err)
		}
	}
	return authError(ErrorInternal, message+" failed", nil, err)
}

func authError(code string, message string, details map[string]any, err error) Error {
	return Error{Code: code, Message: message, Details: details, Err: err}
}

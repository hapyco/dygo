package auth

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestServiceLogin(t *testing.T) {
	t.Parallel()

	passwordHash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	now := time.Date(2026, 5, 7, 12, 0, 0, 0, time.UTC)
	queryer := &fakeAuthQueryer{
		rows: []fakeAuthRow{
			rowValues(int64(7), "admin@example.com", "Admin User", true, true, sql.NullString{String: passwordHash, Valid: true}),
		},
	}
	service := NewService(queryer)
	service.Now = func() time.Time { return now }
	service.TokenGenerator = func() (string, error) { return "raw-session-token", nil }

	result, err := service.Login(context.Background(), LoginRequest{Email: "ADMIN@example.com", Password: "secret"})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if result.Token != "raw-session-token" {
		t.Fatalf("Login().Token = %q, want raw session token", result.Token)
	}
	if result.ExpiresAt != now.Add(defaultSessionTTL) {
		t.Fatalf("Login().ExpiresAt = %s, want default ttl", result.ExpiresAt)
	}
	if result.User.Email != "admin@example.com" || !result.User.Administrator {
		t.Fatalf("Login().User = %+v, want administrator", result.User)
	}
	if len(queryer.execArgs) != 1 {
		t.Fatalf("exec calls = %d, want session insert", len(queryer.execArgs))
	}
	args := queryer.execArgs[0]
	if containsArg(args, "raw-session-token") {
		t.Fatalf("session insert args leaked raw token: %#v", args)
	}
	if args[2] != SessionTokenDigest("raw-session-token") {
		t.Fatalf("session digest arg = %#v, want digest", args[2])
	}
}

func TestServiceLoginDeniesInvalidCredentials(t *testing.T) {
	t.Parallel()

	passwordHash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	tests := []struct {
		name     string
		row      fakeAuthRow
		password string
	}{
		{name: "unknown email", row: fakeAuthRow{err: pgx.ErrNoRows}, password: "secret"},
		{name: "wrong password", row: rowValues(int64(7), "admin@example.com", "Admin User", true, false, sql.NullString{String: passwordHash, Valid: true}), password: "wrong"},
		{name: "disabled user", row: rowValues(int64(7), "admin@example.com", "Admin User", false, false, sql.NullString{String: passwordHash, Valid: true}), password: "secret"},
		{name: "missing password hash", row: rowValues(int64(7), "admin@example.com", "Admin User", true, false, sql.NullString{}), password: "secret"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := NewService(&fakeAuthQueryer{rows: []fakeAuthRow{tt.row}})
			_, err := service.Login(context.Background(), LoginRequest{Email: "admin@example.com", Password: tt.password})
			assertAuthError(t, err, ErrorInvalidCredentials)
		})
	}
}

func TestServiceCurrentUser(t *testing.T) {
	t.Parallel()

	queryer := &fakeAuthQueryer{
		rows: []fakeAuthRow{
			rowValues(int64(99), int64(7), "admin@example.com", "Admin User", true, true),
		},
	}
	user, err := NewService(queryer).CurrentUser(context.Background(), "raw-session-token")
	if err != nil {
		t.Fatalf("CurrentUser() error = %v, want nil", err)
	}
	if user.ID != 7 || !user.Administrator {
		t.Fatalf("CurrentUser() = %+v, want administrator user", user)
	}
	if queryer.rowArgs[0][0] != SessionTokenDigest("raw-session-token") {
		t.Fatalf("CurrentUser() digest arg = %#v, want digest", queryer.rowArgs[0][0])
	}
	if len(queryer.execSQL) != 1 || !strings.Contains(queryer.execSQL[0], "last_seen_at") {
		t.Fatalf("CurrentUser() exec SQL = %#v, want last seen update", queryer.execSQL)
	}
}

func TestServiceCurrentUserRejectsInvalidSession(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeAuthQueryer{rows: []fakeAuthRow{{err: pgx.ErrNoRows}}})
	_, err := service.CurrentUser(context.Background(), "expired-or-revoked")
	assertAuthError(t, err, ErrorUnauthenticated)
}

func TestServiceLogout(t *testing.T) {
	t.Parallel()

	queryer := &fakeAuthQueryer{}
	if err := NewService(queryer).Logout(context.Background(), "raw-session-token"); err != nil {
		t.Fatalf("Logout() error = %v, want nil", err)
	}
	if len(queryer.execArgs) != 1 {
		t.Fatalf("Logout() exec calls = %d, want one", len(queryer.execArgs))
	}
	if queryer.execArgs[0][0] != SessionTokenDigest("raw-session-token") {
		t.Fatalf("Logout() digest arg = %#v, want digest", queryer.execArgs[0][0])
	}
}

func TestServiceSetupAdmin(t *testing.T) {
	t.Parallel()

	queryer := &fakeAuthQueryer{
		rows: []fakeAuthRow{
			rowValues(false),
			rowValues(int64(7), "admin@example.com", "Admin User", true, true),
		},
	}
	user, err := NewService(queryer).SetupAdmin(context.Background(), SetupAdminInput{
		Email:    "Admin@Example.com",
		FullName: "Admin User",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("SetupAdmin() error = %v, want nil", err)
	}
	if user.Email != "admin@example.com" || !user.Administrator || !user.Enabled {
		t.Fatalf("SetupAdmin() user = %+v, want enabled administrator", user)
	}
	args := queryer.rowArgs[1]
	if args[0] != "admin@example.com" || args[1] != "Admin User" {
		t.Fatalf("SetupAdmin() upsert args = %#v, want normalized email/full name", args)
	}
	hash, ok := args[2].(string)
	if !ok {
		t.Fatalf("SetupAdmin() password arg type = %T, want string", args[2])
	}
	if hash == "secret" {
		t.Fatal("SetupAdmin() stored plaintext password")
	}
	if err := ComparePassword(hash, "secret"); err != nil {
		t.Fatalf("SetupAdmin() password hash did not verify: %v", err)
	}
}

func TestServiceSetupAdminRejectsExistingAdministrator(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeAuthQueryer{rows: []fakeAuthRow{rowValues(true)}})
	_, err := service.SetupAdmin(context.Background(), SetupAdminInput{Email: "admin@example.com", FullName: "Admin", Password: "secret"})
	assertAuthError(t, err, ErrorAlreadyExists)
}

func TestServiceMapsSchemaNotReady(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeAuthQueryer{rows: []fakeAuthRow{{err: &pgconn.PgError{Code: "42703"}}}})
	_, err := service.Login(context.Background(), LoginRequest{Email: "admin@example.com", Password: "secret"})
	assertAuthError(t, err, ErrorSchemaNotReady)
}

type fakeAuthQueryer struct {
	rows     []fakeAuthRow
	execErr  error
	rowSQL   []string
	rowArgs  [][]any
	execSQL  []string
	execArgs [][]any
}

func (q *fakeAuthQueryer) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.rowSQL = append(q.rowSQL, sql)
	q.rowArgs = append(q.rowArgs, args)
	if len(q.rows) == 0 {
		return fakeAuthRow{err: pgx.ErrNoRows}
	}
	row := q.rows[0]
	q.rows = q.rows[1:]
	return row
}

func (q *fakeAuthQueryer) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	q.execSQL = append(q.execSQL, sql)
	q.execArgs = append(q.execArgs, args)
	if q.execErr != nil {
		return pgconn.CommandTag{}, q.execErr
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}

type fakeAuthRow struct {
	values []any
	err    error
}

func rowValues(values ...any) fakeAuthRow {
	return fakeAuthRow{values: values}
}

func (r fakeAuthRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("fake auth row destination count mismatch")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *bool:
			*target = r.values[i].(bool)
		case *int64:
			*target = r.values[i].(int64)
		case *string:
			*target = r.values[i].(string)
		case *sql.NullString:
			*target = r.values[i].(sql.NullString)
		default:
			return errors.New("unsupported fake auth scan destination")
		}
	}
	return nil
}

func assertAuthError(t *testing.T, err error, code string) {
	t.Helper()

	var authErr Error
	if !errors.As(err, &authErr) {
		t.Fatalf("error = %v, want auth Error", err)
	}
	if authErr.Code != code {
		t.Fatalf("auth error code = %q, want %q", authErr.Code, code)
	}
}

func containsArg(args []any, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}

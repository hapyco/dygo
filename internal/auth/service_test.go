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
	sessionWriter := &fakeSessionWriter{}
	service.SessionWriter = sessionWriter
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
	if len(queryer.execArgs) != 0 {
		t.Fatalf("exec calls = %d, want session writer to persist login", len(queryer.execArgs))
	}
	if len(sessionWriter.inputs) != 1 {
		t.Fatalf("session writer calls = %d, want one", len(sessionWriter.inputs))
	}
	sessionInput := sessionWriter.inputs[0]
	if sessionInput.UserName != "admin@example.com" {
		t.Fatalf("session user name = %q, want admin@example.com", sessionInput.UserName)
	}
	if sessionInput.TokenDigest == "raw-session-token" {
		t.Fatal("session writer received raw token")
	}
	if sessionInput.TokenDigest != SessionTokenDigest("raw-session-token") {
		t.Fatalf("session digest = %#v, want digest", sessionInput.TokenDigest)
	}
	if sessionInput.Status != "active" {
		t.Fatalf("session status = %q, want active", sessionInput.Status)
	}
	if sessionInput.StartedAt != now || sessionInput.LastSeenAt != now {
		t.Fatalf("session times = started %s last seen %s, want %s", sessionInput.StartedAt, sessionInput.LastSeenAt, now)
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
			service.SessionWriter = &fakeSessionWriter{}
			_, err := service.Login(context.Background(), LoginRequest{Email: "admin@example.com", Password: tt.password})
			assertAuthError(t, err, ErrorInvalidCredentials)
		})
	}
}

func TestServiceLoginRequiresSessionWriter(t *testing.T) {
	t.Parallel()

	passwordHash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	queryer := &fakeAuthQueryer{
		rows: []fakeAuthRow{
			rowValues(int64(7), "admin@example.com", "Admin User", true, true, sql.NullString{String: passwordHash, Valid: true}),
		},
	}
	service := NewService(queryer)
	service.TokenGenerator = func() (string, error) { return "raw-session-token", nil }

	_, err = service.Login(context.Background(), LoginRequest{Email: "admin@example.com", Password: "secret"})
	assertAuthError(t, err, ErrorInternal)
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
		},
	}
	adminWriter := &fakeAdminWriter{
		user: User{ID: 7, Email: "admin@example.com", FullName: "Admin User", Enabled: true, Administrator: true},
	}
	service := NewService(queryer)
	service.AdminWriter = adminWriter
	user, err := service.SetupAdmin(context.Background(), SetupAdminInput{
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
	if adminWriter.input.Email != "admin@example.com" || adminWriter.input.FullName != "Admin User" || adminWriter.input.Password != "secret" {
		t.Fatalf("SetupAdmin() admin writer input = %+v, want normalized email/full name and password", adminWriter.input)
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

type fakeSessionWriter struct {
	inputs []SessionInput
	err    error
}

func (w *fakeSessionWriter) CreateSession(_ context.Context, input SessionInput) error {
	w.inputs = append(w.inputs, input)
	return w.err
}

type fakeAdminWriter struct {
	input AdminInput
	user  User
	err   error
}

func (w *fakeAdminWriter) SaveAdmin(_ context.Context, input AdminInput) (User, error) {
	w.input = input
	return w.user, w.err
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

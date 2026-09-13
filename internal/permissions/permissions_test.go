package permissions

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TODO: Verify multi-role authorization with PostgreSQL; this fake checks action dispatch only.
func TestCheckerReturnsAllowedDecisionForAction(t *testing.T) {
	for _, action := range []Action{ActionRead, ActionDelete} {
		t.Run(string(action), func(t *testing.T) {
			queryer := &fakePermissionQueryer{row: fakePermissionRow{allowed: true}}

			decision, err := NewChecker(queryer).Check(context.Background(), Request{
				Actor:  Actor{UserID: 7},
				Entity: "user",
				Action: action,
			})
			if err != nil {
				t.Fatalf("Check() error = %v, want nil", err)
			}
			if !decision.Allowed || decision.Reason != ReasonAllowed || decision.Actor.UserID != 7 || decision.Entity != "user" || decision.Action != action {
				t.Fatalf("Check() decision = %+v, want allowed action on user", decision)
			}
			if err := NewChecker(queryer).Can(context.Background(), Request{Actor: Actor{UserID: 7}, Entity: "user", Action: action}); err != nil {
				t.Fatalf("Can() error = %v, want nil", err)
			}
			if len(queryer.sql) != 2 {
				t.Fatalf("Check()/Can() queries = %d, want 2", len(queryer.sql))
			}
			if !reflect.DeepEqual(queryer.args[0], []any{int64(7), "user"}) {
				t.Fatalf("Check() args = %#v, want user id and entity", queryer.args[0])
			}
			if !strings.Contains(queryer.sql[0], `COALESCE(p."`+string(action)+`", false) = true`) {
				t.Fatalf("Check() SQL = %q, want %s action column", queryer.sql[0], action)
			}
		})
	}
}

func TestCheckerAllowsAdministratorWithoutRolePermissionRows(t *testing.T) {
	queryer := &fakePermissionQueryer{row: fakePermissionRow{allowed: false}}

	decision, err := NewChecker(queryer).Check(context.Background(), Request{
		Actor:    Actor{UserID: 7, Administrator: true},
		Entity:   "user",
		Action:   ActionDelete,
		RecordID: 12,
	})
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if !decision.Allowed || decision.Reason != ReasonAllowed || !decision.Actor.Administrator || decision.RecordID != 12 {
		t.Fatalf("Check() decision = %+v, want administrator allowed with record id", decision)
	}
	if len(queryer.sql) != 0 {
		t.Fatalf("Check() executed SQL for administrator: %q", queryer.sql[0])
	}
	if err := NewChecker(nil).Can(context.Background(), Request{Actor: Actor{UserID: 7, Administrator: true}, Entity: "user", Action: ActionRead}); err != nil {
		t.Fatalf("Can() administrator error = %v, want nil without queryer", err)
	}
}

func TestCheckerAllowsAdministratorForPageResourceWithoutRolePermissionRows(t *testing.T) {
	decision, err := NewChecker(nil).CheckResource(context.Background(), ResourceRequest{
		Actor:    Actor{UserID: 7, Administrator: true},
		Resource: Resource{Kind: ResourcePage, App: "studio", Name: "home"},
		Action:   ActionRead,
	})
	if err != nil {
		t.Fatalf("CheckResource() error = %v, want nil", err)
	}
	want := Resource{Kind: ResourcePage, App: "studio", Name: "home"}
	if !decision.Allowed || decision.Resource != want || decision.Action != ActionRead {
		t.Fatalf("CheckResource() decision = %+v, want administrator allowed for %+v", decision, want)
	}
}

func TestCheckerChecksAppScopedPageResource(t *testing.T) {
	queryer := &fakePermissionQueryer{row: fakePermissionRow{allowed: true}}
	decision, err := NewChecker(queryer).CheckResource(context.Background(), ResourceRequest{
		Actor:    Actor{UserID: 7},
		Resource: Resource{Kind: ResourcePage, App: "studio", Name: "home"},
		Action:   ActionRead,
	})
	if err != nil || !decision.Allowed {
		t.Fatalf("CheckResource() = %+v, error %v, want allowed", decision, err)
	}
	if !strings.Contains(queryer.sql[0], `p.page_id IS NOT NULL`) || !strings.Contains(queryer.sql[0], `pg.key = $3`) {
		t.Fatalf("page permission SQL = %q, want page target joins", queryer.sql[0])
	}
	if !reflect.DeepEqual(queryer.args[0], []any{int64(7), "studio", "home"}) {
		t.Fatalf("page permission args = %#v, want user/app/page", queryer.args[0])
	}
}

func TestCheckerDenied(t *testing.T) {
	checker := NewChecker(&fakePermissionQueryer{row: fakePermissionRow{allowed: false}})

	decision, err := checker.Check(context.Background(), Request{
		Actor:  Actor{UserID: 7},
		Entity: "user",
		Action: ActionUpdate,
	})
	if err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	if decision.Allowed || decision.Reason != ReasonDenied {
		t.Fatalf("Check() decision = %+v, want denied", decision)
	}

	err = checker.Can(context.Background(), Request{Actor: Actor{UserID: 7}, Entity: "user", Action: ActionUpdate})
	assertPermissionError(t, err, ErrorDenied)
	if !IsDenied(err) {
		t.Fatalf("IsDenied(%v) = false, want true", err)
	}
}

func TestCheckerValidatesRequest(t *testing.T) {
	tests := []struct {
		name    string
		request Request
	}{
		{name: "invalid user id", request: Request{Actor: Actor{UserID: 0}, Entity: "user", Action: ActionRead}},
		{name: "empty entity", request: Request{Actor: Actor{UserID: 7}, Entity: " ", Action: ActionRead}},
		{name: "empty resource", request: Request{Actor: Actor{UserID: 7}, Resource: Resource{Kind: ResourcePage, App: "studio"}, Action: ActionRead}},
		{name: "page without app", request: Request{Actor: Actor{UserID: 7}, Resource: Resource{Kind: ResourcePage, Name: "home"}, Action: ActionRead}},
		{name: "unknown resource kind", request: Request{Actor: Actor{UserID: 7}, Resource: Resource{Kind: "widget", Name: "home"}, Action: ActionRead}},
		{name: "invalid action", request: Request{Actor: Actor{UserID: 7}, Entity: "user", Action: Action("drop_table")}},
		{name: "invalid record id", request: Request{Actor: Actor{UserID: 7}, Entity: "user", Action: ActionRead, RecordID: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryer := &fakePermissionQueryer{row: fakePermissionRow{allowed: true}}

			_, err := NewChecker(queryer).Check(context.Background(), tt.request)
			assertPermissionError(t, err, ErrorInvalidRequest)
			if len(queryer.sql) != 0 {
				t.Fatalf("Check() executed SQL for invalid request: %q", queryer.sql[0])
			}
		})
	}
}

func TestCheckerDatabaseFailureDoesNotLeakSensitiveDetails(t *testing.T) {
	queryer := &fakePermissionQueryer{
		row: fakePermissionRow{err: errors.New(`SELECT failed for postgres://secret@localhost/dygo`)},
	}

	_, err := NewChecker(queryer).Check(context.Background(), Request{
		Actor:  Actor{UserID: 7},
		Entity: "user",
		Action: ActionPrint,
	})
	assertPermissionError(t, err, ErrorInternal)
	if strings.Contains(err.Error(), "postgres://") || strings.Contains(err.Error(), "SELECT") {
		t.Fatalf("Check() error = %q, want no raw database details", err.Error())
	}
	if !errors.Is(err, queryer.row.err) {
		t.Fatalf("Check() error does not unwrap database failure")
	}
}

func TestParseAction(t *testing.T) {
	action, err := ParseAction(" read ")
	if err != nil {
		t.Fatalf("ParseAction(read) error = %v, want nil", err)
	}
	if action != ActionRead {
		t.Fatalf("ParseAction(read) = %q, want read", action)
	}
	custom, err := ParseAction("approve-leave")
	if err != nil || custom != Action("approve-leave") {
		t.Fatalf("ParseAction(approve-leave) = %q, %v, want custom action", custom, err)
	}
	if _, err := ParseAction("drop_table"); err == nil {
		t.Fatal("ParseAction(drop_table) error = nil, want invalid action")
	}
}

type fakePermissionQueryer struct {
	row  fakePermissionRow
	sql  []string
	args [][]any
}

func (q *fakePermissionQueryer) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	q.sql = append(q.sql, sql)
	q.args = append(q.args, args)
	return q.row
}

type fakePermissionRow struct {
	allowed bool
	err     error
}

func (r fakePermissionRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("expected one scan destination")
	}
	value, ok := dest[0].(*bool)
	if !ok {
		return errors.New("scan destination must be *bool")
	}
	*value = r.allowed
	return nil
}

func assertPermissionError(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want permission error %q", code)
	}
	var permissionErr Error
	if !errors.As(err, &permissionErr) {
		t.Fatalf("error = %T %v, want permissions.Error", err, err)
	}
	if permissionErr.Code != code {
		t.Fatalf("permission error code = %q, want %q", permissionErr.Code, code)
	}
}

package cli

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/hapyco/dygo/internal/shape"
)

func TestPermissionListCommand(t *testing.T) {
	root := newPermissionCLIProject(t)
	t.Chdir(root)

	fake := &fakePermissionRunner{
		grants: []permissionGrant{
			{Role: "system-manager", App: "sales", Entity: "lead", Actions: []permissions.Action{permissions.ActionRead, permissions.ActionUpdate}},
		},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithPermissionRunner(context.Background(), []string{"permission", "list", "sales/lead"}, strings.NewReader(""), &stdout, &stderr, fake)
	if err != nil {
		t.Fatalf("Run(permission list) error = %v, want nil", err)
	}
	if stdout.String() != "system-manager sales/lead read update\n" {
		t.Fatalf("permission list stdout = %q, want role grant", stdout.String())
	}
	if fake.listTarget == nil || *fake.listTarget != (shape.AppRef{App: "sales", Name: "lead"}) {
		t.Fatalf("permission list target = %+v, want sales/lead", fake.listTarget)
	}
}

func TestPermissionCheckCommandForUser(t *testing.T) {
	root := newPermissionCLIProject(t)
	t.Chdir(root)

	fake := &fakePermissionRunner{
		userDecision: permissions.Decision{Allowed: true, Reason: permissions.ReasonAllowed},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithPermissionRunner(context.Background(), []string{"permission", "check", "sales/lead", "read", "--user", "admin@example.com"}, strings.NewReader(""), &stdout, &stderr, fake)
	if err != nil {
		t.Fatalf("Run(permission check --user) error = %v, want nil", err)
	}
	if stdout.String() != "allow\n" {
		t.Fatalf("permission check stdout = %q, want allow", stdout.String())
	}
	if fake.user != "admin@example.com" || fake.action != permissions.ActionRead || fake.target != (shape.AppRef{App: "sales", Name: "lead"}) {
		t.Fatalf("permission check fake = user %q action %q target %+v, want user read sales/lead", fake.user, fake.action, fake.target)
	}
}

func TestPermissionExplainCommandForRole(t *testing.T) {
	root := newPermissionCLIProject(t)
	t.Chdir(root)

	fake := &fakePermissionRunner{
		roleDecision: permissions.RoleDecision{Allowed: false, Reason: permissions.ReasonDenied},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithPermissionRunner(context.Background(), []string{"permission", "explain", "sales/lead", "delete", "--role", "studio-member"}, strings.NewReader(""), &stdout, &stderr, fake)
	if err != nil {
		t.Fatalf("Run(permission explain --role) error = %v, want nil", err)
	}
	for _, want := range []string{
		"decision: deny",
		"subject: role studio-member",
		"entity: sales/lead",
		"action: delete",
		"reason: denied",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("permission explain stdout = %q, want substring %q", stdout.String(), want)
		}
	}
}

func TestPermissionCheckRequiresOneSubject(t *testing.T) {
	root := newPermissionCLIProject(t)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := runWithPermissionRunner(context.Background(), []string{"permission", "check", "sales/lead", "read"}, strings.NewReader(""), &stdout, &stderr, &fakePermissionRunner{})
	if err == nil {
		t.Fatal("Run(permission check) error = nil, want subject error")
	}
	if !strings.Contains(err.Error(), "pass exactly one of --user or --role") {
		t.Fatalf("permission check error = %q, want subject error", err.Error())
	}
}

func newPermissionCLIProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIConfig(t, root)
	writeCLIDatabaseSecret(t, root, secrets.EnvironmentDevelopment, "postgres://user:secret-password@localhost:5432/dygo")
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, cliEntityPath(root, "sales", "lead"), `
label: Lead
fields:
  - name: title
    label: Title
    type: text
`)
	return root
}

func runWithPermissionRunner(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, runner permissionRunner) error {
	return runWithServicesAndSetupAndFixturesAndHooks(ctx, args, stdin, stdout, stderr, noopServeRunner, noopDatabaseRunner(), &fakeSchemaSyncRunner{}, &fakeAdminSetupRunner{}, &fakeFixtureRunner{}, runner, nil)
}

type fakePermissionRunner struct {
	grants       []permissionGrant
	userDecision permissions.Decision
	roleDecision permissions.RoleDecision
	err          error

	listTarget  *shape.AppRef
	target      shape.AppRef
	action      permissions.Action
	user        string
	role        string
	root        string
	databaseURL string
}

func (r *fakePermissionRunner) List(_ context.Context, root string, databaseURL string, target *shape.AppRef) ([]permissionGrant, error) {
	r.root = root
	r.databaseURL = databaseURL
	if target != nil {
		copied := *target
		r.listTarget = &copied
	}
	return r.grants, r.err
}

func (r *fakePermissionRunner) CheckUser(_ context.Context, root string, databaseURL string, target shape.AppRef, action permissions.Action, user string) (permissions.Decision, error) {
	r.root = root
	r.databaseURL = databaseURL
	r.target = target
	r.action = action
	r.user = user
	return r.userDecision, r.err
}

func (r *fakePermissionRunner) CheckRole(_ context.Context, root string, databaseURL string, target shape.AppRef, action permissions.Action, role string) (permissions.RoleDecision, error) {
	r.root = root
	r.databaseURL = databaseURL
	r.target = target
	r.action = action
	r.role = role
	return r.roleDecision, r.err
}

package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hapyco/dygo/internal/db"
	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/secrets"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
)

type permissionRunner interface {
	List(context.Context, string, string, *shape.AppRef) ([]permissionGrant, error)
	CheckUser(context.Context, string, string, shape.AppRef, permissions.Action, string) (permissions.Decision, error)
	CheckRole(context.Context, string, string, shape.AppRef, permissions.Action, string) (permissions.RoleDecision, error)
}

type permissionGrant struct {
	Role    string
	App     string
	Entity  string
	Actions []permissions.Action
}

type defaultPermissionRunner struct{}

type permissionRuntimePool interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Close()
}

var openPermissionRuntimePool = func(ctx context.Context, databaseURL string) (permissionRuntimePool, error) {
	return db.OpenRuntimePool(ctx, databaseURL)
}

func newPermissionCommand(ctx context.Context, stdout io.Writer, runner permissionRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permission",
		Short: "Inspect live dygo permissions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newPermissionListCommand(ctx, stdout, runner))
	cmd.AddCommand(newPermissionCheckCommand(ctx, stdout, runner))
	cmd.AddCommand(newPermissionExplainCommand(ctx, stdout, runner))

	return cmd
}

func newPermissionListCommand(ctx context.Context, stdout io.Writer, runner permissionRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	cmd := &cobra.Command{
		Use:   "list [<app>/<entity>]",
		Short: "List live role permission grants",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			var target *shape.AppRef
			if len(args) == 1 {
				ref, err := shape.ParseAppRef(args[0])
				if err != nil {
					return err
				}
				target = &ref
			}
			env, root, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			grants, err := runner.List(ctx, root, databaseURL, target)
			if err != nil {
				return fmt.Errorf("list permissions: %w", err)
			}
			if len(grants) == 0 {
				if _, err := fmt.Fprintf(stdout, "No permission grants found (%s).\n", env); err != nil {
					return fmt.Errorf("write permission output: %w", err)
				}
				return nil
			}
			for _, grant := range grants {
				if _, err := fmt.Fprintf(stdout, "%s %s/%s %s\n", grant.Role, grant.App, grant.Entity, permissionActionList(grant.Actions)); err != nil {
					return fmt.Errorf("write permission output: %w", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	return cmd
}

func newPermissionCheckCommand(ctx context.Context, stdout io.Writer, runner permissionRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	var user string
	var role string
	cmd := &cobra.Command{
		Use:   "check <app>/<entity> <action>",
		Short: "Check whether a user or role is allowed",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			decision, err := runPermissionDecision(ctx, runner, envName, args, user, role)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(stdout, allowDeny(decision)); err != nil {
				return fmt.Errorf("write permission output: %w", err)
			}
			return nil
		},
	}
	addPermissionDecisionFlags(cmd, &envName, &user, &role)
	return cmd
}

func newPermissionExplainCommand(ctx context.Context, stdout io.Writer, runner permissionRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	var user string
	var role string
	cmd := &cobra.Command{
		Use:   "explain <app>/<entity> <action>",
		Short: "Explain a live permission decision",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			decision, err := runPermissionDecision(ctx, runner, envName, args, user, role)
			if err != nil {
				return err
			}
			return writePermissionExplanation(stdout, decision)
		},
	}
	addPermissionDecisionFlags(cmd, &envName, &user, &role)
	return cmd
}

type permissionDecision struct {
	Allowed bool
	Reason  string
	Subject string
	Entity  string
	Action  permissions.Action
}

func runPermissionDecision(ctx context.Context, runner permissionRunner, envName string, args []string, user string, role string) (permissionDecision, error) {
	target, err := shape.ParseAppRef(args[0])
	if err != nil {
		return permissionDecision{}, err
	}
	action, err := permissions.ParseAction(args[1])
	if err != nil {
		return permissionDecision{}, err
	}
	if strings.TrimSpace(user) == "" && strings.TrimSpace(role) == "" {
		return permissionDecision{}, fmt.Errorf("pass exactly one of --user or --role")
	}
	if strings.TrimSpace(user) != "" && strings.TrimSpace(role) != "" {
		return permissionDecision{}, fmt.Errorf("pass exactly one of --user or --role")
	}
	_, root, databaseURL, err := databaseInputs(envName)
	if err != nil {
		return permissionDecision{}, err
	}
	if strings.TrimSpace(user) != "" {
		decision, err := runner.CheckUser(ctx, root, databaseURL, target, action, user)
		if err != nil {
			return permissionDecision{}, fmt.Errorf("check user permission: %w", err)
		}
		return permissionDecision{
			Allowed: decision.Allowed,
			Reason:  decision.Reason,
			Subject: "user " + strings.TrimSpace(user),
			Entity:  target.App + "/" + target.Name,
			Action:  action,
		}, nil
	}
	decision, err := runner.CheckRole(ctx, root, databaseURL, target, action, role)
	if err != nil {
		return permissionDecision{}, fmt.Errorf("check role permission: %w", err)
	}
	return permissionDecision{
		Allowed: decision.Allowed,
		Reason:  decision.Reason,
		Subject: "role " + strings.TrimSpace(role),
		Entity:  target.App + "/" + target.Name,
		Action:  action,
	}, nil
}

func addPermissionDecisionFlags(cmd *cobra.Command, envName *string, user *string, role *string) {
	cmd.Flags().StringVar(envName, "env", *envName, "Environment: development, staging, or production")
	cmd.Flags().StringVar(user, "user", "", "User email or id")
	cmd.Flags().StringVar(role, "role", "", "Role name")
}

func writePermissionExplanation(stdout io.Writer, decision permissionDecision) error {
	if _, err := fmt.Fprintf(stdout, "decision: %s\n", allowDeny(decision)); err != nil {
		return fmt.Errorf("write permission output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "subject: %s\n", decision.Subject); err != nil {
		return fmt.Errorf("write permission output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "entity: %s\n", decision.Entity); err != nil {
		return fmt.Errorf("write permission output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "action: %s\n", decision.Action); err != nil {
		return fmt.Errorf("write permission output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "reason: %s\n", decision.Reason); err != nil {
		return fmt.Errorf("write permission output: %w", err)
	}
	return nil
}

func allowDeny(decision permissionDecision) string {
	if decision.Allowed {
		return "allow"
	}
	return "deny"
}

func permissionActionList(actions []permissions.Action) string {
	if len(actions) == 0 {
		return "(none)"
	}
	values := make([]string, len(actions))
	for index, action := range actions {
		values[index] = string(action)
	}
	return strings.Join(values, " ")
}

func (r defaultPermissionRunner) List(ctx context.Context, _ string, databaseURL string, target *shape.AppRef) ([]permissionGrant, error) {
	pool, err := openPermissionRuntimePool(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	defer pool.Close()

	args := []any{}
	filter := ""
	if target != nil {
		filter = "WHERE a.name = $1 AND e.key = $2"
		args = append(args, target.App, target.Name)
	}
	rows, err := pool.Query(ctx, `
SELECT r.name, a.name, e.key,
	COALESCE(p."read", false),
	COALESCE(p."create", false),
	COALESCE(p."update", false),
	COALESCE(p."delete", false),
	COALESCE(p."export", false),
	COALESCE(p."print", false)
FROM "permission" p
JOIN "role" r ON r.id = p.role_id
JOIN entity e ON e.id = p.entity_id
JOIN app a ON a.id = e.app_id
`+filter+`
ORDER BY r.name, a.name, e.key`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var grants []permissionGrant
	for rows.Next() {
		var grant permissionGrant
		var allowed [6]bool
		if err := rows.Scan(&grant.Role, &grant.App, &grant.Entity, &allowed[0], &allowed[1], &allowed[2], &allowed[3], &allowed[4], &allowed[5]); err != nil {
			return nil, err
		}
		for index, action := range permissions.SupportedActions() {
			if allowed[index] {
				grant.Actions = append(grant.Actions, action)
			}
		}
		grants = append(grants, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return grants, nil
}

func (r defaultPermissionRunner) CheckUser(ctx context.Context, root string, databaseURL string, target shape.AppRef, action permissions.Action, user string) (permissions.Decision, error) {
	pool, err := openPermissionRuntimePool(ctx, databaseURL)
	if err != nil {
		return permissions.Decision{}, err
	}
	defer pool.Close()

	slug, err := permissionEntitySlug(root, target)
	if err != nil {
		return permissions.Decision{}, err
	}
	actor, err := permissionActor(ctx, pool, user)
	if err != nil {
		return permissions.Decision{}, err
	}
	return permissions.NewChecker(pool).Check(ctx, permissions.Request{Actor: actor, Entity: slug, Action: action})
}

func (r defaultPermissionRunner) CheckRole(ctx context.Context, root string, databaseURL string, target shape.AppRef, action permissions.Action, role string) (permissions.RoleDecision, error) {
	pool, err := openPermissionRuntimePool(ctx, databaseURL)
	if err != nil {
		return permissions.RoleDecision{}, err
	}
	defer pool.Close()

	slug, err := permissionEntitySlug(root, target)
	if err != nil {
		return permissions.RoleDecision{}, err
	}
	return permissions.CheckRole(ctx, pool, role, slug, action)
}

func permissionEntitySlug(root string, target shape.AppRef) (string, error) {
	metadata, err := project.LoadMetadata(root)
	if err != nil {
		return "", err
	}
	entity, ok := findEntity(metadata.Entities, target)
	if !ok {
		return "", fmt.Errorf("entity %q not found", target.App+"/"+target.Name)
	}
	slug := entity.RouteSlug()
	if slug == "" {
		return "", fmt.Errorf("entity %q is not routeable", target.App+"/"+target.Name)
	}
	return slug, nil
}

func permissionActor(ctx context.Context, queryer permissions.Queryer, user string) (permissions.Actor, error) {
	user = strings.TrimSpace(user)
	if user == "" {
		return permissions.Actor{}, fmt.Errorf("user is required")
	}
	var id int64
	var administrator bool
	if parsed, err := strconv.ParseInt(user, 10, 64); err == nil {
		err := queryer.QueryRow(ctx, `SELECT id, COALESCE(administrator, false) FROM "user" WHERE id = $1`, parsed).Scan(&id, &administrator)
		if err != nil {
			return permissions.Actor{}, fmt.Errorf("lookup user %q: %w", user, err)
		}
		return permissions.Actor{UserID: id, Administrator: administrator}, nil
	}
	err := queryer.QueryRow(ctx, `SELECT id, COALESCE(administrator, false) FROM "user" WHERE email = $1`, user).Scan(&id, &administrator)
	if err != nil {
		return permissions.Actor{}, fmt.Errorf("lookup user %q: %w", user, err)
	}
	return permissions.Actor{UserID: id, Administrator: administrator}, nil
}

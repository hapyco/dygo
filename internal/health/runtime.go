package health

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Queryer is the database read behavior needed by runtime health checks.
type Queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// CheckResult is one runtime readiness check.
type CheckResult struct {
	Name   string
	Ready  bool
	Detail string
}

// CoreRuntimeChecks runs built-in checks required for a usable dygo runtime.
func CoreRuntimeChecks(ctx context.Context, queryer Queryer) []CheckResult {
	return []CheckResult{
		CheckCoreFixtures(ctx, queryer),
		CheckAdministratorAccount(ctx, queryer),
	}
}

// CheckCoreFixtures verifies the built-in roles and permissions needed by Studio.
func CheckCoreFixtures(ctx context.Context, queryer Queryer) CheckResult {
	var roleCount int
	if err := queryer.QueryRow(ctx, `SELECT COUNT(*) FROM "role" WHERE name IN ($1, $2)`, "studio-member", "system-manager").Scan(&roleCount); err != nil {
		return CheckResult{Name: "core access", Detail: fmt.Sprintf("check required roles: %v; run dygo db migrate", err)}
	}
	var permissionCount int
	if err := queryer.QueryRow(ctx, `SELECT COUNT(*) FROM "permission" WHERE COALESCE(retired, false) = false`).Scan(&permissionCount); err != nil {
		return CheckResult{Name: "core access", Detail: fmt.Sprintf("check permissions: %v; run dygo db migrate", err)}
	}

	var missing []string
	if roleCount < 2 {
		missing = append(missing, "roles")
	}
	if permissionCount == 0 {
		missing = append(missing, "permissions")
	}
	if len(missing) > 0 {
		return CheckResult{Name: "core access", Detail: fmt.Sprintf("missing Core %s; run dygo access apply", strings.Join(missing, " and "))}
	}
	return CheckResult{Name: "core access", Ready: true, Detail: fmt.Sprintf("%d roles and %d permissions ready", roleCount, permissionCount)}
}

// CheckAdministratorAccount verifies an administrator user has been set up.
func CheckAdministratorAccount(ctx context.Context, queryer Queryer) CheckResult {
	var adminExists bool
	if err := queryer.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM "user" WHERE COALESCE(administrator, false) = true LIMIT 1)`).Scan(&adminExists); err != nil {
		return CheckResult{Name: "administrator account", Detail: fmt.Sprintf("check administrator account: %v; run dygo db migrate then dygo setup", err)}
	}
	if !adminExists {
		return CheckResult{Name: "administrator account", Detail: "missing Administrator account; run dygo setup"}
	}
	return CheckResult{Name: "administrator account", Ready: true, Detail: "Administrator account exists"}
}

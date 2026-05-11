# Record Hooks

Record hooks let compiled app code extend dygo Record lifecycle behavior.

Hooks are Go code. dygo does not load Go files dynamically at runtime. A project binary must import the app hooks package and pass its registrar to the public runtime entrypoint.

## App Convention

Hook source files live under the app manifest's `paths.hooks` directory, which defaults to `hooks/`.

```txt
apps/crm/
  app.yml
  entities/
    lead.yml
  hooks/
    register.go
    lead.go
```

The file basename must match an Entity in the same app:

```txt
hooks/lead.go      hooks for Entity "lead"
hooks/register.go  package-level registrar
```

`*_test.go` files are allowed. Non-Go files and nested directories are ignored by v1 validation.

## Registration

The hooks package exposes one `Register` function:

```go
package hooks

import "github.com/dygo-dev/dygo/pkg/sdk"

func Register(registry sdk.RecordHookRegistry) error {
	if err := registerLeadHooks(registry); err != nil {
		return err
	}
	return nil
}
```

An Entity hook file registers hooks for its matching Entity:

```go
package hooks

import (
	"context"

	"github.com/dygo-dev/dygo/pkg/sdk"
)

func registerLeadHooks(registry sdk.RecordHookRegistry) error {
	return registry.RegisterEntity("lead", sdk.RecordBeforeCreate, "normalize-lead", normalizeLead)
}

func normalizeLead(ctx context.Context, hookCtx sdk.RecordHookContext) error {
	return nil
}
```

The project runner imports the hooks package and calls dygo through `pkg/sdk/runtime`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	crmhooks "example.com/my-project/apps/crm/hooks"
	"github.com/dygo-dev/dygo/pkg/sdk"
	dygoruntime "github.com/dygo-dev/dygo/pkg/sdk/runtime"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	err := dygoruntime.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, dygoruntime.Options{
		RecordHooks: []sdk.RecordHookRegistrar{crmhooks.Register},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

## Runtime Behavior

App hooks run after framework global hooks for the same Record event.

`dygo serve` and `dygo fixtures apply` use the compiled hook registry when run through a project binary built with `pkg/sdk/runtime`.

The stock `cmd/dygo` binary only includes framework hooks.

V1 app hooks are synchronous and transactional. There is no dynamic loading, hook priority, framework hook override, scripting hook, after-commit hook, or rollback hook support yet.

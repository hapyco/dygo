# dygo CLI

Source checked from `internal/cli` on 2026-05-25.

## Root

- `dygo` - Shows the root help for the metadata-driven dygo platform CLI.
- `dygo new <name>` - Creates a new dygo project skeleton.
- `dygo upgrade` - Upgrades the current dygo project to match the installed dygo binary.
- `dygo version` - Prints the dygo version.
- `dygo doctor` - Diagnoses the current dygo project.
- `dygo setup` - Runs the first-run project setup flow, including Administrator bootstrap until the UI wizard owns it.
- `dygo dev` - Runs the local development experience with backend, Studio dev server, proxying, and diagnostics.
- `dygo serve` - Starts the dygo server.

## Database

- `dygo db` - Groups database lifecycle commands.
- `dygo db check` - Checks PostgreSQL connectivity for the configured environment.
- `dygo db create` - Creates the configured PostgreSQL database.
- `dygo db drop` - Prints the drop target, prompts interactively, then drops the configured PostgreSQL database.
- `dygo db drop --yes` - Drops the configured PostgreSQL database without an interactive prompt.
- `dygo db migrate` - Prints the full migration plan, prompts interactively, then applies pre-sync patches, metadata sync, post-sync patches, fixtures, and schema dump.
- `dygo db migrate --yes` - Applies the full migration workflow without an interactive prompt.
- `dygo db migrate --dry-run` - Prints the full migration plan without writing.
- `dygo db prune` - Prints the metadata-orphaned schema cleanup plan, prompts interactively, then removes approved objects.
- `dygo db prune --yes` - Prints the cleanup plan and applies it without an interactive prompt.
- `dygo db prune --dry-run` - Previews metadata-orphaned schema cleanup without writing.
- `dygo db reset` - Prints the reset target, prompts interactively, then drops, creates, and migrates the configured PostgreSQL database.
- `dygo db reset --yes` - Drops, creates, and migrates the configured PostgreSQL database without an interactive prompt.
- `dygo db reset --dry-run` - Prints the reset target and planned steps without writing.

## Metadata And Apps

- `dygo app` - Groups dygo app commands.
- `dygo app list` - Lists discovered dygo apps.
- `dygo app validate` - Validates discovered dygo apps.
- `dygo entity` - Groups dygo Entity commands.
- `dygo entity list` - Lists discovered dygo Entities.
- `dygo entity validate` - Validates discovered dygo Entities.

## Fixtures

- `dygo fixture` - Groups app-owned fixture Record commands.
- `dygo fixture apply` - Prints the fixture apply plan, prompts interactively, then applies app-owned fixture Records.
- `dygo fixture apply --yes` - Applies app-owned fixture Records without an interactive prompt.
- `dygo fixture apply --dry-run` - Prints the fixture apply plan without writing or prompting.
- `dygo fixture validate` - Validates fixture files, match fields, dependencies, and references without connecting to the database when possible.
- `dygo fixture export <app>/<entity>` - Prints the fixture export plan, reports unresolved link dependencies, prompts interactively, then writes fixture files.
- `dygo fixture export <app>/<entity> --yes` - Exports selected Records without an interactive prompt.
- `dygo fixture export <app>/<entity> --include-links` - Exports selected Records and their linked fixture dependencies.
- `dygo fixture export <app>/<entity> --dry-run` - Prints the fixture export plan without writing or prompting.

## Hooks

- `dygo hook` - Groups hook inspection and maintenance commands.
- `dygo hook list` - Lists discovered hook packages, registered Entities, and supported lifecycle events.
- `dygo hook validate` - Validates hook files, Entity references, duplicate hook IDs, generated registrar files, and runner wiring.
- `dygo hook sync` - Updates generated project runner wiring for discovered app hook packages without creating a new hook file.

## Generate

- `dygo generate` / `dygo g` - Groups source scaffolding commands.
- `dygo generate app <app>` / `dygo g app <app>` - Generates a new app skeleton.
- `dygo generate entity <app>/<entity>` / `dygo g entity <app>/<entity>` - Generates Entity metadata.
- `dygo generate hook <app>/<entity>` / `dygo g hook <app>/<entity>` - Generates Entity hook scaffolding and project runner wiring.
- `dygo generate fixture <app>/<entity>` / `dygo g fixture <app>/<entity>` - Generates fixture file skeletons for an Entity.
- `dygo generate resource <app>/<entity>` / `dygo g resource <app>/<entity>` - Generates the normal app-owned Entity bundle.

## Routes

- `dygo route` - Groups route registry inspection and validation commands.
- `dygo route list` - Lists route owners, route kinds, and reserved paths.
- `dygo route validate` - Validates route conflicts and reserved-route usage.
- `dygo route resolve <path>` - Explains which route would handle a path.

## Permissions

- `dygo permission` - Groups permission inspection and validation commands.
- `dygo permission check` - Returns a concise allow or deny result for an action.
- `dygo permission explain <entity> <action>` - Explains why an action is allowed or denied.

## Secrets

- `dygo secret` - Groups encrypted dygo secret commands.
- `dygo secret init` - Initializes the root master key and encrypted secrets files.
- `dygo secret edit` - Edits decrypted secrets in an editor and re-encrypts them.
- `dygo secret validate` - Validates encrypted secrets and config references.
- `dygo secret rotate-key --confirm <project-name>/master.key` - Rotates the root master key and re-encrypts all secrets.

## Deferred Command Groups

- `dygo worker` - Defer until the durable job runtime is designed and implemented.
- `dygo scheduler` - Defer until schedule metadata and recurring job runtime exist.
- `dygo site` - Defer until site and tenant lifecycle storage/config are designed.
- `dygo generate` - Defer until project and metadata conventions settle enough for stable scaffolding.

## Out Of Band Binary Updates

- `curl -fsSL https://dygo.dev/install | sh` - Installs or updates the dygo binary outside the workspace CLI.
- `brew upgrade dygo` - Future package-manager-owned binary update path.
- `go install github.com/dygo-dev/dygo/cmd/dygo@latest` - Future Go toolchain-owned binary update path if dygo supports it.

Normal update flow:

```sh
curl -fsSL https://dygo.dev/install | sh
dygo upgrade
```

Until `dygo.dev/install` is wired:

```sh
curl -fsSL https://raw.githubusercontent.com/hapyco/dygo/main/scripts/install.sh | sh
dygo upgrade
```

## Removal And Replacement Notes

- Remove `dygo schema` - Prune moves to `dygo db prune`, and schema snapshot checking moves into `dygo doctor`.
- Replace `dygo schema prune` with `dygo db prune` - Prune is a database cleanup action; help text should clarify that it removes metadata-orphaned schema objects.
- Replace `dygo migrate` with `dygo db migrate` - Metadata schema sync is a database operation and belongs under the database command group.
- Replace `dygo migrate plan` with `dygo db migrate --dry-run` - Planning is the same migration workflow in preview mode, not a separate command group.
- Remove `dygo db prepare` - It only combines `dygo db create` and metadata sync; the explicit flow is clearer.
- Remove `dygo setup admin` - First-run setup should live behind `dygo setup`; the future path is a UI wizard rather than many setup subcommands.
- Remove public `dygo db schema` commands - `dygo db migrate` and `dygo db prune` refresh `db/schema.sql`, and `dygo doctor` should report whether the schema snapshot is missing or out of date.
- Remove public `dygo db schema dump` - Manual dumping can hide drift by making the snapshot match an unintended database state.
- Remove public `dygo patch` commands - Patches are part of the `dygo db migrate` workflow. Add direct patch controls later only if debugging or recovery needs a lower-level expert command.
- Keep explicit `dygo fixture` commands - `dygo db migrate` applies fixtures during the normal app-state workflow, while `dygo fixture apply`, `validate`, and `export` support app-author tooling and debugging.
- Remove `dygo upgrade --cli-only` - Binary updates are handled out of band through installer, package manager, or Go toolchain.
- Remove `dygo upgrade --install-dir` - Install location belongs to the installer, not the project upgrade command.
- Remove `dygo upgrade --project-only` - `dygo upgrade` is project-only by definition.
- Move `dygo serve --studio-dev-url` to `dygo dev` if the override is still needed - Studio/Vite proxying is local development orchestration, not runtime serving.

## Environment Safety Rule

- `--env` defaults to `development`.
- Write commands print the plan and prompt interactively by default when the action benefits from review.
- `--yes` skips the interactive prompt for agents, scripts, and CI.
- `--dry-run` prints the same plan and exits without writing or prompting.
- Destructive commands can run in `development`.
- Protected environments such as `staging` and `production` block destructive commands unless `--force` is passed.
- Read-only commands such as `--dry-run`, `check`, and validation commands never need `--force`.

# Access

Status: proposed decision log.

Current runtime still reads live Core `role`, `user-role`, and `permission` Records from the database. This document records the target authoring model before implementation.

## Decided

- Access is the umbrella name for app authorization metadata.
- App access metadata lives under `apps/<app>/access/`.
- App role vocabulary lives in `apps/<app>/access/_roles.yml`.
- Entity access metadata lives in `apps/<app>/access/<entity>.access.yml`.
- Use `_roles.yml`, not `roles.yml`, because it is an app-level access file inside a folder where normal files are Entity access files.
- Use `<entity>.access.yml`, not `<entity>.policy.yml`, because the file owns the full access contract for that Entity.
- Do not use `apps/<app>/permissions/` or `entities/<entity>/permissions.yml` for the target model.
- `_roles.yml` defines roles contributed by that app, not Entity grants.
- `_roles.yml` uses top-level `roles`.
- `_roles.yml` role items require `name` and `label`.
- `_roles.yml` role item `description` is optional.
- `<entity>.access.yml` defines access for one Entity.
- The first access section inside an Entity access file is singular `policy`.
- Do not use separate `permissions` and `policies` sections in v1.
- Keep `permission` as the Core runtime grant concept, but use `policy` as the access metadata section name.
- A v1 policy item has `role` and `can`.
- Do not add `description` to policy items in v1.
- Future conditional access can add `when` to policy items instead of adding a separate `policies` section.
- Role names are global.
- Role names are globally unique across all apps.
- Apps can define roles, but defining a role does not scope that role to the app.
- Access files can reference any known global role.
- Two apps cannot define the same role name unless dygo later adds an explicit shared-role ownership model.
- Role definitions do not record an owning app in the DB for v1.
- `role` remains a Core Entity in the database.
- Core `role.name` remains globally unique.
- `role` keeps `enabled` as the human/admin switch.
- Do not add `retired` to `role` in v1.
- `permission` remains a Core Entity in the database.
- Add `retired` to `permission` for file-backed grants that disappear from access metadata.
- Do not add `enabled` to `permission` in v1.
- The permission engine ignores retired grants.
- Core `role` and `permission` Records are runtime storage for access metadata, not ordinary fixture authoring.
- Runtime permission checks read database Records only.
- Access files are authoring metadata that create or update database permission Records through `dygo db migrate`.
- Policy items define full permission grants for `(entity, role)`.
- Access metadata does not model negative permission rules.
- Apps can define policy metadata for Entities owned by another app.
- Duplicate file-authored policy metadata for the same `(entity, role)` is invalid in v1.
- Do not merge duplicate policy metadata across apps in v1.
- Do not use app order, cascading, or last-writer-wins to resolve duplicate policy metadata in v1.
- Future replacement, versioning, and duplicate-record conflict behavior belongs to the broader metadata import system, not access-specific v1 rules.
- User-role assignments are runtime data, not app metadata.
- User-role assignments may still use explicit setup, demo, or environment fixtures until Studio/admin tooling owns them.
- `dygo access export` does not export user-role assignments.
- Normal user-role assignment changes belong to Studio, admin CLI, setup flows, or runtime code.
- User-role assignment fixtures are allowed for demo/setup data.
- Administrator remains a `user.administrator` flag, not a role.
- Base actions are `read`, `create`, `update`, `delete`, `export`, and `print`.
- In this access sprint, `policy.can` validates built-in actions only.
- Workflow-specific actions in `policy.can` are deferred to the Workflow sprint.
- File-to-database sync happens through `dygo db migrate`.
- Database-to-file export happens through `dygo access export`.
- `dygo access export` must receive an explicit destination app with `--in <app>`.
- Exported roles are written to `apps/<app>/access/_roles.yml` for the selected `--in` app.
- Role export only writes roles that are not already represented by any loaded `_roles.yml` file.
- Role export must not duplicate roles already contributed by another app.
- Do not add `dygo access import`.
- Do not add `dygo access apply`.
- Remove `dygo permission` from the public CLI instead of keeping it as a compatibility alias.
- Do not add access `check` or `explain` commands in this sprint.
- The first access CLI surface is source-metadata oriented: `validate`, `list`, `show`, `roles`, and `export`.
- `dygo generate app <app>` creates `apps/<app>/access/_roles.yml` by default.
- `dygo generate app <app> --no-access` skips the access folder.
- `dygo generate entity <app>/<entity>` creates `apps/<app>/access/<entity>.access.yml` by default.
- `dygo generate entity <app>/<entity> --no-access` skips the Entity access file.
- Generated access files are minimal skeletons.
- Entity generation does not create roles automatically.
- Once the access loader exists, fixture validate, apply, and export should reject app-owned Core `role` and `permission` Records as fixtures.
- Fixture validate, apply, and export should share one central fixture deny policy.
- The fixture deny policy should live under `internal/fixtures/`, not `internal/reserved/`.
- `internal/reserved/words.yml` stays limited to reserved naming collisions.
- Use one deny list for fixtures in v1; do not add restricted fixture categories yet.
- Collection Entities stay code-denied because that depends on Entity kind, not a static list.
- `core/user`, `core/user-role`, and `core/configuration` stay fixture-allowed for now.
- Core can keep using role and permission fixtures only as a bootstrap bridge until the access loader exists.
- After implementation, do one docs cleanup pass for stale `roles.yml`, `permissions.yml`, `permissions/`, and `dygo permission` references.
- Rename Entity metadata from `entities/<entity>/entity.yml` to `entities/<entity>/<entity>.entity.yml` before adding access file generators and loaders.
- The Entity metadata rename includes shape helpers, generators, validators, JSON Schemas, metadata loading, and docs.
- Reserved-name and fixture-eligibility extensions by apps are deferred beyond v1.
- Track app-extendable reserved and fixture policies in Roadmap item `#261`.
- Track broader metadata import conflict and versioning behavior in Roadmap item `#262`.
- Row-level access is deferred beyond v1.

## Pending

- Define exact permission export rules for Studio-authored database changes.
- Decide whether `dygo access export <app>/<entity> --in <app>` writes required permission changes only, or roles too.
- Decide how Studio-authored access changes export back to app metadata.

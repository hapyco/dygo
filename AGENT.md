# AGENT.md

## Product North Star

Build an open-source enterprise application platform for metadata-driven business software.

The platform should let developers define business objects, fields, relationships, permissions, forms, lists, reports, jobs, notifications, and module navigation as versioned app metadata. The runtime should turn that metadata into a usable operational workbench, document APIs, background work, audit trails, and installable business modules.

Do not treat the project as a clone of any existing framework. Use the concepts, but keep the implementation idiomatic to this repository.

## Core Principles

- Metadata is the platform kernel. It should define application shape, generated UI, permissions, reports, workflows, jobs, and installable module behavior.
- Code owns complex behavior. Metadata may configure shape and simple rules, but trusted code should own advanced business logic, integrations, and side effects.
- Permissions are a first-class runtime primitive. Every UI, API, export, report, file, realtime event, job, and background action must use the same permission model.
- App installation is a lifecycle, not a file copy. Validate, diff, migrate, load metadata, load fixtures, register jobs/hooks/assets, audit the change, and record installed versions.
- Extension order must be deterministic and inspectable. Prefer additive extensions over replacement. Ambiguous overrides should fail early.
- Schema changes should be controlled. Generate or apply explicit migration plans; do not let ordinary request handlers mutate the database schema.
- Uninstall should be conservative. Disable or archive modules by default and preserve data unless a destructive path is explicitly requested and backed up.
- The workbench is product UI, not a temporary admin screen. It should be useful, permission-aware, responsive, and optimized for repeated operational workflows.
- Enterprise operations matter from the beginning: audit, backups, health checks, migration ledgers, observability, upgrade safety, and supply-chain trust.

## Architecture Shape

Think of the platform as five connected products:

- Runtime server: HTTP, APIs, auth, permissions, metadata, jobs, realtime events, files, rendering, and storage adapters.
- Developer platform: CLI, app generator, metadata validator, migration planner, fixtures, packaging, SDK, and documentation.
- Operational workbench: generated forms, lists, reports, dashboards, navigation, notifications, comments, assignments, files, and system screens.
- Operations platform: app installs, upgrades, backups, restore, health checks, logs, job monitors, audit trails, and diagnostics.
- Extension ecosystem: metadata-only apps, trusted compiled extensions, isolated service extensions, and eventually sandboxed logic for limited hooks.

Keep the core implementation private until an API is deliberately stabilized. Anything exported for app authors becomes a long-term compatibility promise.

## Repository Guidance

- Keep the early repository as one coherent module unless a real versioning boundary appears.
- Put platform internals behind internal packages or equivalent private boundaries.
- Expose only a small app-author SDK: app registration, hooks, document service interfaces, job registration, storage interfaces, permission context, and event types.
- Do not expose raw database handles, internal metadata structs, internal transaction helpers, UI internals, or permission engine internals as public API.
- Prefer explicit registration over magic discovery.
- Keep app assets, metadata, migrations, fixtures, and hooks scoped to the owning app.

## App Model

Separate app availability from app installation.

Availability means an app bundle is present in the workspace, registry, or deployment artifact. Installation means a site has accepted that app, applied its migrations, loaded its metadata and fixtures, registered its hooks and jobs, and recorded its installed version.

An app bundle should be able to contain:

- app manifest
- document type metadata
- field definitions
- form/list/mobile/workspace views
- roles and permission rules
- reports and print formats
- workflow definitions
- migrations
- fixtures and reference records
- scheduled jobs
- notification rules
- assets
- trusted hooks or extension endpoints
- package digest, signature, provenance, and compatibility data when packaged

Required install flow:

1. Load the bundle.
2. Verify manifest, digest, signature, and compatibility where available.
3. Resolve dependencies.
4. Compare current and target metadata/schema.
5. Produce a migration and metadata plan.
6. Require backup or restore point for risky changes.
7. Acquire a site/app lock.
8. Apply migrations.
9. Load metadata.
10. Load fixtures.
11. Register hooks, jobs, routes, and assets.
12. Refresh caches.
13. Run smoke checks.
14. Write an immutable install ledger entry.
15. Emit audit events.

Disable/archive should stop jobs and hide navigation while preserving data. Destructive uninstall must require explicit confirmation, dependency checks, backup, dry run, and audit.

## Metadata And Documents

Document type metadata should define:

- table/storage name
- fields and field types
- required/default/read-only rules
- indexes
- child tables
- link fields
- naming strategy
- permissions
- form layout
- list layout
- mobile layout hints
- workflow states
- validation hooks
- lifecycle hooks
- export/import rules
- audit settings
- timeline settings
- notification triggers

Start with these field concepts:

- short text
- long text
- rich text
- integer
- decimal
- currency
- percent
- date
- datetime
- time
- checkbox
- select
- multi-select
- link
- dynamic link
- child table
- attachment
- image
- user
- role
- JSON
- virtual/computed field

Every document table should have standard platform fields for identity, owner, creator/updater, timestamps, version, status, soft deletion, and custom metadata.

Use a document service rather than forcing every dynamic record through hand-written model classes. The service should own lifecycle, validation, permissions, audit, transactions, and after-commit side effects.

Lifecycle:

```text
new -> validate -> before_insert -> insert -> after_insert
load -> validate -> before_save -> save -> after_save
submit -> before_submit -> set status -> after_submit
cancel -> before_cancel -> set status -> after_cancel
delete -> before_delete -> delete -> after_delete
```

Hooks may come from metadata rules, trusted code, isolated service extensions, or future sandboxed logic. Hooks must be time-bounded, traced, auditable, and clear about whether they run before or after permission filtering.

## Storage And Schema

Prefer physical tables for important document types and typed columns for declared fields. Use a JSON/custom extension area only for limited customization and low-traffic fields.

Schema changes should come from migration plans, not request-time DDL. A schema compiler should:

1. Validate app metadata.
2. Resolve dependencies.
3. Compare expected schema with actual schema.
4. Produce a plan.
5. Identify destructive changes.
6. Acquire locks.
7. Apply migrations.
8. Record checksums and versions.
9. Refresh metadata caches.

Forward-only migrations are the default. Destructive changes require explicit backup/checkpoint and should usually be staged by hiding or deprecating fields before dropping data.

## Permissions And Security

Build the permission engine early. Generic role libraries can inspire implementation, but the platform needs document-aware decisions.

Permission dimensions include:

- app
- module
- document type
- document
- field
- child table
- report
- action
- workflow transition
- file
- export
- import
- print
- share
- owner
- assignee
- team or department
- site or tenant

Permission checks must happen in:

- list query compilation
- document reads
- document writes
- field rendering
- field writes
- report execution
- file download
- export
- print
- workflow actions
- API response serialization
- realtime delivery
- background jobs acting for a user or system actor

Rules:

- Deny by default.
- Apply permission scopes in queries, not only after records are loaded.
- Apply field masking before serialization, export, print, or report output.
- UI permissions are presentation only; server checks are authoritative.
- Realtime events must be permission-filtered before delivery.
- Jobs must carry an explicit actor or system context.
- Audit permission-sensitive operations.

Audit records should include actor, site, session, request ID, IP/user agent where allowed, action, resource, before/after diff, permission decision, app version/digest, hook/extension source, and timestamp.

Never log passwords, tokens, raw secrets, private file contents, or unnecessary sensitive data.

## Workbench UI

The platform should ship a complete operational workbench.

Core screens:

- login and setup
- workspace/home
- app switcher
- module navigation
- global search and command palette
- document list view
- document form view
- child table editor
- file attachments
- comments and timeline
- assignments
- notifications
- report list
- report runner
- dashboards
- workflow actions
- user and role management
- permission matrix
- app install/update UI
- job monitor
- scheduler monitor
- audit log
- system health

Core generated components:

- field renderer
- field editor
- field error display
- link picker
- child table
- attachment picker
- form section
- form tab
- list filter
- list card
- list table
- report table
- chart wrapper
- action menu
- timeline event
- permission badge
- workflow state badge

Mobile and offline should be designed into the metadata model:

- mobile summary fields
- mobile field groups
- sticky primary actions
- bottom command bar
- drawer filters
- compact list cards
- scan/upload actions
- offline eligibility per document type/action
- offline shell
- cached recent records
- local draft forms
- queued simple mutations
- conflict-resolution UI

Never silently overwrite enterprise records after reconnect.

## APIs

Expose two API layers:

- Platform APIs used by the workbench and generic clients.
- App-specific APIs and extension endpoints.

Generic document API shape:

```text
GET    /api/docs/{type}
POST   /api/docs/{type}
GET    /api/docs/{type}/{name}
PATCH  /api/docs/{type}/{name}
DELETE /api/docs/{type}/{name}
POST   /api/docs/{type}/{name}/submit
POST   /api/docs/{type}/{name}/cancel
POST   /api/docs/{type}/{name}/actions/{action}
```

Metadata API shape:

```text
GET /api/meta/types
GET /api/meta/types/{type}
GET /api/meta/workspaces
GET /api/meta/forms/{type}
GET /api/meta/lists/{type}
```

Rules:

- Metadata defines allowed fields, filters, sorting, actions, and serialization.
- Permission scopes apply to list queries.
- Field masks apply to responses.
- State-changing requests must use state-changing HTTP verbs and appropriate CSRF/session/token protection.
- Bulk import/export requires explicit permission.
- Public API contracts should be described with a machine-readable spec when stable.
- Avoid generic graph-style APIs until the permission and query compiler are mature.

## Reports, Exports, And Print

Start with:

- saved list filters
- simple tabular reports
- permission-scoped query builder
- CSV/XLSX export
- background report generation
- downloadable report artifacts

Add later:

- report builder with filters, columns, grouping, aggregates, and charts
- trusted read-only SQL reports
- print format metadata
- HTML-to-PDF service
- report-specific print formats

Do not allow arbitrary end-user code or unrestricted SQL in production. Reports must be read-only, permission-scoped, field-masked, auditable, and safe for large data sets.

Use HTML as the canonical print format:

```text
document/report -> print metadata -> trusted template or constrained DSL -> HTML -> PDF backend -> file store
```

Print must respect field permissions, locale, timezone, watermarking, audit, and background generation requirements.

## Jobs, Scheduler, Notifications, And Realtime

Use durable jobs from the beginning.

Core job use cases:

- email send
- push send
- imports
- exports
- report generation
- PDF generation
- app install/update tasks
- webhooks
- integrations
- scheduled workflows
- file scanning
- search indexing

Scheduler should enqueue jobs, not execute work directly.

App schedule metadata should include name, schedule, queue, timeout, and handler reference.

Realtime should start with simple server-to-client event streams for notifications and progress. Add bidirectional sockets when collaboration, chat, live dashboards, or interactive builders require them.

Every event must be permission-filtered before delivery.

Notifications should support document events, workflow events, role/user recipients, digest preferences, push preferences, an in-app inbox, delivery logs, and queued sending.

## Files

Create a storage interface early. Support local development storage and production object storage through adapters.

Track file metadata:

- file id
- storage key
- provider/bucket
- owner
- attached document type/name
- visibility
- checksum
- content type
- size
- created timestamp
- retention policy
- scan status

File serving must be permission-aware. Private files need authenticated access, signed URLs or equivalent controls, expiry, and download audit. Enterprise controls should include file type policy, size policy, antivirus hook, legal hold, and retention support.

## Tenancy

Choose one tenancy model as the initial default and make that choice explicit. The product should not deeply support every tenancy model on day one.

Tenancy concerns:

- site registry
- host resolution
- database or row scope
- installed app set
- metadata cache
- permission cache
- file namespace
- job context
- backup/restore boundary
- migration boundary
- app rollout boundary

Site resolution concept:

```text
host -> site registry -> storage boundary -> installed app set -> metadata cache
```

If using many site-specific storage boundaries, bound connection pools carefully.

## Operations

Required operational features:

- app install ledger
- migration ledger
- migration dry run
- backup and restore
- health checks
- metrics
- traces
- structured logs
- admin audit
- job monitor
- scheduler monitor
- slow query visibility
- system health screen
- vulnerability scanning
- release signing
- app bundle signing
- compatibility matrix
- documented upgrade path
- disaster recovery docs

Upgrade flow:

1. Check target platform version.
2. Check installed app compatibility.
3. Check migrations.
4. Take and verify backup.
5. Apply platform migration.
6. Apply app migrations.
7. Refresh metadata.
8. Run smoke tests.
9. Emit audit and health events.

## Extension Runtime

Do not make native runtime code plugins the primary extension system.

Preferred extension tiers:

- metadata hooks for validations, notifications, workflows, and simple automations
- trusted compiled hooks registered at build time
- isolated service extensions for customer or third-party code
- sandboxed logic later for formulas, validation, transformations, and constrained workflow actions

Extension rules:

- Extensions receive scoped context, not raw storage credentials.
- Calls are time-limited and traced.
- Permission context is explicit.
- Failures are visible in workbench logs.
- App capabilities are declared.
- Marketplace-style apps require publisher identity, bundle signature, digest, SBOM/provenance, migration preview, and declared data access.

## CLI

The CLI is part of the product, not just developer convenience.

Expected command areas:

```text
serve
worker
scheduler
migrate
doctor

site create
site list
site backup
site restore
site migrate
site console

app new
app validate
app package
app install
app uninstall
app migrate
app list
app diff

generate document-type
generate migration
generate sdk

dev
dev ui
dev seed
```

The doctor command should check version, database connectivity, migration status, app compatibility, missing indexes, worker/scheduler health, storage access, queue backlog, disk space, secrets, TLS/debug posture, and other safety checks.

## Testing

Testing must match the platform risk.

Required test layers:

- unit tests for metadata validation
- unit tests for permission decisions
- field type mapping tests
- integration tests for migrations
- integration tests for app install/update/disable/uninstall
- query compiler tests
- API contract tests
- generated form/list UI tests
- mobile viewport tests
- offline draft/conflict tests
- job/scheduler tests
- realtime permission tests
- file permission tests
- audit tests
- race/concurrency tests where relevant
- fuzz tests for filters and query compilation
- security tests for permission bypass attempts

Must-have fixtures:

- multiple sites
- multiple apps
- app dependency graph
- conflicting migrations
- private fields
- child tables
- owner-only records
- workflow-restricted records
- report permission boundaries
- offline draft conflicts

## MVP Priorities

Prove the core loop before broadening scope:

1. Platform module and CLI.
2. Config loader.
3. Site or tenant registry.
4. Database connection and migration runner.
5. Core app manifest.
6. Installed app ledger.
7. Metadata parser and validator.
8. Document type registry.
9. Basic permission engine.
10. Audit log.
11. Structured logging and health checks.
12. Physical table per document type.
13. Field type mapping.
14. Schema compiler.
15. Generated migrations.
16. Document CRUD service.
17. Lifecycle hooks.
18. Field permissions.
19. Naming rules.
20. Child tables.
21. Fixtures.
22. Generated list view.
23. Generated form view.
24. Metadata API.
25. Generic document API.
26. Example business app.
27. App install/list/disable commands.
28. Durable jobs and scheduler.
29. Attachments.
30. Minimal docs.

## Explicit Non-Goals For Early Versions

- supporting every database
- runtime installation of arbitrary trusted-code plugins
- generic graph API
- full offline sync for every document type
- untrusted marketplace code execution
- broad low-code scripting with server privileges
- complex collaborative editing
- serverless-first deployment
- multiple first-class workbench frontend frameworks
- cloning every feature from prior art
- destructive uninstall as the default path

## Implementation Style

- Prefer simple, explicit, boring interfaces over magic.
- Keep metadata schemas versioned and validated.
- Make lifecycle decisions visible in ledgers and audit logs.
- Keep permissions central and hard to bypass.
- Keep migrations deterministic and repeatable.
- Add public API only when the contract is worth maintaining.
- Document architectural decisions as they are made.
- Let the smallest working platform kernel ship before adding builders, marketplace, advanced reports, or broad customization.

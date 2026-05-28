# Dygo Todo

## Rule

Use this as a flat local staging list. Track progress with Markdown checkboxes; when syncing, create a GitHub issue for any item that does not already have one, add it to the board, and remove synced checked items from this file.

Source: seeded from live GitHub Project 3 (Dygo 1.0) on 2026-05-27.

## Tasks

- [ ] #1 Build the platform CLI: Create the first stable CLI surface for local development, metadata validation, runtime operations, and future project scaffolding.
- [ ] #83 Add Studio standard page skeletons: Finish the remaining Studio page skeletons that are not already covered by the current record-focused page structure.
- [ ] #84 Add Studio metadata renderer foundation: Finish the Studio renderer foundation now that the first record list/form renderers exist.
- [ ] #121 Harden Studio dev proxy lifecycle: Make `dygo serve` development proxying quieter and more robust.
- [x] Add Studio user-menu Reload action: Add a Reload item to the top-right user menu that refreshes runtime Studio state without logging out, including boot, platform, metadata, records, and route remount state, while preserving browser-level user preferences for now. Keep the reload helper as the future extension point for DB-backed boot and preference refresh behavior.
- [ ] Add Core Preference entity for per-user UI and app state: Add a short `preference` Entity for private per-user lightweight state such as sidebar state, table density, hidden columns, default record view, app-specific UI defaults, and per-entity last-used choices. Keep `user` focused on account/profile data, keep global defaults in `configuration`, and split mature product concepts like saved views or notification settings into dedicated Entities later.
- [ ] Add Studio record list sort toolbar control: Turn the placeholder Sort icon in the page toolbar into a functional record-list sorting surface that can choose fields, directions, clear sort state, and sync with existing DataTable/query sorting behavior.
- [ ] Add Studio record page sidebar: Turn the placeholder Sidebar icon in the page toolbar into a contextual record-page sidebar for secondary panels such as view options, filters, activity, details, or future per-page tools.
- [ ] #51 Add Studio metadata browser: Add the first minimal Studio surface that reads runtime metadata and lets builders inspect installed Apps and Entities.
- [ ] #132 Design Studio record form layout metadata: Define how metadata-driven Record forms move beyond the current single-column fallback.
- [ ] #154 Support Link field traversal and fetch-from Entity field options: Allow Link fields to traverse related Records and allow field options or derived values to be fetched from another Entity field through explicit metadata.
- [ ] #169 Expose trusted system record writer API for app code: Make `is-system` a fully usable framework feature for app developers by exposing an explicit trusted-code writer API for system Entities.
- [ ] #2 Define global Studio surfaces: Design the first global Studio surfaces that every app can rely on.
- [ ] #5 Create the opinions directory: Add a dedicated place for project opinions, architectural decisions, and conventions.
- [ ] #7 Add project documentation links: Create the first documentation entry points for contributors and users.
- [ ] #21 Split field type registry files by responsibility: When `internal/entity/fieldtype` grows beyond the first registry implementation, split the package into responsibility-focused files instead of keeping all definitions in one file.
- [ ] #24 Add role inheritance for hierarchical permissions: Add hierarchical RBAC so one role can inherit permissions from another role.
- [ ] #31 Add worker command: Add the `dygo worker` command group for running background jobs once the jobs runtime exists.
- [ ] #32 Add scheduler command: Add the `dygo scheduler` command group for running recurring jobs once Schedule and Job runtime behavior exists.
- [ ] #33 Add site command group: Add a `dygo site` command group for future site and tenant lifecycle operations.
- [ ] #38 Add one-command local project setup: Add a one-command local setup path so a developer can bootstrap dygo without manually installing the CLI, creating the database, initializing secrets, and running checks step by step.
- [ ] #65 Add secret field type: Add a separate `secret` field type for encrypted, decryptable sensitive values such as API keys, webhook secrets, OAuth client secrets, service credentials, and integration tokens.
- [ ] #68 Add account invitations and password recovery: Add account invitation and password recovery flows for real Studio users.
- [ ] #69 Add OAuth and SSO auth providers: Add an OAuth/SSO integration path for teams that do not want local password-only login.
- [ ] #70 Add API-key authentication: Add developer- and agent-friendly API-key authentication after cookie-based Studio auth is stable.
- [ ] #72 Add Studio access gate and start page: Add the first post-login Studio entry point and access gate for normal users.
- [ ] #73 Add dedicated session management surface: Add a dedicated way to inspect and revoke user sessions without exposing session token digest fields through the generic Record API.
- [ ] #74 Add Studio user and role management: Add the first Studio management surface for users, roles, role assignments, and Entity permissions.
- [ ] #88 Add post-transaction Record hooks: Add after-commit and rollback-aware Record hook behavior for external side effects that must not run inside the database transaction.
- [ ] #89 Design hook priority and override policy: Decide whether app hooks can reorder, disable, or replace framework global hooks before dygo exposes override behavior.
- [ ] #98 Add secrets rotation recovery diagnostics: Add an explicit recovery and diagnostic path for interrupted secrets key rotation.
- [ ] #99 Design production secrets recipients and provider support: Define the next secrets architecture after the current single local `master.key` model.
- [ ] #103 Design actor-aware hook data access modes: Make hook data access modes explicit before dygo apps rely on implicit trusted access.
- [ ] #104 Design callable app actions and whitelisted method API: Define how dygo apps expose explicit server-side methods beyond Record CRUD and lifecycle hooks.
- [ ] #106 Clarify fixture Activity and history documentation: Remove drift between fixture documentation and current Record runtime behavior.
- [x] #117 Add Studio Record filtering UI and query sync: Make the Record list Filter action functional with a minimal metadata-backed filtering experience.
- [ ] #119 Design Studio breadcrumbs and navigation model: Define the durable Studio breadcrumb and navigation model before route labels spread across pages and custom apps.
- [ ] #122 Code-split Studio UI bundle: Reduce the initial Studio JavaScript bundle size as the UI grows.
- [ ] #124 Add Studio typed record cell renderers: Introduce typed Record cell rendering so Studio lists can display common metadata field types better than plain text.
- [ ] #125 Build Studio command palette actions and search: Turn the current Command-K shell into a usable Studio command palette instead of a static placeholder list.
- [ ] #126 Design page-specific Studio toolbars: Design how Studio page toolbars adapt across page types.
- [ ] #127 Design global Studio page tabs: Introduce a tab-like structure that lets users switch between open page sheets through global Studio tabs.
- [ ] #128 Design page-specific Studio sidebars: Design how Studio pages can provide contextual sidebars without conflicting with the global navigation sidebar.
- [ ] Polish Studio page header shell: Tighten the page header spacing, hierarchy, and action layout so it reads like a lighter app shell instead of a boxed header.
- [ ] Polish Studio toolbar shell: Refine the shared form/page toolbar spacing, alignment, controls, and visual weight so the toolbar feels consistent across page types.
- [ ] Polish Studio sidebar shell: Refine the shell sidebar spacing, section hierarchy, active state treatment, and density so navigation feels closer to the target reference.
- [ ] #130 Define Record rename and system name behavior: Decide and implement how existing Records handle name changes.
- [ ] #131 Add Studio form validation and field error mapping: Improve the Studio record form error path so validation failures land on the right fields instead of only showing a page-level save error.
- [ ] #133 Add Record tagging foundation: Add first-party tagging so users and apps can attach reusable tags to Records across Entities.
- [ ] #134 Add Record todo foundation: Add first-party todos that can be attached to Records so operational follow-ups live with the business object they belong to.
- [ ] #135 Add per-Record sharing foundation: Add a first-party sharing model so a user can grant other authenticated users or roles access to a specific Record.
- [ ] #138 Design durable Jobs and Queues architecture: Define the first dygo background job and queue model before implementation spreads across Core, SDK, CLI, Studio, and app runtime code.
- [ ] #139 Add Core Job metadata and Postgres queue storage: Add the durable Core storage that lets dygo persist, inspect, claim, retry, and audit background jobs.
- [ ] #140 Implement Job worker runtime, claiming, retries, and timeouts: Build the internal worker runtime that safely claims queued jobs, runs registered handlers, records attempts, and retries failed work.
- [ ] #141 Add SDK and internal Jobs APIs: Expose jobs through both public app SDK APIs and internal framework APIs without leaking storage details.
- [ ] #142 Add Job operations visibility and retry controls: Make background jobs inspectable and operable through dygo surfaces instead of leaving failures hidden in database rows.
- [ ] #143 Add job-backed importer foundation for data imports: Define and build the first importer foundation so dygo can import business data through durable jobs instead of long request lifecycles.
- [ ] #144 Add Studio keyboard shortcut foundation: Add a deliberate keyboard shortcut foundation for Studio so common navigation and record actions can become fast without ad hoc key handlers.
- [ ] #145 Add Studio Kanban view for Records: Add a metadata-driven Kanban view for Records so users can group records by a status-like field and move records across columns.
- [ ] #146 Add Studio Calendar view for Records: Add a metadata-driven Calendar view for date or datetime-backed Records.
- [ ] #147 Add Studio Gantt chart view for Records: Add a metadata-driven Gantt chart view for Records with date ranges.
- [ ] #148 Add Studio grid view for Records: Add a spreadsheet-style grid view for Records, distinct from the existing list table behavior.
- [ ] #150 Reject framework-reserved app names in projects: Dygo projects should not allow user-defined app names that collide with framework-owned app names. The reserved names should live in the framework, not in each project, so app authors do not accidentally shadow system concepts.
- [ ] #151 Make local console logging verbose, including Vite output: Make local development logs more useful by showing detailed framework/server logs and surfacing Vite logs in the same console flow.
- [ ] #156 Add Tree Entity support for hierarchical Records: Add first-class Tree Entity support so an Entity can model hierarchical Records with parent/child relationships, path traversal, and cycle-safe mutations.
- [ ] #162 Watch field type behavior contract shape: Keep the field type behavior contract useful without letting it become an unstructured cross-layer bag of flags.
- [ ] #171 Support environment-oriented fixtures: Make fixtures environment-oriented so dygo can apply different fixture sets for development, staging, and production without mixing bootstrap/demo data into the wrong environment.
- [ ] #176 Add dygo repair for generated project artifacts: Add `dygo repair` as the same-version maintenance command for generated dygo project artifacts.

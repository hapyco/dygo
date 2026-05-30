# Dygo Todo

Use this as a flat local staging list. Track progress with Markdown checkboxes; when we sync to GitHub later, create/update the matching issues and remove synced checked items from this file.

Source: local staging list for `hapyco/dygo`, last updated on 2026-05-30.

## Open issues

- [ ] #1 Build the platform CLI
- [ ] #2 Define global Studio surfaces
- [ ] #5 Create the opinions directory
- [ ] #7 Add project documentation links
- [ ] #21 Split field type registry files by responsibility
- [ ] #24 Add role inheritance for hierarchical permissions
- [x] #31 Add worker command
- [ ] #32 Add scheduler command
- [ ] #33 Add site command group
- [ ] #38 Add one-command local project setup
- [ ] #51 Add Studio metadata browser
- [ ] #65 Add secret field type
- [ ] #68 Add account invitations and password recovery
- [ ] #69 Add OAuth and SSO auth providers
- [ ] #70 Add API-key authentication
- [ ] #72 Add Studio access gate and start page
- [ ] #73 Add dedicated session management surface
- [ ] #74 Add Studio user and role management
- [ ] #83 Add Studio standard page skeletons
- [ ] #84 Add Studio metadata renderer foundation
- [ ] #194 Add Studio theming support and dark mode
- [ ] #88 Add post-transaction Record hooks
- [ ] #89 Design hook priority and override policy
- [ ] Add compiled hook registration discovery path for non-runtime hook introspection
- [ ] #98 Add secrets rotation recovery diagnostics
- [ ] #99 Design production secrets recipients and provider support
- [ ] #103 Design actor-aware hook data access modes
- [ ] #104 Design callable app actions and whitelisted method API
- [ ] #106 Clarify fixture Activity and history documentation
- [ ] #119 Design Studio breadcrumbs and navigation model
- [ ] #121 Harden Studio dev proxy lifecycle
- [ ] #122 Code-split Studio UI bundle
- [ ] #124 Add Studio typed record cell renderers
- [ ] #126 Design page-specific Studio toolbars
- [ ] #127 Design global Studio page tabs
- [ ] #128 Design page-specific Studio sidebars
- [ ] #195 Polish Studio command palette interactions and visual details
- [ ] #196 Add Studio command palette v2 with contextual actions and record search
- [ ] #197 Add Studio multi-tab navigation support
- [ ] #198 Add pinned section to Studio sidebar
- [ ] #130 Define Record rename and system name behavior
- [ ] #131 Add Studio form validation and field error mapping
- [ ] #132 Design Studio record form layout metadata
- [ ] #133 Add Record tagging foundation
- [ ] #134 Add Record todo foundation
- [ ] #135 Add per-Record sharing foundation
- [x] #138 Design durable Jobs and Queues architecture
- [x] #139 Add Core Job metadata and Postgres queue storage
- [x] #140 Implement Job worker runtime, claiming, retries, and timeouts
- [x] #141 Add SDK and internal Jobs APIs
- [ ] #142 Add Job operations visibility and retry controls
- [ ] #143 Add job-backed importer foundation for data imports
- [ ] Define removed `job.yml` retirement behavior for synced Core Job rows
- [ ] Add Core retention policy Entity for platform record cleanup
- [ ] #144 Add Studio keyboard shortcut foundation
- [ ] #199 Add custom Studio right-click context menus
- [ ] #145 Add Studio Kanban view for Records
- [ ] #146 Add Studio Calendar view for Records
- [ ] #147 Add Studio Gantt chart view for Records
- [ ] #148 Add Studio grid view for Records
- [ ] #156 Add Tree Entity support for hierarchical Records
- [ ] #162 Watch field type behavior contract shape
- [ ] #169 Expose trusted system record writer API for app code
- [ ] #171 Support environment-oriented fixtures
- [ ] Cache fetched-link traversal in record fetch to reuse shared path prefixes
- [ ] Add Link on-delete policies in metadata and schema planning (restrict/cascade/set-null)
- [ ] #176 Add dygo repair for generated project artifacts
- [ ] Make entity generation and hook runner writes atomic to avoid partial scaffolds
- [ ] #179 Add Core Preference entity for per-user UI and app state
- [ ] #182 Polish Studio page header shell
- [ ] #183 Polish Studio toolbar shell
- [ ] #184 Polish Studio sidebar shell
- [ ] #200 Polish Studio record list sidebar and activity rail
- [ ] #185 Add Studio record-list request race protection
- [ ] #186 Add Studio clear-filters action
- [ ] #187 Add typed Studio filter value controls
- [ ] #188 Add searchable Studio filter field picker
- [ ] #189 Polish Studio filter dirty state
- [ ] #190 Canonicalize invalid Studio filter URLs
- [ ] #201 Extract Studio record-list URL query canonicalization helper
- [ ] Debounce record-list route updates for filter changes to reduce replace churn
- [ ] #192 Support filtered Link field records
- [ ] #202 Add saved filters for Studio record lists

## Done

- [x] #150 Reject framework-reserved app names in projects
- [x] #151 Make local console logging verbose, including Vite output
- [x] #117 Add Studio Record filtering UI and query sync
- [x] #178 Add Studio user-menu Reload action
- [x] #180 Add Studio record list sort toolbar control
- [x] #181 Add Studio record page sidebar
- [x] #125 Build Studio command palette actions and search
- [x] #154 Support Link field traversal and fetch-from Entity field options
- [x] #203 Move Studio metadata queries from Pinia to TanStack Query
- [x] #204 Move Studio platform config from Pinia to TanStack Query
- [x] #205 Move Studio record form reads and mutations to TanStack Query
- [x] #206 Reduce Studio Pinia stores to shell and local UI state
- [x] #207 Simplify Studio reload and logout with TanStack Query cache operations
- [x] #208 Adopt @vueuse/core for Studio local UI utilities
- [x] #209 Use VueUse keyboard helpers for Studio command menu

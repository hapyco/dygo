# Directory Shape

## Framework

```txt
dygo/                           - Framework repository root
  cmd/                          - Framework binaries live here
    dygo/                       - Stock dygo CLI
  internal/                     - Private framework packages
    app/                        - App discovery and registry
    auth/                       - Session authentication
    cli/                        - Cobra command implementation
    config/                     - Config loading defaults
    corevalues/                 - Built-in metadata constants
    db/                         - PostgreSQL runtime layer
    entity/                     - Entity metadata catalog
    fixtures/                   - Fixture loading runtime
    health/                     - Health check handlers
    hookevents/                 - Hook event definitions
    hookgen/                    - Hook scaffold generator
    hooks/                      - Hook runtime registry
    naming/                     - Record naming strategies
    patches/                    - Explicit patch runtime
    permissions/                - Permission evaluation logic
    project/                    - Project root discovery
    projectgen/                 - Project scaffold generator
    recordquery/                - Record query helpers
    reserved/                   - Reserved name registry
    secrets/                    - Encrypted secrets runtime
    server/                     - HTTP server runtime
    studio/                     - Studio asset handling
    upgrade/                    - Project upgrade runtime
    yamlmeta/                   - YAML metadata helpers
  apps/                         - First-party dygo apps
    core/                       - Core platform app
    studio/                     - Studio web app
  pkg/                          - Public Go API surface
    sdk/                        - App hook SDK
  configs/                      - Framework config files
    secrets/                    - Encrypted dev secrets
  db/                           - Framework DB artifacts
    schema.sql                  - Framework schema snapshot
  docs/                         - Framework documentation
  schemas/                      - Editor JSON Schemas
  scripts/                      - Release and helper scripts
```

## Project

```txt
project/                         - Generated dygo project root
  dygo.yml                       - Project config and marker
  cmd/                           - Project binaries live here
    dygo/                        - Project dygo runner
      main.go                    - Hook-aware CLI entrypoint
  apps/                          - App packages live here
    <app>/                       - App package bundle
      app.yml                    - App manifest and paths
      entities/                  - App Entity definitions
        <entity>/                - Normal Entity bundle
          entity.yml             - Entity metadata definition
          hooks.go               - Entity hook scaffold
          fixtures.yml           - Entity fixture records
          permissions.yml        - Entity permission rules
          views.yml              - Entity view metadata
        _collections/            - Collection row definitions
          <collection>.yml       - Single-file collection metadata
          <collection>/          - Folder-form collection bundle
            entity.yml           - Collection metadata definition
      jobs/                      - App background jobs
        _schedules.yml           - Recurring job schedules
      pages/                     - Custom app pages
        <page>/                  - Custom page bundle
          page.yml               - Page metadata definition
      reports/                   - Cross-Entity report definitions
        <report>.yml             - Single-file report metadata
        <report>/                - Folder-form report bundle
          report.yml             - Report metadata definition
      roles.yml                  - App role definitions
  db/                            - Database generated artifacts
    schema.sql                   - PostgreSQL schema snapshot
  docs/                          - Project documentation files
  config/                        - Project config files
    secrets/                     - Encrypted environment secrets
    storage.yml                  - Future storage config
    queues.yml                   - Future queue config
    logging.yml                  - Future logging config
  .dygo/                         - Local dygo cache
```

## Runtime

```txt
deploy/                          - Deployed project root
  bin/                           - Compiled runner and hooks
    dygo                         - Project dygo runner binary
  config/                        - Deployment config files
    dygo.yml                     - Runtime project config
    secrets/                     - Mounted secret files
  apps/                          - App metadata files
    crm/                         - Deployed business app
      app.yml                    - App manifest
      entities/                  - Entity metadata files
      jobs/                      - Job metadata files
      pages/                     - Custom page files
      reports/                   - Report metadata files
      roles.yml                  - Role metadata file
  db/                            - Runtime DB artifacts
    schema.sql                   - Deployed schema snapshot
  studio/                        - Studio static assets
    dist/                        - Static UI bundle
  storage/                       - Persistent runtime storage
    files/                       - Uploaded/generated files
      public/                    - Public runtime files
      private/                   - Private runtime files
    logs/                        - Runtime log files
    tmp/                         - Runtime temporary files
  releases/                      - Versioned release bundles
  current/                       - Active release symlink
```

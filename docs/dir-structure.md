# Directory Structure

This document explains the proposed dygo repository structure. dygo has two major layers: the framework/platform layer, and apps built on top of that framework.

```txt
dygo/                                      # Root repository for the dygo framework and first-party apps.
  README.md                               # Project overview and positioning.
  LICENSE                                 # Open-source license for the project.
  go.mod                                  # Go module definition for the dygo codebase.
  go.sum                                  # Locked dependency checksums for Go modules.
  Makefile                                # Common developer commands for build, test, lint, and local setup.
  .gitignore                              # Files and folders Git should ignore.
  .env.example                            # Example local environment variables without real secrets.

  cmd/                                    # Executable entrypoints for dygo binaries.
    dygo/                                 # Main dygo CLI and server binary.
      main.go                             # Starts the CLI and wires commands to framework internals.

  internal/                               # Private framework internals that external apps should not import directly.
    app/                                  # App registry, installer, manifests, dependency resolution, and app lifecycle.
      kernel/                             # Core application kernel and boot process.
      registry/                           # Installed app registry and app lookup logic.
      installer/                          # App install, sync, update, and uninstall workflows.
      manifest/                           # App manifest parsing, validation, and compatibility checks.

    config/                               # Non-secret configuration loading and validation code.
    secrets/                              # Encrypted secrets and credential management.
    db/                                   # Database connectivity and persistence primitives.
    entity/                               # Metadata engine for Entity-style business objects.
    record/                               # Runtime record model built on top of entities.
    permissions/                          # Role, field, row, and action permission engine.
    modules/                              # Module loading and extension system for app-owned features.
    sites/                                # Site and tenant management.
    jobs/                                 # Background jobs, scheduled tasks, and workers.
    console/                              # Backend support for the dygo Console UI.
    files/                                # File storage and file access management.
    audit/                                # Audit logs, activity history, and security-relevant events.
    telemetry/                            # Metrics, tracing, health checks, and diagnostics.
    server/                               # HTTP server, routing, middleware, and request lifecycle.
    auth/                                 # Authentication, sessions, users, passwords, and identity adapters.
    utils/                                # Shared internal helpers that do not belong to a specific package.

  pkg/                                    # Public Go packages that trusted compiled apps may import.
    sdk/                                  # Stable SDK surface for app authors.

  apps/                                   # First-party and development-time dygo apps.
    core/                                 # Core app containing system entities, roles, and required metadata.
      app.yaml                            # App manifest for dependency, version, and install metadata.
      entities/                           # System entities such as User, Role, Site, File, and Installed App.
      modules/                            # Core module definitions and Space grouping.
      permissions/                        # Core permissions and default roles.
      fixtures/                           # Default records required by the core app.
      patches/                            # One-time data patches for the core app.
      migrations/                         # SQL migrations owned by the core app.
      views/                              # Default forms, lists, and Space views for core entities.
      jobs/                               # Core scheduled jobs and background task definitions.

    console/                              # First-party app that provides Console metadata and UI surfaces.
      app.yaml                            # App manifest for the Console app.
      entities/                           # Console-specific entities such as Space, View, Menu, and Report.
      modules/                            # Console module definitions.
      views/                              # Console forms, lists, and generated operational views.
      permissions/                        # Permissions for Console configuration and usage.

    examples/                             # Example apps used for development and documentation.
      crm/                                # Example CRM app built on top of dygo.

  ui/                                     # Frontend projects shipped with dygo.
    console/                              # Vue-based Console frontend.
      package.json                        # JavaScript package definition for the Console UI.
      vite.config.ts                      # Vite build configuration.
      index.html                          # Console frontend HTML entrypoint.
      src/                                # Vue source code for the Console UI.
        app/                              # Vue app bootstrap and providers.
        components/                       # Shared UI components.
        layouts/                          # Main Console layouts and shells.
        pages/                            # Route-level pages.
        router/                           # Vue Router configuration.
        stores/                           # Pinia stores or equivalent state management.
        modules/                          # Frontend module loaders and app-specific UI registration.
        views/                            # Metadata-driven view renderers for forms, lists, reports, and Spaces.
        api/                              # API client for dygo backend endpoints.
        styles/                           # Tailwind, tokens, and global styles.
      public/                             # Static assets for the Console frontend.

  entities/                               # Framework-level Entity definitions outside a specific app when needed.
    system/                               # System entities that define dygo's own runtime concepts.
    examples/                             # Small standalone entity examples for documentation or tests.

  views/                                  # Framework-level view definitions outside a specific app when needed.
    system/                               # System views for framework-owned entities.

  configs/                                # Safe, commit-friendly configuration files.
    dygo.yaml                             # Base dygo configuration.
    environments/                         # Environment-specific non-secret configuration.
      development.yaml                    # Development config.
      staging.yaml                        # Staging config.
      production.yaml                     # Production config.
    secrets/                              # ASCII-armored age-encrypted secret files and public recipients.
      development.age.yaml                # Encrypted development secrets.
      staging.age.yaml                    # Encrypted staging secrets.
      production.age.yaml                 # Encrypted production secrets.
      recipients/                         # Public age recipients safe to commit.
        development.txt                   # Development public recipient.
        staging.txt                       # Staging public recipient.
        production.txt                    # Production public recipient.

  .dygo/                                  # Local-only dygo state ignored by git.
    secrets/
      keys/                               # Private age identities ignored by git.
        development.agekey                # Development private identity.
        staging.agekey                    # Staging private identity.
        production.agekey                 # Production private identity.
      tmp/                                # Short-lived plaintext edit files ignored by git.

  sites/                                  # Site-specific runtime state and tenant configuration.
    default/                              # Default local site.
      site.yaml                           # Site identity, hostnames, database, timezone, and runtime settings.
      apps.yaml                           # Apps installed on this site and their install order.
      maintenance.yaml                    # Site-specific maintenance mode state and message.
      storage/                            # Site-owned uploaded files.
        public/                           # Public uploaded files for this site.
        private/                          # Private uploaded files for this site.
      logs/                               # Site-specific runtime logs.
      backups/                            # Site-specific backups.

  db/                                     # Database assets owned by the framework repository.
    migrations/                           # Core database migrations not owned by a single app.
      core/                               # Framework core migration files.
    seeds/                                # Seed data for local development and tests.
    snapshots/                            # Schema snapshots or generated database state captures.

  docs/                                   # Project documentation.
    architecture.md                       # High-level architecture and major system boundaries.
    roadmap.md                            # Product and engineering roadmap.
    directory-structure.md                # This file.
    entity-system.md                      # Entity and record model documentation.
    module-system.md                      # App/module system documentation.
    app-authoring.md                      # Guide for creating dygo apps.
    sites.md                              # Site and tenancy model documentation.
    deployment.md                         # Deployment and operations guide.

  examples/                               # Runnable examples outside first-party apps.
    apps/                                 # Example external apps.
    sites/                                # Example site configurations.

  scripts/                                # Helper scripts for local development and CI.
    dev.sh                                # Starts local development services.
    build-ui.sh                           # Builds the Vue Console frontend.
    migrate.sh                            # Runs migrations in local/dev contexts.

  deploy/                                 # Deployment templates and infrastructure examples.
    docker/                               # Docker and Compose deployment files.
      Dockerfile                          # Container image definition.
      compose.yaml                        # Local or simple production Compose stack.
    systemd/                              # systemd service templates.
    nginx/                                # Nginx reverse proxy examples.

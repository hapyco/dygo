# Doctrine

dygo is an opinionated Go framework for business software. Its job is to make the safe path obvious: define business metadata and behavior, then let the framework handle the platform foundation.

## Principles

1. Speed needs structure.
   Coding agents and generators are useful, but business systems need stable data models, permissions, auditability, and repeatable operations.

2. Builders should write business logic.
   Apps should define Entities, Records, permissions, hooks, fixtures, jobs, reports, and workflows. dygo should own the common platform pieces.

3. Opinion is a feature.
   File layout, CLI workflows, metadata shape, app installation, schema sync, and runtime behavior should be conventional unless there is a strong reason to escape.

4. Metadata must stay readable.
   App manifests, Entity metadata, fixtures, hooks, permissions, views, jobs, and reports should be clear to humans and coding agents.

5. Permissions come early.
   Server-side permission enforcement is the source of truth. UI checks are secondary. The default should be deny.

6. Important behavior should be observable.
   App installs, migrations, jobs, permission denials, file access, workflow changes, and runtime failures should be visible through logs, audit events, health checks, or Studio surfaces.

7. Studio is product UI.
   Studio is where people run and configure the business. Metadata-driven screens should still feel designed and consistent.

8. Enterprise-grade should not mean bloated.
   Add platform layers when they reduce real complexity. Avoid abstractions that do not yet carry framework weight.

9. Apps own the business.
   Framework code provides platform capability. Business-specific behavior belongs in apps.

10. Internals stay internal until they earn public API.
    Public API is a promise. Keep implementation private until app authors need a stable contract.

11. Agents are first-class builders.
    Docs, files, commands, output, and tests should be predictable enough that agents can make correct changes without guessing.

## Happy Path

The basic dygo loop should stay small:

```sh
dygo generate app <app>
dygo generate entity <app>/<entity>
dygo db prepare
dygo dev
```

The exact commands may evolve, but the workflow should remain clear: define metadata, prepare app state, run Studio, and build the business system inside a stable framework.

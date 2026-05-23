# AGENT.md

This file gives coding agents the product and engineering context needed to work on dygo.

## Product Philosophy

dygo is an opinionated Go framework for building serious business software.

It is designed for business processes, internal operating systems, enterprise apps, workflow-heavy products, and analytics-ready operational systems.

dygo embraces agentic development, but it must protect business software from the mess that agentic coding can create.

The goal is speed with structure.

## Core Belief

Builders should focus on business logic.

dygo should handle the foundation:

- app structure
- entities and records
- permissions
- jobs
- metadata schema sync
- app-owned patches
- logging
- auditing
- observability
- credentials
- site configuration
- maintenance mode
- consistent Studio UI
- secure APIs
- analytics-ready data patterns

Do not make implementers rebuild these pieces in every app.

## What dygo is not

dygo is not an AI slop generator.

dygo is not a vibe-coding playground.

dygo is not a generic admin template.

dygo is not a loose collection of helpers.

dygo is a framework with strong opinions and safe boundaries.

## Agentic Coding Position

dygo should praise and support agentic coding.

Agents should be able to build apps on top of dygo quickly by following strong conventions, clear docs, predictable file structure, and well-documented CLI commands.

But agents should not invent architecture casually.

When implementing features, prefer:

- explicit entity definitions
- clear manifests
- small focused files
- predictable naming
- secure defaults
- observable behavior
- testable boundaries
- boring reliability over cleverness

## Engineering Principles

### 1. Business logic belongs in apps

Framework code should provide reusable platform capability.

Business-specific behavior should live in apps built on top of dygo.

### 2. Framework internals stay internal

Use `internal/` for dygo implementation details.

Only expose stable public APIs through `pkg/sdk/`.

### 3. Everything important should be observable

When adding runtime behavior, consider:

- logs
- audit events
- metrics
- traces
- failure states
- admin visibility

Silent behavior is usually bad behavior.

### 4. Security and permissions are not optional

Business apps need permissions from the start.

Any record, API, job, file, report, or view that exposes business data must respect dygo's permission model.

### 5. Metadata should be explicit

dygo is metadata-driven, but metadata should not become mystery behavior.

Entity definitions, views, permissions, fixtures, app manifests, and jobs should be readable, diffable, and easy for humans and agents to understand.

### 6. Keep Studio consistent

Studio should feel like one coherent product.

Do not create one-off UI patterns unless the framework needs a new reusable pattern.

Prefer metadata-driven views, shared components, and consistent interaction models.

### 7. Enterprise-grade does not mean bloated

dygo should feel simple to use, but serious underneath.

Avoid unnecessary abstractions. Build strong primitives.

## Preferred Stack

Backend:

- Go
- PostgreSQL
- CLI-first workflows
- explicit config
- encrypted credentials
- app manifests
- schema-driven entities and records
- background jobs
- observability and auditability

## Product Vocabulary

Follow `docs/nomenclature.md`.

Use Studio for the main operational and builder UI. Use Space for a page or group inside Studio. Use Entity for business object definitions. Use Record for saved business data.

Core is the required system App. Studio is the first-party UI App. Business Apps define Entities, Permissions, Hooks, Fixtures, and Patches.

Entity metadata uses singular file-derived keys only. Do not add required display plural or storage plural metadata; storage naming comes from the Entity key converted from kebab-case to snake_case.

Use field-level `index` and `unique` only for single-field shorthands. Use top-level Entity `indexes` and `constraints` for composite indexes, composite uniqueness, and structured check constraints.

## Framework Dogfooding Rules

Framework internals should dogfood framework primitives wherever metadata is available.

If framework code needs a one-off path, first ask whether the one-off is actually a missing framework primitive. When the behavior is reusable across Records, metadata sync, fixtures, patches, hooks, permissions, Studio, or CLI, introduce or extend the framework-level primitive instead of hiding bespoke logic in one subsystem.

Prefer shared contracts and registries for:

- naming strategies
- field type behavior
- storage/system field naming
- metadata loading
- YAML metadata decoding
- permission actions
- patch operations
- hook events
- Record query parsing
- API envelopes and errors

Bootstrap exceptions are allowed, but they must be explicit, small, and documented in the code path that needs them.

`dygo migrate` stays additive and safe. `dygo schema prune` is the explicit destructive cleanup command for dygo's managed schema: metadata is source of truth, and prune may remove tables, columns, indexes, and constraints that exist in the managed schema but no longer exist in metadata. Do not keep long-lived unmanaged database objects in the managed schema; model them as metadata, clean them up in patches, or place them in another PostgreSQL schema.

## Implementation Guidance

When adding a feature, ask:

1. Is this framework capability or app-specific business logic?
2. Does this need permissions?
3. Does this need audit logging?
4. Does this need observability?
5. Does this need CLI support?
6. Does this need documentation?
7. Will an agent understand this structure later?
8. Is this safe for serious business software?

If the answer is unclear, choose the simpler and more explicit design.

## Project Skills

For Go CLI work, use `.agents/skills/go-cli-cobra-viper/SKILL.md`. It captures dygo's Cobra command patterns, when to introduce Viper-style config precedence, CLI output rules, and test expectations.

## Tone of the Codebase

dygo should feel:

- opinionated
- boring where it matters
- fast where it helps
- secure by default
- observable by default
- friendly to agents
- clean enough for enterprise systems
- simple enough for small business teams

## North Star

Use agentic coding for speed.

Use dygo for structure.

Build business software that is fast to create, safe to run, and clean enough to scale.

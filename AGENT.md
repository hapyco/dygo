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

- app/module structure
- entities and records
- permissions
- jobs
- migrations
- logging
- auditing
- observability
- credentials
- site configuration
- maintenance mode
- consistent Console UI
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

Business-specific behavior should live in apps/modules built on top of dygo.

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

### 6. Keep the Console consistent

The Console UI should feel like one coherent product.

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

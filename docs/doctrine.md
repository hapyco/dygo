# The dygo Doctrine

dygo is an opinionated framework for building serious business software in Go.

This doctrine explains the beliefs that should guide the framework, its APIs, its documentation, and the apps built on top of it.

## 1. Speed Needs Structure

Modern builders can move faster than ever.

Coding agents, generated code, and rapid iteration make it possible to build useful software in days instead of months. dygo should embrace that speed.

But business software is not disposable.

Business systems hold customer data, financial data, operational history, approvals, records, files, jobs, logs, permissions, and decisions people rely on. Speed without structure turns those systems into fragile piles of code.

dygo exists to keep the speed while adding the structure.

The framework should make the fast path safe, repeatable, and maintainable.

## 2. Builders Should Write Business Logic

Builders should spend their time on the business.

They should define customers, invoices, leads, tickets, work orders, approvals, roles, reports, dashboards, and domain-specific workflows.

They should not rebuild the same foundation in every project:

- command-line workflows
- app structure
- schema-driven entities and records
- metadata-driven schema sync
- permissions
- background jobs
- scheduling
- logging
- audit trails
- observability
- encrypted credentials
- site configuration
- maintenance mode
- secure APIs
- consistent operational UI

dygo should own the foundation so builders can own the domain.

## 3. Opinion Is A Feature

dygo should not be a box of unrelated libraries.

It should have a point of view about how business software is built: where files go, how apps are installed, how entities and records are modeled, how permissions are checked, how jobs run, how views are exposed, and how systems are operated.

These opinions are not there to restrict good teams. They are there to remove repeated decisions that do not need to be made again.

Good conventions reduce friction.

When a builder follows the dygo path, things should work with very little ceremony. When they need to leave the path, the escape hatch should be explicit and documented.

## 4. Metadata Should Be Readable

dygo is metadata-driven, but metadata must not become mystery.

Entity definitions, fields, views, permissions, jobs, fixtures, reports, workflows, and app manifests should be readable, diffable, and understandable by both humans and agents.

Metadata should answer:

- what exists
- who can see it
- who can change it
- how it appears in Studio
- how it moves through workflows
- what jobs or hooks act on it

If important behavior is hidden in unclear metadata or implicit magic, the framework has failed.

## 5. Permissions Come Early

Permissions are not an add-on.

Every serious business system eventually needs to answer who can read, create, update, delete, export, print, approve, assign, view, mask, attach, download, and automate.

dygo should treat permissions as a platform primitive from the beginning.

The same permission model must apply to:

- generated UI
- APIs
- reports
- exports
- files
- jobs
- realtime events
- workflows
- background actions

UI checks are not enough. Server-side enforcement is the source of truth.

The default should be deny.

## 6. Everything Important Should Be Observable

Business software should not behave silently.

When something important happens, dygo should make it visible through the right surface:

- logs
- audit events
- metrics
- traces
- job history
- system health
- admin screens
- failure states

Silent background behavior creates fear. Observable systems create trust.

If an app installs, a job fails, a report runs, a permission denies access, a file is downloaded, or a workflow changes state, there should be a way to understand what happened.

## 7. Studio Is Product UI

Studio is not a temporary admin panel.

It is where people run the business.

It should feel consistent, predictable, and built for repeated daily work. Lists, forms, filters, reports, files, comments, assignments, notifications, and permissions should share a coherent interaction model.

Studio should be generated from metadata where possible, but still feel designed.

Internal software deserves good product taste.

## 8. Enterprise-Grade Does Not Mean Bloated

dygo should be serious underneath and simple on the surface.

Enterprise-grade means reliable defaults, secure boundaries, metadata-driven schema sync, audit trails, observability, permissions, backups, and operational clarity.

It does not mean creating abstractions before they are needed.

Start with strong primitives. Add layers only when they reduce real complexity.

The framework should be powerful because its pieces fit together, not because every possible feature exists on day one.

## 9. Apps Own The Business

Framework code should provide reusable platform capability.

Business-specific behavior belongs in apps built on top of dygo.

This boundary matters.

The framework should know how to install an app, load its metadata, sync its schema, run its patches, enforce its permissions, expose its views, and schedule its jobs.

The app should know what the business is trying to do.

When that line stays clean, dygo can grow without becoming tangled with one business domain.

## 10. Internals Stay Internal Until They Earn Public API

Public API is a promise.

Most framework code should begin as private implementation. In Go, that means using `internal/` for implementation details and waiting before exposing stable packages.

Only promote APIs when app authors truly need them and the contract is worth maintaining.

The framework should move quickly internally without forcing early design mistakes onto every future app.

## 11. Agents Are First-Class Builders

dygo should be designed for humans and coding agents working together.

That means:

- clear documentation
- predictable file structure
- small focused files
- explicit entity definitions
- readable manifests
- consistent naming
- repeatable CLI workflows
- examples worth copying
- tests that describe behavior

Agents should not need to guess the architecture. The framework should make the right next step obvious.

Agentic development is not an excuse for messy systems. It is a reason to make the framework more explicit.

## 12. The Happy Path Should Be Obvious

A new builder should be able to understand the basic dygo loop quickly:

```sh
dygo app new crm
dygo generate entity Lead
dygo migrate
dygo serve
```

The exact commands may evolve, but the feeling should not.

The first success should be small, fast, and real: define something meaningful, run the app, see it in Studio, use the API, and know where the code lives.

Frameworks people love make users feel oriented.

## North Star

Use agentic coding for speed.

Use dygo for structure.

Build business software that is fast to create, safe to run, and clean enough to scale.

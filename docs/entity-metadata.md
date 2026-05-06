# Entity Metadata

Entities define business object structure in dygo.

The Entity catalog loads Entity files from discovered apps. During `dygo migrate` and `dygo db prepare`, dygo uses this metadata to create or update PostgreSQL tables. Core is not a separate schema path; Core tables come from `apps/core/entities/*.yml` the same way business app tables come from their Entity files.

Entity metadata is still the contract layer. Record CRUD, permission enforcement, Studio views, and runtime behavior are handled by later framework layers.

## Example

```yaml
name: lead
label: Lead
plural-name: leads
plural-label: Leads
description: Sales lead
fields:
  - name: full-name
    label: Full Name
    type: text
    required: true

  - name: email
    label: Email
    type: email
    unique: true

  - name: status
    label: Status
    type: select
    index: true
    options:
      values:
        - New
        - Qualified
        - Lost

  - name: company
    label: Company
    type: link
    options:
      entity: company

  - name: contacts
    label: Contacts
    type: child-table
    options:
      entity: lead-contact

indexes:
  - name: by-company-status
    fields: [company, status]

constraints:
  - type: unique
    fields: [company, status]

  - type: check
    field: status
    operator: in
    value:
      - New
      - Qualified
      - Lost
```

## Rules

Entity `name`, `label`, `plural-name`, `plural-label`, and at least one field are required.

Entity names, plural names, field names, and field type names use kebab-case.

`plural-name` is explicit. dygo does not auto-pluralize Entity names in runtime code because English pluralization is too fragile for schema decisions. Future generators may suggest a default, but the YAML must store the chosen plural name.

When dygo needs a SQL table name from Entity metadata, it converts `plural-name` from kebab-case to snake_case. For example, `user-roles` maps to `user_roles`.

Current metadata-driven schema sync supports scalar fields, `select`, and `link` fields. `child-table` parsing is supported, but child table storage is deferred until the record model is designed.

Field `name`, `label`, and `type` are required.

Field names must be unique inside an Entity.

`index: true` creates a non-unique database index for field types that support indexing. It is useful for fields commonly used in filters, lookups, joins, or status screens.

`unique: true` creates a single-field uniqueness rule.

Composite indexes and composite uniqueness are top-level Entity metadata, not field metadata.

`indexes` contains non-unique lookup/performance indexes:

```yaml
indexes:
  - fields: [status, created-at]
  - name: by-company-status
    fields: [company, status]
```

`constraints` contains composite uniqueness and structured checks:

```yaml
constraints:
  - type: unique
    fields: [user, role]

  - type: check
    field: amount
    operator: gte
    value: 0
```

Unique constraints require at least two fields. Single-field uniqueness should stay on the Field with `unique: true`.

Index and constraint names are optional. If omitted, dygo derives deterministic names from the Entity plural name, type, and fields. Provided names must use kebab-case and are converted to snake_case for PostgreSQL.

Supported check operators are `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, and `not-in`. Checks are single-field only in v1 and must use structured metadata, not raw SQL.

Check fields must be DB-backed scalar fields. `child-table`, `json`, `attachment`, and `link` checks are not supported in v1.

During `dygo migrate`, Field metadata is upserted into the Core `fields` table with name, label, type, required, unique, index, default, position, and options. Top-level Entity `indexes` and `constraints` are upserted into the Core `indexes` and `constraints` tables.

Type-specific settings live under `options`.

`select` fields require non-empty `options.values`.

`link` and `child-table` fields require `options.entity`.

## Built-In Field Types

```txt
text
email
phone
long-text
int
decimal
currency
boolean
date
datetime
time
select
link
child-table
attachment
json
```

Field types are registered in Go. App-defined field types in YAML are out of scope for v1.

## App Discovery

Entity files belong to an app's manifest-defined `entities` directory. By default, that directory is:

```txt
entities
```

dygo loads regular `*.yml` files from the immediate directory only. Missing `entities` directories are allowed for apps that do not define Entities yet.

Entity names are unique within the owning app for v1. Two different apps may use the same Entity name.

Validate discovered Entity metadata from the current project:

```sh
go run ./cmd/dygo entities list
go run ./cmd/dygo entities validate
```

`entities list` prints a tree grouped by app name.

`entities validate` checks Entity syntax, field types, duplicate Entity names within an app, and `link` or `child-table` targets.

Both commands discover the dygo project root before loading apps, so they can be run from nested directories inside a project.

`link` and `child-table` targets use Entity names in v1. A target is valid only when exactly one loaded Entity has that name. If no Entity matches, validation fails. If multiple apps define the same target name, validation fails as ambiguous until app-qualified references exist.

Validation errors include the app name, Entity name, field name when relevant, file path, and a best-effort YAML line number.

# Entity Metadata

Entities define business object structure in dygo.

The Entity catalog loads Entity files from discovered apps. During `dygo migrate` and `dygo db prepare`, dygo uses this metadata to create or update PostgreSQL tables. Core is not a separate schema path; Core tables come from `apps/core/entities/*.yml` the same way business app tables come from their Entity files.

Entity metadata is still the contract layer. The generic Record API reads persisted metadata and uses it to operate saved Records. Permission enforcement, Studio views, child table storage, and richer runtime behavior are handled by later framework layers.

## Example

```yaml
name: lead
label: Lead
description: Sales lead
route:
  slug: sales-lead
naming:
  strategy: series
  pattern: "LEAD-{YYYY}-{MM}-{#####}"
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
    check:
      operator: in
      value:
        - New
        - Qualified
        - Lost
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
```

## Rules

Entity `name`, `label`, and at least one field are required.

Entity names, field names, and field type names use kebab-case.

dygo uses singular Entity names only. There is no separate required metadata for display plurals or storage plurals.

`icon` is optional and should use a Lucide icon name, such as `box`, `user`, or `shield-check`. Studio resolves lower-kebab Lucide names and Vue component keys. Unknown icon names are non-fatal; Studio falls back to the Lucide `box` icon.

The stable internal Entity identity is `{app, entity}`. Two apps may define the same Entity name, such as `crm/contact` and `support/contact`.

The user-facing route slug is separate from that internal identity. `route.slug` is optional and defaults to Entity `name`. Route slugs must be globally unique across loaded apps and must not use Studio's reserved root slugs: `api`, `assets`, `health`, `login`, or `logout`. dygo fails validation on route slug conflicts instead of generating unstable numeric suffixes. If two apps both define `contact`, set one explicit slug, such as:

```yaml
route:
  slug: support-contact
```

Studio record pages use `/{route-slug}` at the root. The route does not prepend the app name unless the app author intentionally chooses that slug.

When dygo needs a SQL table name from Entity metadata, Core tables keep their historical singular names, such as `user` and `entity`. Non-Core app tables are app-scoped by default, so `crm/lead` maps to `crm_lead` and `support/contact` maps to `support_contact`.

Current metadata-driven schema sync supports scalar fields, `select`, `link`, and `password` fields. `child-table` parsing is supported, but child table storage is deferred until the record model is designed.

Field `name`, `label`, and `type` are required.

Field names must be unique inside an Entity.

Every metadata-backed Record has system fields:

```txt
id
name
created-at
updated-at
```

`id` is the internal numeric primary key. `name` is the stable system/business identifier. Entity `naming` metadata controls how `name` is created.

If `naming` is omitted, dygo uses random naming:

```yaml
naming:
  strategy: random
```

The effective default random length is `16`, generated with Go `crypto/rand` and a Base58-style alphabet. Builders may override the length:

```yaml
naming:
  strategy: random
  length: 24
```

Field-based naming copies a required, unique, stored, non-write-only field into the system `name` on create:

```yaml
naming:
  strategy: field
  field: email
```

A normal Field called `name` is allowed only when it is the naming source:

```yaml
naming:
  strategy: field
  field: name
```

Otherwise `name` is reserved as a system field and cannot appear under `fields`.

Series naming uses a pattern with date tokens and exactly one hash counter token:

```yaml
naming:
  strategy: series
  pattern: "SINV-{YYYY}-{MM}-{#####}"
```

Supported v1 series tokens are `{YY}`, `{YYYY}`, `{MM}`, and one counter token such as `{#####}`. The number of hashes controls zero-padding. Series counters are stored in Core `naming-series` Records and incremented transactionally.

Updating a field used for `naming.strategy: field` does not rename an existing Record. Explicit Record rename is future work.

`index: true` creates a non-unique database index for field types that support indexing. It is useful for fields commonly used in filters, lookups, joins, or status screens.

`unique: true` creates a single-field uniqueness rule.

`check` creates a single-field structured value rule:

```yaml
fields:
  - name: amount
    label: Amount
    type: currency
    check:
      operator: gte
      value: 0
```

Composite indexes and composite uniqueness are top-level Entity metadata, not field metadata.

`indexes` contains non-unique lookup/performance indexes:

```yaml
indexes:
  - fields: [status, created-at]
  - name: by-company-status
    fields: [company, status]
```

`constraints` contains composite uniqueness:

```yaml
constraints:
  - type: unique
    fields: [user, role]
```

Unique constraints require at least two fields. Single-field uniqueness should stay on the Field with `unique: true`.

Index and constraint names are optional. If omitted, dygo derives deterministic names from the Entity name, type, and fields. Provided names must use kebab-case and are converted to snake_case for PostgreSQL.

Supported field check operators are `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, and `not-in`. Field checks must use structured metadata, not raw SQL.

Check fields must be DB-backed scalar fields. `password`, `child-table`, `json`, `attachment`, and `link` checks are not supported in v1.

During `dygo migrate`, Entity naming metadata is upserted into the Core `entity` table. Field metadata is upserted into the Core `field` table with field-name, label, type, required, unique, index, default, check, position, and options. Top-level Entity `indexes` and `constraints` are upserted into the Core `index` and `constraint` tables.

Type-specific settings live under `options`.

`select` fields require non-empty `options.values`.

`link` and `child-table` fields require `options.entity`. `options.app` is optional. When omitted, dygo resolves the target in the current app first; otherwise the target Entity name must be globally unambiguous. Set `options.app` for cross-app links or ambiguous target names:

```yaml
options:
  app: support
  entity: contact
```

`link` fields are framework-level relationships. They create indexed storage columns and dygo validates linked Records at runtime, but v1 does not create database foreign key constraints for links.

## Built-In Field Types

```txt
text
email
phone
password
long-text
int
bigint
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

`password` fields are write-only Record fields. Metadata uses the clean field name, such as `password`, while storage uses a hash column, such as `password_hash`. Password fields cannot be indexed, unique, defaulted, or used in top-level indexes or constraints.

## App Discovery

Entity files belong to an app's manifest-defined `entities` directory. By default, that directory is:

```txt
entities
```

dygo loads regular `*.yml` files from the immediate directory only. Missing `entities` directories are allowed for apps that do not define Entities yet.

Entity identities are unique per app. Two different apps may use the same Entity name when their route slugs are unique.

Validate discovered Entity metadata from the current project:

```sh
go run ./cmd/dygo entities list
go run ./cmd/dygo entities validate
```

`entities list` prints a tree grouped by app name.

`entities validate` checks Entity syntax, field types, duplicate app-owned Entity identities, duplicate route slugs, `link` or `child-table` targets, and top-level `hooks/<entity>.go` filenames for each app.

Both commands discover the dygo project root before loading apps, so they can be run from nested directories inside a project.

`link` and `child-table` targets use `{app, entity}` identity when `options.app` is set. Without `options.app`, dygo resolves same-app targets first, then a single globally unambiguous target. If no Entity matches or multiple external apps match, validation fails.

Validation errors include the app name, Entity name, field name when relevant, file path, and a best-effort YAML line number.

# Entity Metadata

Entities define business object structure in dygo.

The Entity catalog loads Entity files from discovered apps. During `dygo migrate` and `dygo db prepare`, dygo uses this metadata to create or update PostgreSQL tables. Core is not a separate schema path; Core tables come from `apps/core/entities/` the same way business app tables come from their Entity files.

Entity metadata is still the contract layer. The generic Record API reads persisted metadata and uses it to operate saved Records. Permission enforcement, Studio views, collection row storage, and richer runtime behavior are handled by later framework layers.

## Example

```yaml
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
    type: collection
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

Entity identity comes from the file path. The YAML file must not contain top-level `name`.

The simple form `entities/<entity>.yml` defines Entity `<entity>`. The folder form `entities/<entity>/<entity>.yml` defines the same parent Entity. Both forms are equivalent, but an app cannot define both for the same Entity.

Entity names, field names, and field type names use kebab-case.

`label` and at least one field are required.

dygo uses singular Entity names only. There is no separate required metadata for display plurals or storage plurals.

`icon` is optional and should use a Lucide icon name, such as `box`, `user`, or `shield-check`. Studio resolves lower-kebab Lucide names and Vue component keys. Unknown icon names are non-fatal; Studio falls back to the Lucide `box` icon.

`is-single: true` marks an Entity as a singleton settings/config surface. Single Entities have exactly one framework-owned Record whose system `name` is the Entity name. dygo seeds that Record during metadata sync, Studio opens the form directly instead of a list, and normal create/delete/list operations are not used.

Single Entities cannot define explicit `naming`; dygo owns the singleton Record name. Every required stored field on a Single Entity must define a non-null default so `dygo migrate` can seed the row deterministically.

Single Entities cannot be targets of `link` or `collection` fields because there is no meaningful Record selection. A Single Entity may still contain link fields to normal Entities.

```yaml
label: Invoice Settings
is-single: true
fields:
  - name: default-due-days
    label: Default Due Days
    type: int
    required: true
    default: 30
```

The stable internal Entity identity is `{app, entity}`. Two apps may define the same Entity name, such as `crm/contact` and `support/contact`.

The user-facing route slug is separate from that internal identity. `route.slug` is optional and defaults to Entity `name`. Route slugs must be globally unique across loaded apps and must not use Studio's reserved root slugs: `api`, `assets`, `health`, `login`, or `logout`. dygo fails validation on route slug conflicts instead of generating unstable numeric suffixes. If two apps both define `contact`, set one explicit slug, such as:

```yaml
route:
  slug: support-contact
```

Studio record pages use `/{slug}` at the root. The route does not prepend the app name unless the app author intentionally chooses that slug.

When dygo needs a SQL table name from Entity metadata, Core tables keep their historical singular names, such as `user` and `entity`. Non-Core app tables are app-scoped by default, so `crm/lead` maps to `crm_lead` and `support/contact` maps to `support_contact`.

## Collection Entities

Collection row Entities are defined by folder position. Inside an Entity folder, the self-named YAML file defines the parent Entity. Any other `.yml` file in that folder defines a collection Entity.

```txt
entities/invoice/
  invoice.yml       -> invoice, standard Entity
  invoice-item.yml  -> invoice-item, collection Entity
  invoice-tax.yml   -> invoice-tax, collection Entity
```

The parent still declares usage with a `type: collection` field:

```yaml
fields:
  - name: items
    label: Items
    type: collection
    options:
      entity: invoice-item
```

Collection row Entities do not use `kind: collection`. Folder location implies collection ownership, and the filename still defines the Entity name. dygo does not automatically prefix collection Entity names.

A collection Entity must be referenced by exactly one collection field in its parent Entity file. If a collection file exists but the parent does not reference it, validation fails. If more than one parent field references it, validation fails.

Collection Entities are non-routeable, hidden from normal Studio navigation, cannot be targets of `link` fields, and must be same-app parent-owned collection targets in v1. Collection fields cannot target normal or Single Entities.

Special folder parent filenames are not supported: `entity.yml`, `_entity.yml`, and `index.yml`. Use the self-named parent file instead, such as `entities/invoice/invoice.yml`.

Current metadata-driven schema sync supports scalar fields, `select`, `link`, and `password` fields. `collection` parsing is supported, but collection row storage is deferred until the record collection model is implemented.

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

Template naming renders required stored fields into a deterministic name. Link field tokens render the linked Record's system `name`:

```yaml
naming:
  strategy: template
  template: "{app}.{key}"
```

Updating a field used for `naming.strategy: field` or `naming.strategy: template` does not rename an existing Record. Explicit Record rename is future work.

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

Check fields must be DB-backed scalar fields. `password`, `collection`, `json`, `attachment`, and `link` checks are not supported in v1.

During `dygo migrate`, Entity naming metadata is upserted into the Core `entity` table. Field metadata is upserted into the Core `field` table with field-name, label, type, required, unique, index, default, check, position, and options. Top-level Entity `indexes` and `constraints` are upserted into the Core `index` and `constraint` tables.

Type-specific settings live under `options`.

`select` fields require non-empty `options.values`.

`link` and `collection` fields require `options.entity`. For `link`, `options.app` is optional. When omitted, dygo resolves the target in the current app first; otherwise the target Entity name must be globally unambiguous. Set `options.app` for cross-app links or ambiguous target names:

```yaml
options:
  app: support
  entity: contact
```

`link` fields are framework-level relationships. They create indexed storage columns and dygo validates linked Records at runtime, but v1 does not create database foreign key constraints for links.

`collection` fields must target a collection Entity owned by the same parent Entity folder. Cross-app collection ownership is not supported in v1.

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
collection
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

dygo loads root `*.yml` files and one-level Entity folders. Missing `entities` directories are allowed for apps that do not define Entities yet.

Entity identities are unique per app. Two different apps may use the same Entity name when their route slugs are unique.

Moving a file without changing its basename does not move data because Entity identity is unchanged. Renaming a file changes Entity identity and requires explicit patch or migration handling.

Validate discovered Entity metadata from the current project:

```sh
go run ./cmd/dygo entities list
go run ./cmd/dygo entities validate
```

`entities list` prints a tree grouped by app name.

`entities validate` checks Entity syntax, path-derived names, field types, duplicate app-owned Entity identities, duplicate route slugs, `link` or `collection` targets, collection ownership, and top-level `hooks/<entity>.go` filenames for each app.

Both commands discover the dygo project root before loading apps, so they can be run from nested directories inside a project.

`link` targets use `{app, entity}` identity when `options.app` is set. Without `options.app`, dygo resolves same-app targets first, then a single globally unambiguous target. If no Entity matches or multiple external apps match, validation fails. `collection` targets must resolve to a same-app collection Entity owned by the current Entity folder.

Validation errors include the app name, Entity name, field name when relevant, file path, and a best-effort YAML line number.

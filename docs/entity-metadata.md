# Entity Metadata

Entities define business object structure in dygo.

The Entity catalog loads Entity files from discovered apps. During `dygo db migrate`, dygo uses this metadata to create or update PostgreSQL tables. Core is not a separate schema path; Core tables come from `apps/core/entities/` the same way business app tables come from their Entity files.

Entity metadata is the contract layer. The generic Record API reads persisted metadata and uses it to operate saved Records and parent-owned collection rows. Permission enforcement is active today; richer Studio views and runtime behavior are coming soon on the same metadata foundation.

## Example

```yaml
label: Lead
description: Sales lead
route:
  slug: sales-lead
name:
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

Entity identity comes from the file path. Top-level `name` configures Record system names; it does not define Entity identity.

Normal Entity metadata lives at `entities/<entity>/<entity>.entity.yml`. The parent folder defines the Entity key.

Entity keys, field names, and field type names use kebab-case.

`label` and at least one field are required.

dygo uses singular Entity keys only. There is no separate required metadata for display plurals or storage plurals.

`icon` is optional and should use a Lucide icon name, such as `box`, `user`, or `shield-check`. Studio resolves lower-kebab Lucide names and Vue component keys. Unknown icon names are non-fatal; Studio falls back to the Lucide `box` icon.

`is-system: true` makes an Entity readable through normal access rules but writable only through a trusted system Record writer. Studio, ordinary CRUD (including Administrator and `AsSystem`), fixtures, and imports cannot mutate its Records. App code uses the App-scoped [`Records.System(reason)` SDK](sdk.md#trusted-system-record-writes), or an App-owned post-sync [system Record patch](patches.md).

`is-private: true` marks an Entity as owner-scoped application state. `private-owner-field` must name a Link field that identifies the owner. Framework services use that metadata to keep private Records out of generic metadata and API surfaces and to apply the owner predicate consistently. Business App services can use the public SDK `RecordData.AsPrivate(actor, reason)` contract. Private storage is a reusable Entity contract; it is not tied to a particular App or Entity key.

`is-single: true` marks an Entity as a singleton settings/config surface. Single Entities have exactly one framework-owned Record whose system `name` is the Entity key. dygo seeds that Record during metadata sync, Studio opens the form directly instead of a list, and normal create/delete/list operations are not used.

Single Entities cannot define explicit `name` configuration; dygo owns the singleton Record name. Every required stored field on a Single Entity must define a non-null default so `dygo db migrate` can seed the row deterministically.

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

The stable internal Entity identity is `{app, key}`. Two apps may define the same Entity key, such as `crm/contact` and `support/contact`.

The user-facing route slug is separate from that internal identity. `route.slug` is optional and defaults to the Entity key. Route slugs must be globally unique across loaded apps and must not use Studio's reserved root slugs: `api`, `assets`, `boot`, `health`, `login`, `logout`, `me`, or `setup`. dygo fails validation on route slug conflicts instead of generating unstable numeric suffixes. If two apps both define `contact`, set one explicit slug, such as:

```yaml
route:
  slug: support-contact
```

Studio record pages use `/{slug}` at the root. The route does not prepend the app name unless the app author intentionally chooses that slug.

When dygo needs a SQL table name from Entity metadata, Core tables keep their historical singular names, such as `user` and `entity`. Non-Core app tables are app-scoped by default, so `crm/lead` maps to `crm_lead` and `support/contact` maps to `support_contact`.

## Tree Entities

Declare one parent Link to model a hierarchy:

```yaml
label: Department
tree:
  parent-field: parent
  label-field: title
fields:
  - name: title
    label: Title
    type: text
  - name: parent
    label: Parent
    type: link
    index: true
    options:
      entity: department
```

The parent must be an optional, indexed Link to the same App and Entity. Its foreign key must remain enabled. Do not give it a default, fetch rule, or uniqueness rule. Single and Collection Entities cannot be trees.

A null parent defines a root. Multiple roots are allowed. Every Record can have children. `label-field` is optional and must name a stored text field. Studio falls back to the Record name when the label is empty or unreadable.

Tree mutations use the normal Record pipeline. Self-parenting and cycles are rejected, including concurrent moves. A Record with children cannot be deleted. Move or delete its children explicitly first. A move requires update access to the Record and parent field, plus read access to the destination. Tree relationships do not grant permissions.

Schema preparation validates existing data before activating Tree metadata. Invalid hierarchies require an explicit data repair. No nested-set bounds, stored paths, or auxiliary tree tables are maintained.

Apply fixtures and import rows in parent-before-child order. Use parent Record names as Link values. Missing parents and cycles follow the existing fixture/import error and transaction rules; dygo does not reorder rows automatically.

## Collection Entities

Collection row Entities live in the app's `entities/_collections/` folder. Normal Entity folders under `entities/<entity>/` define routeable or Single Entities.

```txt
entities/
  invoice/
    invoice.entity.yml
    fixtures.yml
    hooks.go
  _collections/
    invoice-item.yml
    invoice-tax/
      invoice-tax.entity.yml
```

A parent declares usage with a `type: collection` field:

```yaml
fields:
  - name: items
    label: Items
    type: collection
    options:
      entity: invoice-item
```

Collection row Entities do not use `kind: collection`. The `entities/_collections/` path marks them as collection row Entities, and the file or folder name still defines the Entity key. dygo does not automatically prefix collection Entity keys.

Collection fields may reference same-app or cross-app collection Entities. Same-app references can omit `options.app`; cross-app references must set it:

```yaml
options:
  app: support
  entity: contact-row
```

A collection Entity may be reused by multiple parent Entities or multiple collection fields. The rows remain owned child rows, not shared Records; ownership is stored per row with the parent Entity, parent Record, and parent Field.

Collection Entities are non-routeable, hidden from normal Studio navigation, and cannot be targets of `link` fields. Collection fields cannot target normal or Single Entities. Collection-in-collection is not supported in v1.

Collection row Entities do not define top-level `name:` metadata. dygo assigns framework-owned random row names with length `16`, and inline collection editors do not expose `name` as an editable field.

Current metadata-driven schema sync supports scalar fields, `select`, `link`, `password`, `secret`, and collection row storage. Parent collection fields are virtual and do not create a parent table column. The child collection table stores `id`, `name`, `created_at`, `updated_at`, `parent_entity_id`, `parent_record_id`, `parent_field_id`, `ordinal`, and the child Entity's own stored field columns. `parent_entity_id` is an FK to Core `entity`, `parent_field_id` is an FK to Core `field`, and `parent_record_id` is a bigint because the parent table depends on `parent_entity_id`. dygo deletes child rows transactionally when deleting the parent Record. `(parent_entity_id, parent_record_id, parent_field_id, ordinal)` is unique with a deferrable initially deferred constraint, and `(parent_entity_id, parent_record_id, parent_field_id)` is indexed for ordered child row reads.

Field `name`, `label`, and `type` are required.

Field names must be unique inside an Entity.

Fields can copy a stored value from a linked Record path with `fetch.from`:

```yaml
fields:
  - name: customer
    label: Customer
    type: link
    options:
      entity: customer

  - name: customer-territory
    label: Customer Territory
    type: link
    fetch:
      from: customer.territory
    options:
      entity: territory
```

Every path segment before the last segment must be a `link` field. The final field can be scalar or `link`, but its type must match the destination field. If the final field is a `link`, both link fields must target the same Entity. dygo resolves fetched fields when Records are created or updated and stores the copied value on the destination Record.

Every metadata-backed Record has system fields:

```txt
id
name
created-at
updated-at
```

`id` is the internal numeric primary key. `name` is the stable system/business identifier. Entity `name` metadata controls how `name` is created.

Normal routeable Entities must define `name`. Use random naming when dygo should generate opaque Record names:

```yaml
name:
  strategy: random
```

The effective default random length is `16`, generated with Go `crypto/rand` and a Base58-style alphabet. Builders may override the length:

```yaml
name:
  strategy: random
  length: 24
```

Manual naming allows create requests and fixtures to provide the system `name` directly:

```yaml
name:
  strategy: manual
  label: Name
```

`name` is a system field and cannot appear under `fields`.

Format naming renders required stored fields into a deterministic name. Link field tokens render the linked Record's system `name`:

```yaml
name:
  strategy: format
  format: "{email}"
```

Series naming uses a pattern with date tokens and exactly one hash counter token:

```yaml
name:
  strategy: series
  pattern: "SINV-{YYYY}-{MM}-{#####}"
```

Supported v1 series tokens are `{YY}`, `{YYYY}`, `{MM}`, and one counter token such as `{#####}`. The number of hashes controls zero-padding. Series counters are stored in Core `naming-series` Records and incremented transactionally.

Format naming may include multiple field tokens:

```yaml
name:
  strategy: format
  format: "{app}.{key}"
```

Updating a field used for `name.strategy: format` does not rename an existing Record. Explicit Record rename is coming soon.

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

Index and constraint names are optional. If omitted, dygo derives deterministic names from the Entity key, type, and fields. Provided names must use kebab-case and are converted to snake_case for PostgreSQL.

Supported field check operators are `eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, and `not-in`. Field checks must use structured metadata, not raw SQL.

Check fields must be DB-backed scalar fields. `password`, `secret`, `collection`, `json`, `attachment`, and `link` checks are not supported in v1.

During `dygo db migrate`, normal Entity name metadata is upserted into the Core `entity` table. Collection row Entities omit naming metadata because their row names are framework-owned. Field metadata is upserted into the Core `field` table with field-name, label, type, required, unique, index, default, check, fetch, position, and options. Top-level Entity `indexes` and `constraints` are upserted into the Core `index` and `constraint` tables.

Type-specific settings live under `options`.

`select` fields require non-empty `options.values`.

`link` and `collection` fields require `options.entity`. For `link`, `options.app` is optional. When omitted, dygo resolves the target key in the current app first; otherwise the target Entity key must be globally unambiguous. Set `options.app` for cross-app links or ambiguous target keys:

```yaml
options:
  app: support
  entity: contact
```

`link` fields create storage columns and database foreign key constraints by default. PostgreSQL's default delete behavior applies, so deleting a target Record fails while other Records still reference it. Set `index: true` when the link is commonly filtered or joined. Set `options.foreign-key: false` for framework-level links that must not create database constraints, such as audit/history references that should survive target Record deletion:

```yaml
options:
  entity: user
  foreign-key: false
```

Link options may select a readable field for Studio labels and limit the
choices shown by the link picker. Use `from` for a value from the current form:

```yaml
options:
  entity: employee
  display-field: full-name
  filters:
    - field: department
      from: department
    - field: status
      operator: eq
      value: Active
```

The runtime validates `display-field` and filter fields against the target
Entity. Studio resolves the target route and uses the permission-aware Record
list API for search results. An unresolved dependent value omits that filter
until the parent form has a value.

`collection` fields use the same `{app, entity}` target shape as `link` fields. They must target a collection Entity; normal and Single Entities are rejected.

## Built-In Field Types

```txt
text
email
phone
password
secret
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

`secret` fields store decryptable ciphertext, not password hashes. Use them for
integration credentials. Storage uses `<field>_encrypted`; values are omitted
from ordinary reads. Required values are enforced by Record validation. See
[Secret fields](records.md#secret-fields) and [Record encryption keys](secrets.md#record-encryption-keys).

## App Discovery

Entity files belong to an app's manifest-defined `entities` directory. By default, that directory is:

```txt
entities
```

dygo loads normal Entity bundles from `entities/<entity>/<entity>.entity.yml` and collection Entity metadata from `entities/_collections/<collection>.yml` or `entities/_collections/<collection>/<collection>.entity.yml`. Missing `entities` directories are allowed for apps that do not define Entities yet.

Entity identities are unique per app across normal and collection Entities. An app cannot define both a normal `invoice` Entity and an `invoice` collection Entity. Two different apps may use the same Entity key when their route slugs are unique.

Moving a file without changing its basename does not move data because Entity identity is unchanged. Renaming a file changes Entity identity and requires explicit patch or migration handling.

Validate discovered Entity metadata from the current project:

```sh
dygo entity list
dygo entity validate
```

`entity list` prints a tree grouped by app name.

`entity validate` checks Entity syntax, path-derived names, field types, duplicate app-owned Entity identities, duplicate route slugs, `link` or `collection` targets, and Entity-bundle hook conventions.

Both commands discover the dygo project root before loading apps, so they can be run from nested directories inside a project.

`link` and `collection` targets use `{app, entity}` identity when `options.app` is set. Without `options.app`, dygo resolves same-app targets first, then a single globally unambiguous target. If no Entity matches or multiple external apps match, validation fails. Collection targets must resolve to collection Entities.

Validation errors include the app name, Entity key, field name when relevant, file path, and a best-effort YAML line number.

## Form tabs

Use either top-level `fields` or `tabs`. Each tab needs a label in `tab`, a unique kebab-case `name`, and a non-empty `fields` list. Set `icon` to add an optional Lucide icon. An explicit tab remains visible even when it is the only tab.

```yaml
label: Contact
name:
  strategy: random
tabs:
  - tab: Details
    name: details
    fields:
      - name: full-name
        label: Full Name
        type: text
      - type: column
      - name: email
        label: Email
        type: email
      - type: section
        label: Notes
        description: Additional contact information
      - name: notes
        label: Notes
        type: text
```

`type: column` starts the next column. `type: section` starts a new section and requires a label. Its description is optional. These markers control the form layout only. Do not add required, unique, index, default, check, fetch, or options settings to a marker.

Storage field names must be unique across all tabs. Markers do not create database columns or Record fields. Indexes, constraints, permissions, and Record APIs use storage fields only. Metadata sync saves the ordered layout in `entity.form` for Studio.

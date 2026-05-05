# Entity Metadata

Entities define business object structure in dygo.

The Entity catalog loads Entity files from discovered apps. Entity metadata is still the contract layer: migrations create platform tables, but record storage, permissions, Studio views, and runtime behavior are handled by later framework layers.

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

  - name: status
    label: Status
    type: select
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
```

## Rules

Entity `name`, `label`, `plural-name`, `plural-label`, and at least one field are required.

Entity names, plural names, field names, and field type names use kebab-case.

`plural-name` is explicit. dygo does not auto-pluralize Entity names in runtime code because English pluralization is too fragile for schema decisions. Future generators may suggest a default, but the YAML must store the chosen plural name.

When dygo needs a SQL table name from Entity metadata, it converts `plural-name` from kebab-case to snake_case. For example, `user-roles` maps to `user_roles`.

Field `name`, `label`, and `type` are required.

Field names must be unique inside an Entity.

Type-specific settings live under `options`.

`select` fields require non-empty `options.values`.

`link` and `child-table` fields require `options.entity`.

## Built-In Field Types

```txt
text
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

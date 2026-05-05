# Entity Metadata

Entities define business object structure in dygo.

The first Entity catalog is metadata-only. It loads Entity files from discovered apps and does not create database tables, records, permissions, Studio views, or migrations.

## Example

```yaml
name: lead
label: Lead
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

Entity `name`, `label`, and at least one field are required.

Entity names, field names, and field type names use kebab-case.

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

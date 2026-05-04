# Entity Metadata

Entities define business object structure in dygo.

The first Entity parser is metadata-only. It validates one Entity file at a time and does not create database tables, records, permissions, Studio views, or migrations.

## Example

```yaml
name: lead
label: Lead
module: crm
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

Entity `name`, `label`, `module`, and at least one field are required.

Entity names, module names, field names, and field type names use kebab-case.

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

## Follow-Up

Issue `#19` will load Entity definitions from discovered apps. This first parser only validates individual Entity files.

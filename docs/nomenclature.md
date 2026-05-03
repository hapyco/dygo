# dygo Nomenclature

dygo should use one vocabulary across code, docs, CLI output, generated metadata, and the Console.

The terms below are the source of truth unless a future design note explicitly changes them.

## Usage Rules

Use **Console**, not Desk.

Use **Space**, not Workspace.

Use **Entity**, not DocType.

Use **Record**, not Document, when referring to saved business data.

Technical implementation details may still use words such as document when they describe a file format, YAML payload, or parser concept rather than dygo business data.

## Core Terms

| Concept | dygo term |
|---|---|
| Main operational UI | Console |
| UI page/group inside Console | Space |
| Business object definition | Entity |
| Saved instance of an Entity | Record |
| Repeating structured data inside a Record | Child Table |
| One item inside a Child Table | Row |
| Group of related features/entities | Module |
| Installable package | App |
| Field inside an Entity | Field |
| Way to display Records | View |
| Structure inside a form view | Layout |
| Search/filter/sort state | Saved View |
| User-facing access rule | Permission |
| Internal permission rule logic | Policy |
| Extension point | Hook |
| Seed/reference data | Fixture |
| One-time data/schema operation | Patch |
| Background execution | Job |
| Recurring job definition | Schedule |
| Business state process | Workflow |
| App/module install history | Ledger |
| Human-friendly timeline | Activity Log |
| Compliance/security-grade history | Audit Log |
| File attached to a Record | Attachment |
| Generated print/PDF design | Print Format |
| Report definition/output | Report |
| Dashboard/chart area | Dashboard |
| Site/tenant boundary | Site |

## Example Language

The Console contains Spaces.

A Space organizes work around a business function.

An App contains Modules.

A Module contains Entities, Reports, and Views.

An Entity defines Fields, Permissions, Views, Hooks, and Child Tables.

A Record is saved data created from an Entity.

A Child Table contains Rows inside a Record.

## Frappe To dygo

dygo is not trying to clone Frappe, but some concepts map cleanly enough to make migration and comparison easier.

| Frappe term | dygo term |
|---|---|
| DocType | Entity |
| Document | Record |
| Desk | Console |
| Workspace | Space |
| Child Table | Child Table |
| Module | Module |
| App | App |
| Field | Field |
| Report | Report |
| Permission | Permission |
| Hook | Hook |
| Fixture | Fixture |
| Patch | Patch |

# dygo Nomenclature

dygo should use one vocabulary across code, docs, CLI output, generated metadata, and the Studio.

The terms below are the source of truth unless a future design note explicitly changes them.

## Usage Rules

Use **Studio** for the main operational and builder UI.

Use **Space** for a page or group inside Studio.

Use **Entity** for a business object definition.

Use **Record** for saved business data created from an Entity.

Use **name** for the stable system/business identifier generated for a Record. Use **id** for the internal numeric primary key.

Technical implementation details may still use words such as document when they describe a file format, YAML payload, or parser concept rather than dygo business data.

## Core Terms

| Concept | dygo term |
|---|---|
| Main operational/builder UI | Studio |
| UI page/group inside Studio | Space |
| Business object definition | Entity |
| Saved instance of an Entity | Record |
| Stable Record identifier | name |
| Internal numeric Record identity | id |
| Repeating structured data inside a Record | Child Table |
| One item inside a Child Table | Row |
| Installable package | App |
| Field inside an Entity | Field |
| Search/filter/sort state | Saved View |
| User-facing access rule | Permission |
| Internal permission rule logic | Policy |
| Extension point | Hook |
| Seed/reference data | Fixture |
| One-time data/schema operation | Patch |
| Background execution | Job |
| Recurring job definition | Schedule |
| Business state process | Workflow |
| App install/change history | Ledger |
| Human-friendly timeline and Record history | Activity |
| Compliance/security-grade history | Audit Log |
| File attached to a Record | Attachment |
| Generated print/PDF design | Print Format |
| Report definition/output | Report |
| Dashboard/chart area | Dashboard |

## Example Language

The Studio contains Spaces.

A Space organizes work around a business function.

An Entity defines Fields, Permissions, Hooks, and Child Tables.

A Record is saved data created from an Entity.

A Child Table contains Rows inside a Record.

The Studio globally renders Entities and Records.

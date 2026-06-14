# Workflow

Status: proposed.

This document records the intended direction for dygo Workflow. The current runtime does not implement this model yet.

## Direction

Workflow should own business lifecycle actions, status transitions, approvals, and document-style operations.

Roles and permissions decide who can invoke an action. Workflow decides whether the action is valid for the Record's current state and what state change or successor Record it creates.

Do not hardcode ERP verbs like `submit`, `cancel`, or `amend` as global CRUD actions. They are workflow actions that an app can opt into.

## Why This Is Workflow

Business software often has actions that are more important than ordinary updates:

- Frappe uses `submit`, `cancel`, and `amend` for auditable transaction documents.
- Odoo commonly uses verbs like `confirm`, `post`, `cancel`, and `reset_to_draft`.
- SAP-style systems commonly use `post`, `reverse`, `release`, `approve`, and `cancel`.
- Dynamics 365 Finance commonly uses workflow `submit`, `approve`, and `recall`, plus accounting `post` and `reverse`.
- CMS-style apps commonly use `publish` and `unpublish`.

These are not default CRUD. They are named business actions with rules.

## CRUD vs Workflow Actions

Base permission actions should stay small:

```txt
read
create
update
delete
export
print
```

Workflow actions are app-defined:

```txt
submit
approve
reject
post
cancel
reverse
amend
publish
unpublish
close
reopen
recall
```

The framework should validate action names against either the built-in permission actions or the Entity's workflow actions.

## Example Shape

Exact file layout is an open decision. A likely app shape is:

```txt
apps/sales/
  workflows/
    invoice.yml
```

Example:

```yaml
entity: invoice

state:
  field: status
  initial: draft

states:
  - draft
  - submitted
  - cancelled

actions:
  submit:
    from: draft
    to: submitted

  cancel:
    from: submitted
    to: cancelled

  amend:
    from: cancelled
    creates: draft
    link:
      field: amended_from
```

An accounting-style Entity could use different verbs:

```yaml
entity: journal_entry

state:
  field: status
  initial: draft

states:
  - draft
  - approved
  - posted
  - reversed

actions:
  approve:
    from: draft
    to: approved

  post:
    from: approved
    to: posted

  reverse:
    from: posted
    to: reversed
```

## Permissions

Permission files grant workflow actions the same way they grant built-in actions:

```yaml
entity: invoice
grants:
  - role: manager
    actions: [read, create, update, submit, cancel, amend]

  - role: user
    actions: [read, create, update, submit]
```

If an action is not a built-in permission action and is not declared by the Entity workflow, validation should fail.

## Runtime Rules

Workflow execution should eventually:

- check the user's permission for the action
- check the Record is in one of the allowed source states
- update the state or create the successor Record
- run inside a database transaction
- write an audit/log entry for the action
- run before/after hooks around the action

No workflow means no lifecycle actions. The Entity only gets ordinary Record operations.

## Hooks

Workflow should provide hook points for app behavior:

```txt
before_<action>
after_<action>
```

Examples:

```txt
before_submit
after_submit
before_post
after_reverse
```

The workflow layer owns validation and state movement. Hooks own app-specific side effects.

## Open Decisions

- exact file layout: `apps/<app>/workflows/<entity>.yml` vs Entity co-location
- whether workflow state uses a reserved field or an app-selected field
- whether immutable states block `update` by default
- how workflow actions appear in the Record API
- how Studio designs and exports workflow metadata
- whether common workflows should be templates or just examples

# Core App

Core is the required system App for dygo.

It provides the foundation dygo needs before business apps can run: system metadata, users, roles, permissions, sessions, installed App state, fixtures, and patches.

The first Entity scaffold is metadata-only. It defines these Core system contracts:

```txt
app
entity
field
user
role
user-role
permission
session
```

These contracts do not create database tables, authentication behavior, permission resolution, records, migrations, or Studio screens yet.

Lifecycle and history records such as patch runs, migration runs, and ledger-style change history are deferred until migrations and app lifecycle behavior are designed.

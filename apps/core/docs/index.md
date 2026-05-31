# Core App

Core is the required system App for dygo.

It provides the foundation dygo needs before business apps can run: system metadata, users, roles, permissions, sessions, activity history, persisted Logs, installed App state, fixtures, and patches.

The first Entity scaffold is metadata-only. It defines these Core system contracts:

```txt
app
activity
log
entity
field
index
naming-series
patch-run
user
role
user-role
permission
session
```

These contracts create Core database tables through dygo's metadata-driven schema sync. The `activity` contract is the append-only storage shape for Record history and product timeline events. The `log` contract stores framework and app diagnostics.

`patch-run` is the first app lifecycle ledger. It records successfully applied app patches so later patch planning and application can detect pending patches and checksum drift.

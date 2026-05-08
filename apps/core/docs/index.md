# Core App

Core is the required system App for dygo.

It provides the foundation dygo needs before business apps can run: system metadata, users, roles, permissions, sessions, activity history, installed App state, fixtures, and patches.

The first Entity scaffold is metadata-only. It defines these Core system contracts:

```txt
app
activity
entity
field
user
role
user-role
permission
session
```

These contracts create Core database tables through dygo's metadata-driven schema sync. The `activity` contract is the storage shape for future Record history and timeline events; runtime activity writing is still a follow-up layer.

Lifecycle records such as patch runs and ledger-style app change history are deferred until patches and app lifecycle behavior are designed.

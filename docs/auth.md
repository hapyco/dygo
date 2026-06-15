# Auth

dygo auth v1 is Studio-oriented and session-based.

The first auth layer uses Core `user` and `session` records. Password fields are stored as bcrypt hashes, and browser sessions are carried by an HttpOnly cookie named `dygo_session`.

## Bootstrap

Create the first Administrator account after preparing the database:

```sh
dygo db prepare
dygo setup
```

For automation:

```sh
printf 'change-me\n' | dygo setup --email admin@example.com --full-name "Admin User" --password-stdin
```

The Administrator account is special. It is not a role and does not depend on role assignment. Record permission enforcement treats `administrator=true` as privileged inside the permission engine before regular role permissions are checked.

## API

```txt
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
GET  /api/v1/boot
```

Login accepts:

```json
{"data":{"email":"admin@example.com","password":"secret"}}
```

Login returns the current user and sets the session cookie:

```json
{"data":{"id":1,"email":"admin@example.com","full-name":"Admin User","enabled":true,"administrator":true}}
```

`/auth/me` returns the same user shape for the current session. `/boot` returns the authenticated user's tight Studio startup payload, including resolved defaults such as `home`. `/auth/logout` revokes the current session and clears the cookie.

## Boundaries

`/health` and `POST /api/v1/auth/login` are public. Boot, metadata, and Record API routes require a valid session. Metadata routes filter or deny data through the permission engine; Record routes require the relevant Entity action.

This layer does not add API keys, OAuth, SSO, password reset, or Studio login UI. Record APIs are guarded separately through the permission engine.

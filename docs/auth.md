# Auth

dygo auth v1 is Studio-oriented and session-based.

The first auth layer uses Core `user` and `session` records. Password fields are stored as bcrypt hashes, and browser sessions are carried by an HttpOnly cookie named `dygo_session`.

## Bootstrap

Create the first Administrator account after running metadata sync:

```sh
go run ./cmd/dygo migrate
go run ./cmd/dygo setup admin
```

For automation:

```sh
printf 'change-me\n' | go run ./cmd/dygo setup admin --email admin@example.com --full-name "Admin User" --password-stdin
```

The Administrator account is special. It is not a role and does not depend on role assignment. Record permission enforcement treats `administrator=true` as privileged inside the permission engine before regular role permissions are checked.

## API

```txt
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/auth/me
```

Login accepts:

```json
{"data":{"email":"admin@example.com","password":"secret"}}
```

Login returns the current user and sets the session cookie:

```json
{"data":{"id":1,"email":"admin@example.com","full-name":"Admin User","enabled":true,"administrator":true}}
```

`/auth/me` returns the same user shape for the current session. `/auth/logout` revokes the current session and clears the cookie.

## Boundaries

`/health` and `POST /api/v1/auth/login` are public. Metadata and Record API routes require a valid session.

This layer does not add API keys, OAuth, SSO, password reset, or Studio login UI. Record APIs are guarded separately through the permission engine.

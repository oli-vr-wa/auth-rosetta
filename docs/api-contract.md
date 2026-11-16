# Auth Rosetta API Contract

Every backend (.NET, NestJS, Python, Go, Java) **must** implement this contract exactly, so the single React frontend can be pointed at any of them by changing `VITE_API_URL`.

If a backend behaves differently from this document, the backend is wrong, not the contract.

## Conventions

| Item | Rule |
|---|---|
| Base path | `/api/auth` |
| Format | JSON request and response bodies, `Content-Type: application/json` |
| JSON casing | `camelCase` |
| Error body | `{ "error": "message" }` for **every** error (see [Error handling](#error-handling)) |
| Authentication | JWT in an `HttpOnly` cookie named `access_token` (see [Session cookie](#session-cookie)) |
| Email handling | Trimmed and lowercased before any lookup or storage |

The `Authorization` header is **not** part of this contract. Clients never see or handle the token: the server sets it on login, the browser sends it automatically, and the server clears it on logout.

### CORS

Because the session lives in a cookie, CORS must allow credentials.

| Setting | Value |
|---|---|
| Allowed origin | `http://localhost:5173` by default, overridable with the optional `FRONTEND_ORIGIN` environment variable |
| Allow credentials | `true` (`Access-Control-Allow-Credentials: true`) |
| Allowed methods | `GET`, `POST`, `OPTIONS` |
| Allowed headers | `Content-Type` |

Rules:

- The allowed origin is **explicit**. A wildcard `*` is never allowed, because browsers reject it together with credentials.
- If `FRONTEND_ORIGIN` is set, it must be a plain origin such as `http://localhost:5173`: no wildcard, no path, no trailing slash. An invalid value must stop the server at startup.
- An empty or missing value must fall back to the default and must never be interpreted as "allow any origin".
- A request from a different origin must never receive an `Access-Control-Allow-Origin` header that matches it. Answering with the single allowed origin, or omitting the header, are both acceptable.

The frontend must send every request with `credentials: "include"`.

### Ports

| Backend | Port |
|---|---|
| .NET 10 | 5001 |
| NestJS | 5002 |
| Python | 5003 |
| Go | 5004 |
| Java | 5005 |

### Databases

Each backend uses its own database on the shared Postgres container: `auth_net`, `auth_nest`, `auth_python`, `auth_go`, `auth_java`.

## Client requirements

Anything calling this API from a browser must follow these rules.

| Rule | Why |
|---|---|
| Send every request with `credentials: "include"` | Without it the browser neither stores nor sends the session cookie |
| Open the frontend at `http://localhost:5173` and call the API through `localhost`, never `127.0.0.1` | `SameSite=Strict` and the CORS origin check both treat `127.0.0.1` as a different site |
| Never store, read or send the token yourself | The cookie is `HttpOnly`, so JavaScript cannot see it |
| Send `code` as a string | A number would drop leading zeros |
| Treat a `401` from `/me` as "not logged in" | There is no client-side token to inspect |
| Show the `error` field to the user | Every error body has the same shape |
| Log out or clear cookies for `localhost` when switching backends | Every backend uses the cookie name `access_token`, and cookies are scoped by host, not by port |

## Flow

1. **Register**: account is created with 2FA disabled, and a TOTP secret is returned.
2. **Confirm 2FA**: the user scans the QR code, submits a code, and 2FA becomes enabled.
3. **Login**: email, password and a current TOTP code are all required. The server sets the session cookie.
4. **Success page**: the frontend calls `/me`, and the browser attaches the cookie.
5. **Logout**: the token is revoked server-side and the cookie is cleared.

---

## Session cookie

Set by `POST /login`, cleared by `POST /logout`.

| Attribute | Value |
|---|---|
| Name | `access_token` |
| Value | The signed JWT |
| `HttpOnly` | `true` (JavaScript must never be able to read it) |
| `Secure` | `true` whenever the request arrived over HTTPS. May be `false` on plain-HTTP local development |
| `SameSite` | `Strict` |
| `Path` | `/` |
| Lifetime | 60 minutes (`Expires` / `Max-Age` matches the JWT `exp`) |

To clear the cookie, send `Set-Cookie` with the same name, path and attributes and an expiry date in the past.

---

## Error handling

Every error response, including ones produced by the framework, has the body:

```json
{ "error": "message" }
```

| Situation | Status | `error` |
|---|---|---|
| Validation failure | `400` | A human-readable message. Several failures are joined with `; ` |
| Malformed JSON, or a body that is too large | `400` | `Invalid request` |
| Missing, invalid, expired or revoked session | `401` | `Unauthorized` (any wording is acceptable, but it must not reveal the reason) |
| Failed login | `401` | `Invalid credentials` |
| Unknown route | `404` | Any message |
| Duplicate email | `409` | `Email already registered` |
| Unexpected server failure | `500` | `Internal server error` |

The messages written in code format above (`Invalid request`, `Invalid credentials`, `Email already registered`, `Internal server error`) are exact. Validation messages are free text.

**Generic to the client, specific in the logs.** The response never says more than the table allows. The real reason (which login check failed, why a token was rejected, the exception behind a `500`) is written to the **server log** instead.

Logging rules:

- Never log secrets: passwords, tokens, TOTP codes, TOTP secrets, cookie values or request bodies.
- Avoid logging email addresses at debug level.
- Failure details such as "password ok: false, code ok: true" may be logged at debug level.

---

## Endpoints

### `POST /api/auth/register`

Creates a user and generates a TOTP secret. 2FA is **not** enabled until confirmed.

**Request**

```json
{ "email": "user@example.com", "password": "at-least-8-chars" }
```

**Responses**

| Status | Body | When |
|---|---|---|
| `201 Created` | `{ "otpauthUri": "otpauth://totp/...", "secret": "BASE32SECRET" }` | User created |
| `400 Bad Request` | `{ "error": "..." }` | Fails [validation](#validation-rules) |
| `409 Conflict` | `{ "error": "Email already registered" }` | Email exists, including two simultaneous registrations |

---

### `POST /api/auth/2fa/confirm`

Verifies a TOTP code against the stored secret and enables 2FA.

**Request**

```json
{ "email": "user@example.com", "code": "123456" }
```

**Responses**

| Status | Body | When |
|---|---|---|
| `200 OK` | `{ "message": "Two-factor authentication enabled" }` | Code valid, 2FA now enabled |
| `400 Bad Request` | `{ "error": "Invalid code" }` | Code wrong, code not exactly 6 digits, or the user does not exist (all indistinguishable) |

---

### `POST /api/auth/login`

Requires password **and** a valid TOTP code, and the account must have 2FA enabled. On success the token is delivered **only** in the session cookie, never in the response body.

**Request**

```json
{ "email": "user@example.com", "password": "at-least-8-chars", "code": "123456" }
```

**Responses**

| Status | Body | Headers | When |
|---|---|---|---|
| `200 OK` | `{ "message": "Login successful" }` | `Set-Cookie: access_token=...` | All checks pass |
| `400 Bad Request` | `{ "error": "..." }` | none | The body is malformed: invalid JSON, or a field is missing or not a string |
| `401 Unauthorized` | `{ "error": "Invalid credentials" }` | none | Any failure |

Rules:

- A login failure must **always** return the same generic `401`. Never reveal whether the email, password, code, or 2FA status was the cause.
- Login applies **no** password or code policy. A short wrong password is an ordinary `401`, never a `400`, so the policy is not leaked and accounts created under an older policy still work.
- An unknown email must take about as long as a wrong password. Run a password-hash comparison against a dummy hash when the user does not exist, so response time does not reveal which accounts exist.

---

### `GET /api/auth/me`

Returns the identity of the authenticated user.

**Requires:** the `access_token` cookie.

**Responses**

| Status | Body | When |
|---|---|---|
| `200 OK` | `{ "email": "user@example.com" }` | Valid, non-revoked token |
| `401 Unauthorized` | `{ "error": "Unauthorized" }` | Cookie missing, or token invalid, expired or revoked |

An anonymous request must never return `200`.

---

### `POST /api/auth/logout`

Revokes the token in the cookie so it can no longer be used, even before it expires, and clears the cookie.

**Requires:** the `access_token` cookie.

**Responses**

| Status | Body | Headers | When |
|---|---|---|---|
| `204 No Content` | none | `Set-Cookie` that clears `access_token` | Token revoked |
| `401 Unauthorized` | `{ "error": "Unauthorized" }` | none | Cookie missing, or token invalid, expired or already revoked |

---

## Validation rules

Validation is enforced by the backend. The frontend's checks are only a convenience.

| Field | Where | Rule |
|---|---|---|
| `email` | register | Required, trimmed, non-empty, at most 256 characters |
| `password` | register | At least 8 characters and at most **72 bytes** (UTF-8). Not trimmed, because spaces are legitimate |
| `code` | confirm | Exactly 6 digits, sent as a string |
| `email`, `password`, `code` | login | Must be present and be strings. No other rules (see the login rules above) |

The 72-byte password limit exists because bcrypt only reads the first 72 bytes. Without a limit, two long passwords sharing a 72-byte prefix would be treated as identical. The limit is in bytes, not characters, because a short password in a non-Latin script can exceed 72 bytes.

## Cryptography and token specifications

### Password hashing

| Setting | Value |
|---|---|
| Algorithm | bcrypt |
| Cost / work factor | 12 |

### TOTP (RFC 6238)

| Setting | Value |
|---|---|
| Algorithm | SHA-1 |
| Digits | 6 |
| Period | 30 seconds |
| Secret | 20 random bytes, Base32 encoded |
| Verification window | Accept the previous, current and next step (±1) |
| Issuer | `AuthRosetta` |

`otpauthUri` format:

```text
otpauth://totp/AuthRosetta:{email}?secret={secret}&issuer=AuthRosetta
```

The issuer and email in the label must be URL-encoded. The URI may also carry the default parameters `algorithm`, `digits` and `period`, which authenticator apps accept.

A malformed code (wrong length, not digits) is simply an invalid code. It must never cause a `500`.

### JWT

The JWT is the value of the session cookie.

| Setting | Value |
|---|---|
| Algorithm | HS256 (verification must accept only this algorithm) |
| Signing key | `JWT_KEY` environment variable (32+ bytes, never committed) |
| Issuer | `AuthRosetta` |
| Audience | `AuthRosetta` |
| Lifetime | 60 minutes |
| Clock skew | 30 seconds |

**Required claims**

| Claim | Value |
|---|---|
| `sub` | User ID |
| `email` | User email |
| `jti` | Unique token ID (random UUID) |
| `exp` | Expiry (Unix seconds) |
| `iss` / `aud` | As above |

### Token revocation

- `POST /logout` stores the token's `jti` and `exp` in a denylist. Revoking the same token twice is harmless.
- Every authenticated request must reject a token whose `jti` is in the denylist.
- Denylist entries whose `exp` has passed may be deleted.

## Data model

Each backend uses its own naming conventions, but must store these fields.

**users**

| Field | Type | Notes |
|---|---|---|
| `id` | UUID | Primary key |
| `email` | string (max 256) | Unique, stored lowercase |
| `passwordHash` | string | bcrypt hash |
| `totpSecret` | string | Base32 secret |
| `twoFactorEnabled` | boolean | Default `false` |
| `createdAt` | timestamp (UTC) | |

**revoked_tokens**

| Field | Type | Notes |
|---|---|---|
| `jti` | string | Primary key |
| `expiresAt` | timestamp (UTC) | |

Schema changes are made with migrations, never by letting the ORM synchronize the schema automatically.

## Environment variables

| Variable | Required | Purpose |
|---|---|---|
| `DB_USER` | Yes | Postgres username |
| `DB_PASSWORD` | Yes | Postgres password |
| `JWT_KEY` | Yes | JWT signing key, at least 32 characters |
| `FRONTEND_ORIGIN` | No | Allowed CORS origin, default `http://localhost:5173` |

All of them come from the root `.env` file. A backend must refuse to start if a required variable is missing, or if `JWT_KEY` is too short. Individual stacks may read extra optional variables (for example a database host or a port), which are not part of the contract.

## Security notes

### Why a cookie and not a header

The token is in an `HttpOnly` cookie so that JavaScript, including injected scripts and compromised dependencies, can never read it. This prevents token theft through XSS.

### CSRF

Cookies are sent automatically, so the API must defend against cross-site requests:

- `SameSite=Strict` stops the browser attaching the cookie to requests started from other sites.
- All state-changing endpoints require a JSON body, which forces a CORS preflight that only the allowed origin passes.
- `localhost:5173` and `localhost:5001` are the same *site* (ports are ignored), so this works in development. In production the frontend and API must share a registrable domain (for example `app.example.com` and `api.example.com`).

### CORS is not access control

CORS only stops other **websites** from using a visitor's browser, and their cookies, against the API. `curl` and other non-browser clients ignore it completely, so it is never a substitute for authentication.

## Known limitations (documented on purpose)

- An XSS bug can no longer steal the token, but injected script can still make authenticated requests while the victim's page is open, because the browser attaches the cookie.
- The TOTP secret is returned only once, by `/register`. If the user reloads or closes the page before confirming, the secret cannot be retrieved. The account then exists with 2FA unconfirmed, the email stays taken (`409`), and no endpoint re-issues a secret. Unconfirmed accounts never expire.
- A TOTP code can be reused within its validity window (about 90 seconds with the ±1 step window), so there is no replay protection.
- There is no CSRF token. The design relies on `SameSite=Strict` plus the CORS rules above.
- TOTP secrets are stored in plaintext. Production systems should encrypt them at rest.
- There is no rate limiting or account lockout, so a 6-digit code can be brute-forced.
- Logout revokes only the presented token. "Log out everywhere" is not supported.
- Every authenticated request performs a denylist lookup, which trades away statelessness.
- All backends share one `JWT_KEY`, issuer and audience, so a token issued by one backend passes **signature** validation in another. Revocation is stored per database, so a token revoked in one backend is still accepted by the others. This only matters because the project runs several stacks side by side. Using a per-stack audience (for example `AuthRosetta-nest`) would prevent it.
- A production setup would normally use a short-lived access token plus a refresh token.

## Manual test checklist

Use a client **without a cookie jar** (such as `curl`) for the anonymous and replay checks. A leftover cookie in a REST client's jar silently turns those checks into authenticated requests. In Windows PowerShell, use `curl.exe`, because plain `curl` is an alias for something else.

**Anonymous and validation**

1. Call `/me` with no cookie and expect `401` with an `error` field.
2. Call `/me` with a garbage cookie and expect `401`.
3. Register with a 7-character password and expect `400`.
4. Register with a password longer than 72 bytes and expect `400`.
5. Register with an empty or whitespace-only email and expect `400`.
6. Send malformed JSON (`{bad`) to `/login` and expect `400` with `{ "error": "Invalid request" }`.
7. Request an unknown route and expect `404` with an `error` field.

**The flow**

8. Register a user and receive `otpauthUri` and `secret`. Register the same email again and expect `409`.
9. Add the secret to an authenticator app.
10. Login **before** confirming 2FA, and expect `401` with no `Set-Cookie`.
11. Confirm with a wrong code, and with an unknown email, and expect `400` both times. Confirm with a valid code and expect `200`.
12. Login with a wrong password, wrong code, and unknown email, and expect an identical `401` body each time.
13. Login with valid credentials, expect `200` and a `Set-Cookie` header containing `HttpOnly` and `SameSite=Strict`, and confirm the body contains no token.
14. Call `/me` and receive the email.
15. Save the cookie value, then call `/logout` and expect `204` plus a cookie-clearing `Set-Cookie`.
16. Call `/me` again and expect `401` (cookie gone).
17. Replay the **saved** cookie value on `/me` and on `/logout`, and expect `401` both times (token revoked server-side).

**CORS**

18. Send a preflight `OPTIONS` request with `Origin: http://localhost:5173` and expect `Access-Control-Allow-Origin: http://localhost:5173` and `Access-Control-Allow-Credentials: true`.
19. Send the same request with `Origin: http://evil.example` and confirm the response never contains `Access-Control-Allow-Origin: http://evil.example` or `*`.
# Auth Rosetta API Contract

Every backend (.NET, NestJS, Python, Go, Java) **must** implement this contract exactly, so the single React frontend can be pointed at any of them by changing `VITE_API_URL`.

If a backend behaves differently from this document, the backend is wrong, not the contract.

## Conventions

| Item | Rule |
|---|---|
| Base path | `/api/auth` |
| Format | JSON request and response bodies, `Content-Type: application/json` |
| JSON casing | `camelCase` |
| Error body | `{ "error": "message" }` |
| Auth | `Authorization: Bearer <token>` |
| CORS | Allow origin `http://localhost:5173`, any header, any method |
| Email handling | Trimmed and lowercased before any lookup or storage |

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

## Flow

1. **Register**: account is created with 2FA disabled, and a TOTP secret is returned.
2. **Confirm 2FA**: the user scans the QR code, submits a code, and 2FA becomes enabled.
3. **Login**: email, password and a current TOTP code are all required. A JWT is returned.
4. **Success page**: the frontend calls `/me` with the token.
5. **Logout**: the token is revoked server-side.

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
| `400 Bad Request` | `{ "error": "..." }` | Email empty, or password shorter than 8 characters |
| `409 Conflict` | `{ "error": "Email already registered" }` | Email exists |

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
| `400 Bad Request` | `{ "error": "Invalid code" }` | Code wrong, or the user does not exist (indistinguishable) |

---

### `POST /api/auth/login`

Requires password **and** a valid TOTP code, and the account must have 2FA enabled.

**Request**

```json
{ "email": "user@example.com", "password": "at-least-8-chars", "code": "123456" }
```

**Responses**

| Status | Body | When |
|---|---|---|
| `200 OK` | `{ "token": "<jwt>" }` | All checks pass |
| `401 Unauthorized` | `{ "error": "Invalid credentials" }` | Any failure |

A login failure must **always** return the same generic `401`. Never reveal whether the email, password, code, or 2FA status was the cause.

---

### `GET /api/auth/me`

Returns the identity of the authenticated user.

**Headers:** `Authorization: Bearer <token>`

**Responses**

| Status | Body | When |
|---|---|---|
| `200 OK` | `{ "email": "user@example.com" }` | Valid, non-revoked token |
| `401 Unauthorized` | none or `{ "error": "..." }` | Missing, invalid, expired or revoked token |

---

### `POST /api/auth/logout`

Revokes the presented token so it can no longer be used, even before it expires.

**Headers:** `Authorization: Bearer <token>`

**Responses**

| Status | Body | When |
|---|---|---|
| `204 No Content` | none | Token revoked |
| `401 Unauthorized` | none or `{ "error": "..." }` | Missing, invalid, expired or already revoked token |

---

## Validation rules

| Field | Rule |
|---|---|
| `email` | Required, non-empty after trimming |
| `password` | Required, minimum 8 characters |
| `code` | Required, 6-digit string (send as a string to preserve leading zeros) |

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

The issuer and email in the label must be URL-encoded.

### JWT

| Setting | Value |
|---|---|
| Algorithm | HS256 |
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

- `POST /logout` stores the token's `jti` and `exp` in a denylist.
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

## Environment variables

| Variable | Purpose |
|---|---|
| `DB_USER` | Postgres username |
| `DB_PASSWORD` | Postgres password |
| `JWT_KEY` | JWT signing key |

All three come from the root `.env` file.

## Known limitations (documented on purpose)

- TOTP secrets are stored in plaintext. Production systems should encrypt them at rest.
- There is no rate limiting or account lockout, so a 6-digit code can be brute-forced.
- Logout revokes only the presented token. "Log out everywhere" is not supported.
- Every authenticated request performs a denylist lookup, which trades away statelessness.

## Manual test checklist

1. Register a user and receive `otpauthUri` and `secret`.
2. Add the secret to an authenticator app.
3. Login **before** confirming 2FA, and expect `401`.
4. Confirm with a valid code and expect `200`.
5. Login with a wrong password, wrong code, and unknown email, and expect an identical `401` each time.
6. Login with valid credentials and receive a token.
7. Call `/me` and receive the email.
8. Call `/logout` and expect `204`.
9. Call `/me` again with the same token and expect `401`.
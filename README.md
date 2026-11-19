# Auth Rosetta

A multi-stack monorepo exploring how different backend ecosystems natively implement the exact same Two-Factor Authentication (TOTP) login flow.

This project serves as a "Rosetta Stone" for backend development. It uses a single React SPA that can be hot-swapped to point at different backend APIs. Each backend connects to its own isolated PostgreSQL database, allowing for a pure comparison of how each language and framework handles database migrations, password hashing, TOTP secret generation, JWT creation, cookie handling, validation and error handling.

## Architecture

* **Frontend:** React + Vite (TypeScript) + Tailwind CSS
* **Infrastructure:** Docker Compose (shared PostgreSQL container with logical DB isolation)
* **Backends:**

| Stack | Compose profile | Port | Database | Status |
|---|---|---|---|---|
| **.NET 10** (C# + Entity Framework Core) | `net` | 5001 | `auth_net` | Completed |
| **NestJS** (Node + TypeScript + TypeORM) | `nest` | 5002 | `auth_nest` | Completed |
| **Python** | `python` | 5003 | `auth_python` | Working |
| **Go** | `go` | 5004 | `auth_go` | Planned |
| **Java** (Spring Boot) | `java` | 5005 | `auth_java` | Planned |

Every backend implements the same [API contract](docs/api-contract.md), so the frontend works with any of them by changing a couple of environment variables.

## The flow

1. **Register** with an email and password. The API returns a TOTP secret and an `otpauth://` URI.
2. **Scan the QR code** with an authenticator app and enter a code to confirm. This enables 2FA.
3. **Log in** with email, password and a current 6-digit code.
4. The API sets the session in an **`HttpOnly` cookie**. The browser sends it automatically and JavaScript never sees the token.
5. The **success page** calls `/me` to prove the session works. **Logout** revokes the token server-side and clears the cookie.

## Prerequisites

* [Docker](https://www.docker.com/) with Docker Compose
* [.NET 10 SDK](https://dotnet.microsoft.com/download) and the EF Core CLI (`dotnet tool install --global dotnet-ef`), needed to run .NET migrations from your machine
* [Node.js](https://nodejs.org/) 22.12 or newer (20.19+ also works). Needed to run NestJS migrations from your machine, and to run the frontend or NestJS outside Docker

## Getting Started

### 1. Environment variables

Create a `.env` file in the root directory (copy `.env.example`):

```env
DB_USER=rosetta_admin
DB_PASSWORD=your_secure_password
JWT_KEY=paste-a-long-random-value-here

# Which backend docker compose starts. One of: net | nest | python | go | java
COMPOSE_PROFILES=net

# The API URL your BROWSER uses. The port must match the backend above:
# net = 5001, nest = 5002, python = 5003, go = 5004, java = 5005
VITE_API_URL=http://localhost:5001

# Optional: the origin the API allows through CORS (default shown)
# FRONTEND_ORIGIN=http://localhost:5173
```

`JWT_KEY` signs the JWTs and must be at least 32 characters. Generate one with either command:

```powershell
# PowerShell (Windows PowerShell 5.1 and 7+)
$b = New-Object byte[] 48; [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b); [Convert]::ToBase64String($b)
```

```bash
# macOS, Linux, Git Bash
openssl rand -base64 48
```

If the value ever gets mangled, wrap it in double quotes. The `.env` file is ignored by git, so never commit it.

The backends refuse to start if `DB_USER`, `DB_PASSWORD` or `JWT_KEY` is missing, if `JWT_KEY` is too short, or if `FRONTEND_ORIGIN` is not a plain origin (no wildcard, path or trailing slash).

### 2. Start everything

```bash
docker compose up -d --build
```

This starts:

* **PostgreSQL** (always), which creates an isolated database for each stack (`auth_net`, `auth_nest`, `auth_python`, `auth_go`, `auth_java`)
* **The frontend** (always), with the Vite dev server and hot reload
* **The backend** selected by `COMPOSE_PROFILES`

Then open **http://localhost:5173**.

Verify the databases exist:

```bash
docker exec -it rosetta_db psql -U rosetta_admin -c "\l"
```

The database init script only runs when the `postgres_data` volume is first created. To start over from scratch (this **deletes all data**):

```bash
docker compose down -v
docker compose up -d --build
```

### 3. Apply the database migrations

Migrations are run from your machine, which works because Postgres publishes port 5432. Containers do not apply them automatically. A backend starts without them, but requests that touch the database fail until they are applied.

**.NET**

```bash
dotnet ef database update --project backend-net/AuthRosetta.Infrastructure --startup-project backend-net/AuthRosetta.Api
```

Create a new migration after changing the model:

```bash
dotnet ef migrations add <Name> --project backend-net/AuthRosetta.Infrastructure --startup-project backend-net/AuthRosetta.Api
```

**NestJS** (TypeORM)

```bash
cd backend-nest
npm install              # first time only, installs the tooling on your machine
npm run migration:run
```

Create a new migration after changing an entity, then review the generated file before running it:

```bash
npm run migration:generate -- src/database/migrations/<Name>
```

Other useful scripts: `npm run migration:show` lists applied and pending migrations, and `npm run migration:revert` undoes the last one. The schema is never synchronized automatically, so every change goes through a migration.

### 4. Switching backends

The frontend runs unchanged against every backend. Two settings select which one, and they must be changed together.

1. Stop the current containers **while the old profile is still set**. Compose only manages services in the active profile, so changing the profile first would leave the old backend running:
   ```bash
   docker compose down
   ```
2. Edit the root `.env`: set `COMPOSE_PROFILES` to the new stack (for example `nest`) and `VITE_API_URL` to its port (for example `http://localhost:5002`).
3. Start again:
   ```bash
   docker compose up -d --build
   ```
4. Log out, or clear the cookies for `localhost`. Every backend uses the cookie name `access_token`, and cookies are shared across ports on the same host. Because all backends share one `JWT_KEY`, a session token from one backend is also **accepted** by another (revocation is stored per database, so a revoked token can still work elsewhere). Clearing cookies avoids confusing results.

To run a backend without changing `.env`, pass the profile explicitly, for example `docker compose --profile nest up -d --build`.

### 5. The frontend

The frontend runs in a container with your source mounted in, so edits in `frontend-react/src` appear in the browser immediately. File-change polling is enabled (`USE_POLLING`) because file events from the Windows filesystem do not reach Linux containers.

`VITE_API_URL` points at the API **as your browser sees it**, so it is always `http://localhost:<port>`, never a Docker service name.

Always use `localhost`, not `127.0.0.1`. The session cookie is `SameSite=Strict` and the API only allows the `http://localhost:5173` origin, so using `127.0.0.1` makes login look successful while the next request fails.

Useful commands:

```bash
docker compose logs -f frontend                        # Vite output and errors
docker compose exec frontend npm install <package>     # add a dependency
docker compose up -d frontend                          # apply a changed VITE_API_URL
```

Install packages **inside** the container, so they land in the Linux `node_modules`. If you install from your host instead, rebuild with `docker compose up -d --build --renew-anon-volumes frontend`.

**Running the frontend without Docker** (optional): stop the container first so port 5173 is free, then:

```bash
docker compose stop frontend
cd frontend-react
npm install
cp .env.example .env     # Windows PowerShell: Copy-Item .env.example .env
npm run dev
```

In this mode the URL comes from `frontend-react/.env` instead of the root `.env`.

### 6. Running a backend outside Docker (optional)

Useful for debugging and hot reload. Stop the container first, because it uses the same port.

**.NET** (port 5001)

```bash
docker compose stop backend-net
dotnet run --project backend-net/AuthRosetta.Api
```

**NestJS** (port 5002)

```bash
docker compose stop backend-nest
cd backend-nest
npm install
npm run start:dev
```

Outside Docker, both load the root `.env` automatically. `npm run start:dev` also prints the debug log lines that explain why a login or token was rejected. The container hides them, since it runs with `NODE_ENV=production`. To see them in the container, add `NODE_ENV: development` to the service's `environment` block in `docker-compose.yml`.

Setup notes for the other stacks will be added as each one is completed.

### 7. Testing the API without the frontend

[`requests.http`](requests.http) runs the whole flow (register, 2FA, login, logout, and token revocation) against any backend. Open it in VS Code with the [REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) extension, set `@baseUrl` to the stack under test, and send the requests top to bottom. The comments at the top explain the manual authenticator step.

HTTP editors keep a **cookie jar**, and a leftover session cookie is sent even when the request preview does not show it. That can turn an anonymous check into an authenticated one, so a `200` where you expect `401` usually means a stale cookie, not a backend bug. Restart the editor to clear its cookies, or use `curl` for the anonymous and revoked-token checks. In Windows PowerShell, type `curl.exe`, because plain `curl` is an alias for something else:

```powershell
curl.exe -i http://localhost:5002/api/auth/me -H "Cookie: access_token=not-a-real-token"
```

The full checklist is at the end of the [API contract](docs/api-contract.md#manual-test-checklist).

## Security design

* Passwords are hashed with **bcrypt** (cost 12), and limited to 72 bytes, because bcrypt ignores anything beyond that.
* TOTP follows RFC 6238: SHA-1, 6 digits, 30-second period, with a ±1 step verification window.
* Sessions are JWTs (HS256) delivered in an **`HttpOnly`, `SameSite=Strict`** cookie, so injected scripts cannot read the token.
* Logout stores the token's `jti` in a denylist, so a token is rejected even before it expires.
* Login failures always return the same generic `401`, and an unknown email takes about as long as a wrong password.
* Errors are generic to the client and specific in the server log. Logs never contain passwords, tokens, codes or cookie values.
* Input is validated by the backend, and every error response has the same `{ "error": "..." }` shape.

This is a learning project, and the [API contract](docs/api-contract.md#known-limitations-documented-on-purpose) lists its deliberate limitations (plaintext TOTP secrets, no rate limiting, no CSRF token beyond `SameSite`, tokens accepted across backends, and more).

## Troubleshooting

| Problem | Fix |
|---|---|
| DBeaver shows only the `postgres` database | Tick **Show all databases** in the connection's Main tab, or set the Database field to `auth_net` or `auth_nest`. |
| A stack's database does not exist | The init script did not run. Reset with `docker compose down -v` then `docker compose up -d --build`. |
| Login succeeds but `/me` returns 401 | You are probably on `127.0.0.1`. Use `http://localhost:5173`. |
| `/me` returns 200 when it should be 401 in `requests.http` | The editor's cookie jar sent a leftover session cookie. Restart the editor, or test with `curl.exe`. |
| CORS error in the browser console | Check the frontend runs on port 5173 and the API allows credentials. If you changed `FRONTEND_ORIGIN`, it must match the page's origin exactly. |
| A backend exits with `Missing required environment variables` | `DB_USER`, `DB_PASSWORD` or `JWT_KEY` is missing from the root `.env`, or the variable names are misspelled. |
| A backend exits with a `FRONTEND_ORIGIN` error | Use a plain origin such as `http://localhost:5173`, with no wildcard, path or trailing slash. |
| Frontend calls the wrong backend | `VITE_API_URL` and `COMPOSE_PROFILES` are out of sync. Fix `.env`, then run `docker compose up -d frontend`. |
| Old backend still running after switching | The profile was changed before `docker compose down`. Stop it with `docker compose stop <service>`. |
| Edits do not appear in the browser | Check `docker compose logs -f frontend`, and that `USE_POLLING` is `"true"` for the frontend service. |
| Port 5173 already in use | A local `npm run dev` or another container holds it. The API only allows 5173, so free it rather than changing the port. |
| Port 5001 or 5002 already in use | Stop either the local process or the container using it. |
| NestJS image fails to build while installing `bcrypt` | Its native build did not work. Switch to `bcryptjs`, which has the same API. |
| Valid code rejected | Check your computer's clock. TOTP depends on accurate time. |

## Project Structure

```text
auth-rosetta/
├── docker-compose.yml       # Database, frontend and backend containers
├── init-database.sql        # Postgres initialization script
├── requests.http            # API test requests, shared by every backend
├── .env                     # Local secrets and backend selection (ignored in git)
├── .env.example             # Example .env file for reference
├── docs/
│   └── api-contract.md      # The contract every backend must implement
├── frontend-react/          # The shared React application
│   ├── Dockerfile           # Vite dev server image
│   └── src/
│       ├── api/             # fetch wrapper and endpoint functions
│       ├── components/      # shared UI pieces
│       └── pages/           # Register, Login, Success
├── backend-net/             # .NET 10 implementation
│   ├── Dockerfile
│   ├── AuthRosetta.Api/             # Endpoints, middleware, composition root
│   ├── AuthRosetta.Application/     # Use cases and interfaces
│   └── AuthRosetta.Infrastructure/  # EF Core, BCrypt, Otp.NET, JWT
├── backend-nest/            # NestJS implementation
│   ├── Dockerfile
│   └── src/
│       ├── auth/            # Controller, AuthService, guard, password/TOTP/token services, DTOs
│       ├── users/           # User entity
│       ├── common/          # Exception filter, JSON middleware, decorators
│       ├── config/          # Environment validation
│       ├── database/        # TypeORM options, data source, migrations
│       ├── app.module.ts
│       ├── app.setup.ts     # CORS, cookies, validation, filters
│       └── main.ts
├── backend-python/          # Python implementation
├── backend-go/              # Go implementation
└── backend-java/            # Java implementation
```
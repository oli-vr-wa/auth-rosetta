# Auth Rosetta

A multi-stack monorepo exploring how different backend ecosystems natively implement the exact same Two-Factor Authentication (TOTP) login flow.

This project serves as a "Rosetta Stone" for backend development. It uses a single React SPA that can be hot-swapped to point at different backend APIs. Each backend connects to its own isolated PostgreSQL database, allowing for a pure comparison of how each language and framework handles database migrations, password hashing, TOTP secret generation, JWT creation, cookie handling, validation and error handling.

## Architecture

- **Frontend:** React + Vite (TypeScript) + Tailwind CSS
- **Infrastructure:** Docker Compose (shared PostgreSQL container with logical DB isolation)
- **Backends:**

| Stack | Compose profile | Port | Database | Status |
|---|---|---|---|---|
| **.NET 10** (C# + Entity Framework Core) | `net` | 5001 | `auth_net` | Completed |
| **NestJS** (Node + TypeScript + TypeORM) | `nest` | 5002 | `auth_nest` | Completed |
| **Python** (FastAPI + SQLAlchemy + Alembic) | `python` | 5003 | `auth_python` | Completed |
| **Go** (standard library + pgx + goose) | `go` | 5004 | `auth_go` | Completed |
| **Java** (Spring Boot 4.2 + JdbcClient + Flyway) | `java` | 5005 | `auth_java` | Completed |

Every backend implements the same [API contract](docs/api-contract.md), so the frontend works with any of them by changing a couple of environment variables. A shared black-box test suite, [`contract-tests/`](contract-tests/), checks that each backend really does: it only speaks HTTP, so the same file runs against every stack.

## The flow

1. **Register** with an email and password. The API returns a TOTP secret and an `otpauth://` URI.
2. **Scan the QR code** with an authenticator app and enter a code to confirm. This enables 2FA. The authenticator entry is labelled with the stack (for example `AuthRosetta-Go`), so entries from different backends are easy to tell apart.
3. **Log in** with email, password and a current 6-digit code.
4. The API sets the session in an **`HttpOnly` cookie**. The browser sends it automatically and JavaScript never sees the token.
5. The **success page** calls `/me` to prove the session works. **Logout** revokes the token server-side and clears the cookie.

## Prerequisites

- [Docker](https://www.docker.com/) with Docker Compose
- [.NET 10 SDK](https://dotnet.microsoft.com/download) and the EF Core CLI (`dotnet tool install --global dotnet-ef`), needed to run .NET migrations from your machine
- [Node.js](https://nodejs.org/) 22.12 or newer (20.19+ also works). Needed to run NestJS migrations from your machine, and to run the frontend or NestJS outside Docker
- [Python](https://www.python.org/downloads/) 3.14 (the version the backend image uses), with the `pip` option ticked in the installer. Needed to run the Python migrations and tests from your machine, and to run the contract tests
- [Go](https://go.dev/dl/) 1.27 or newer (`go.mod` requires 1.27.4; on Windows, `winget install GoLang.Go`, then open a new terminal). Needed to run the Go migrations and tests from your machine, and to run the Go backend outside Docker
- [Java](https://adoptium.net/) 25 (Eclipse Temurin LTS). Needed to run the Java tests or Spring Boot app outside Docker (the Maven Wrapper `./mvnw` is included)

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

# Optional: DEBUG shows why a login or token was rejected (never logs secrets).
# Python, Go and Java backends. Use upper case: DEBUG, INFO, WARNING or ERROR.
# LOG_LEVEL=INFO
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

Two rules about this file:

- **Shell variables beat `.env`.** Docker Compose, the contract tests, and the backends all give an environment variable set in your terminal priority over the same name in `.env`. A stale value (for example `COMPOSE_PROFILES`) makes Compose start the wrong backend even after you edit the file. List suspects with `Get-ChildItem Env: | Where-Object Name -match "COMPOSE|VITE|FRONTEND|JWT|^DB_"` and clear one with `Remove-Item Env:NAME`.
- **Only plain values belong in `.env`.** Compose's `${VAR:-default}` syntax goes in `docker-compose.yml`, never in `.env`. The backends read `.env` literally and reject it.

### 2. Start everything

```bash
docker compose up -d --build
```

This starts:

- **PostgreSQL** (always), which creates an isolated database for each stack (`auth_net`, `auth_nest`, `auth_python`, `auth_go`, `auth_java`)
- **The frontend** (always), with the Vite dev server and hot reload
- **The backend** selected by `COMPOSE_PROFILES`

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

Migrations are run from your machine (except Java, which runs Flyway automatically on startup), which works because Postgres publishes port 5432. Containers for .NET, NestJS, Python and Go do not apply them automatically, so requests that touch the database fail until they are applied.

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

**Python** (Alembic)

Create the virtual environment once, from inside `backend-python`:

```powershell
cd backend-python
py -3.14 -m venv .venv
.\.venv\Scripts\Activate.ps1          # a new terminal needs this again
python -m pip install --upgrade pip
pip install -r requirements-dev.txt
```

On macOS or Linux, use `python3.14 -m venv .venv` and `source .venv/bin/activate`. Then:

```bash
alembic upgrade head
```

Create a new migration after changing a model, then **read the generated file** in `migrations/versions/` before applying it, because autogenerate is a best guess:

```bash
alembic revision --autogenerate -m "<Name>"
```

`alembic current` shows where the database is, and `alembic downgrade -1` undoes the last migration. Run `alembic`, `pytest` and `uvicorn` from inside `backend-python`, because they find the `app` package from the current directory. The connection settings come from the root `.env`; set `DB_PORT` there only if Postgres is published on a port other than 5432.

**Go** (goose)

```bash
cd backend-go
go run ./cmd/migrate up
```

`go run ./cmd/migrate status` lists applied and pending migrations, and `go run ./cmd/migrate down` undoes the last one. Running `up` twice is safe: the second run reports that it is already up to date.

The migrations are plain SQL files in `backend-go/migrations/`, compiled into the `migrate` command. Create a new migration by adding the next numbered file, for example `00002_<name>.sql`, with a `-- +goose Up` section and a `-- +goose Down` section, then run `up` again. Go does not generate migrations from the code, so write the SQL yourself and name constraints explicitly (the repository relies on the name `uq_users_email` to detect duplicate emails).

Run every Go command from inside `backend-go`. The programs read the root `.env` from `../.env`, and carry on without it if the file is not found, so running from another folder shows up as missing-variable errors.

**Java** (Flyway)

Flyway runs automatically when `backend-java` starts (both in Docker and via `./mvnw spring-boot:run`), applying versioned SQL scripts from `backend-java/src/main/resources/db/migration/` (starting with `V1__init.sql`).

To create a new migration, add `V2__<description>.sql` in that directory and restart the application. To inspect or run migrations manually via Maven:

```bash
cd backend-java
./mvnw flyway:info       # Windows PowerShell: .\mvnw.cmd flyway:info
```

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
   If Compose still builds the old backend, a shell variable is overriding `.env` (see [Environment variables](#1-environment-variables)).
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

**Python** (port 5003)

```bash
docker compose stop backend-python
cd backend-python
# with the virtual environment active (see step 3)
uvicorn app.main:app --reload --port 5003
```

**Go** (port 5004)

```bash
docker compose stop backend-go
cd backend-go
go run ./cmd/api
```

Go has no hot reload built in; stop the server with Ctrl+C and run it again after a change.

**Java** (port 5005)

```bash
docker compose stop backend-java
cd backend-java
./mvnw spring-boot:run    # Windows PowerShell: .\mvnw.cmd spring-boot:run
```

Outside Docker, all five load the root `.env` automatically. `npm run start:dev` also prints the debug log lines that explain why a login or token was rejected. The Nest container hides them, since it runs with `NODE_ENV=production`. To see them in that container, add `NODE_ENV: development` to the service's `environment` block in `docker-compose.yml`. The Python, Go and Java backends use `LOG_LEVEL=DEBUG` in the root `.env` instead, both outside Docker and in the container.

### 7. Testing the API without the frontend

[`requests.http`](requests.http) runs the whole flow (register, 2FA, login, logout, and token revocation) against any backend. Open it in VS Code with the [REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) extension, set `@baseUrl` to the stack under test, and send the requests top to bottom. The comments at the top explain the manual authenticator step.

HTTP editors keep a **cookie jar**, and a leftover session cookie is sent even when the request preview does not show it. That can turn an anonymous check into an authenticated one, so a `200` where you expect `401` usually means a stale cookie, not a backend bug. Restart the editor to clear its cookies, or use `curl` for the anonymous and revoked-token checks. In Windows PowerShell, type `curl.exe`, because plain `curl` is an alias for something else:

```powershell
curl.exe -i http://localhost:5002/api/auth/me -H "Cookie: access_token=not-a-real-token"
```

The full checklist is at the end of the [API contract](docs/api-contract.md#manual-test-checklist). For an automated check of every rule, use the contract tests below.

### 8. Contract tests

[`contract-tests/`](contract-tests/) is the executable form of the [API contract](docs/api-contract.md). It is a black-box suite: it only sends HTTP requests, so the same file verifies every backend. It covers registration rules, malformed and oversized bodies, the exact error messages, login failures, cookie attributes, token claims and forged tokens, logout revocation, CORS, and a concurrent double registration.

**The target comes from the root `.env`.** The suite tests whatever is running at `VITE_API_URL`, uses `FRONTEND_ORIGIN` for the CORS checks, and stops with a clear message if `COMPOSE_PROFILES` and the port disagree. It also warns when a shell variable is overriding `.env`. The first lines of every run name the target:

```text
contract target: http://localhost:5003  (CORS origin http://localhost:5173)
  base URL from .env
  active backend profile: python
```

Run it with the stack up and the database migrated, using any Python environment that has the three test packages (the `backend-python` virtual environment qualifies):

```bash
pip install -r contract-tests/requirements.txt     # once
cd contract-tests
python -m pytest -v -rf
```

To test another backend, follow [Switching backends](#4-switching-backends) and run the same command. For one-off runs, `CONTRACT_BASE_URL` and `CONTRACT_ORIGIN` override the values from `.env`.

Things to know:

- **It tests whatever answers at the URL.** A server you started on your machine and forgot about would be tested instead of the container. Stop local servers before `docker compose up`.
- **Every run leaves test users behind** (emails like `contract-<random>@example.com`). Clean them out of the database you tested, for example:
```bash
docker compose exec postgres psql -U <DB_USER> -d auth_python -c "DELETE FROM users WHERE email LIKE 'contract-%@example.com';"
```
  Use `auth_net`, `auth_nest`, `auth_go` or `auth_java` for the other stacks, and check the table name if you changed it.
- **The timing check is a rough sanity test**, not a benchmark. If only that one test fails, run it once more before investigating.

**Python's own tests** (unit tests with in-memory fakes, plus in-process HTTP tests) need no database or running server:

```bash
cd backend-python
python -m pytest -q
```

**Go's own tests** work the same way. The default run needs no database or running server:

```bash
cd backend-go
go test ./... -count=1
go vet ./...
```

A second group, the Postgres repository tests, is hidden behind the `integration` build tag, so the default run skips it. It needs Postgres running and the Go migrations applied, and it removes the rows it creates:

```bash
go test -count=1 -tags integration ./internal/store -v
```

`-count=1` turns off Go's test result cache, which matters for tests that talk to a database. If your editor reports "No packages found" for `users_integration_test.go`, set gopls's `buildFlags` to `["-tags=integration"]` in `.vscode/settings.json`.

**Java's own tests** work the same way. The default run executes all unit and MockMvc slice tests without needing a database or running server:

```bash
cd backend-java
./mvnw test               # Windows PowerShell: .\mvnw.cmd test
```

Tests tagged with `@Tag("integration")` (PostgreSQL repository tests and full-stack HTTP + Postgres lifecycle tests) are excluded from the default run and can be executed with Postgres running:

```bash
./mvnw test -Dgroups=integration    # Windows PowerShell: .\mvnw.cmd test -Dgroups=integration
```

## Security design

- Passwords are hashed with **bcrypt** (cost 12), and limited to 72 bytes, bcrypt's input limit. Longer passwords are rejected at registration instead of being silently truncated, and a NUL character in a password does not cut it short.
- TOTP follows RFC 6238: SHA-1, 6 digits, 30-second period, with a ±1 step verification window.
- Sessions are JWTs (HS256) delivered in an **`HttpOnly`, `SameSite=Strict`** cookie, so injected scripts cannot read the token.
- Logout stores the token's `jti` in a denylist, so a token is rejected even before it expires.
- Login failures always return the same generic `401`, and an unknown email takes about as long as a wrong password.
- Errors are generic to the client and specific in the server log. Logs never contain passwords, tokens, codes, cookie values or database parameters.
- Input is validated by the backend, and every error response has the same `{ "error": "..." }` shape. Request bodies over 8 KiB are rejected with `400`, and text the database cannot store (NUL characters, lone surrogates) is rejected or treated as a failed attempt instead of causing a `500`.

This is a learning project, and the [API contract](docs/api-contract.md#known-limitations-documented-on-purpose) lists its deliberate limitations (plaintext TOTP secrets, no rate limiting, no CSRF token beyond `SameSite`, tokens accepted across backends, and more).

## Troubleshooting

| Problem | Fix |
|---|---|
| DBeaver shows only the `postgres` database | Tick **Show all databases** in the connection's Main tab, or set the Database field to `auth_net`, `auth_nest`, `auth_python`, `auth_go` or `auth_java`. |
| A stack's database does not exist | The init script did not run. Reset with `docker compose down -v` then `docker compose up -d --build`. |
| Login succeeds but `/me` returns 401 | You are probably on `127.0.0.1`. Use `http://localhost:5173`. |
| `/me` returns 200 when it should be 401 in `requests.http` | The editor's cookie jar sent a leftover session cookie. Restart the editor, or test with `curl.exe`. |
| CORS error in the browser console | Check the frontend runs on port 5173 and the API allows credentials. If you changed `FRONTEND_ORIGIN`, it must match the page's origin exactly. |
| A backend exits with `Missing required environment variables` | `DB_USER`, `DB_PASSWORD` or `JWT_KEY` is missing from the root `.env`, or the variable names are misspelled. The Go and Java backends print `invalid configuration` followed by every missing or invalid variable. |
| A backend exits with a `FRONTEND_ORIGIN` error | Use a plain origin such as `http://localhost:5173`, with no wildcard, path or trailing slash. Compose's `${VAR:-default}` syntax belongs in `docker-compose.yml`, never in `.env`. |
| Compose starts the wrong backend, or ignores your `.env` edits | A variable with the same name is set in your shell and wins. List them with `Get-ChildItem Env: \| Where-Object Name -match "COMPOSE\|VITE\|FRONTEND\|JWT\|^DB_"`, clear with `Remove-Item Env:NAME`, and run `docker compose --profile "*" down` before starting again. |
| Frontend calls the wrong backend | `VITE_API_URL` and `COMPOSE_PROFILES` are out of sync. Fix `.env`, then run `docker compose up -d frontend`. |
| Old backend still running after switching | The profile was changed before `docker compose down`. Stop it with `docker compose stop <service>`. |
| Edits do not appear in the browser | Check `docker compose logs -f frontend`, and that `USE_POLLING` is `"true"` for the frontend service. |
| Port 5173 already in use | A local `npm run dev` or another container holds it. The API only allows 5173, so free it rather than changing the port. |
| Port 5001, 5002, 5003, 5004 or 5005 already in use | Stop either the local process or the container using it. A forgotten local run (`go run ./cmd/api` or `.\mvnw.cmd spring-boot:run`) is a common cause. |
| NestJS image fails to build while installing `bcrypt` | Its native build did not work. Switch to `bcryptjs`, which has the same API. |
| Valid code rejected | Check your computer's clock. TOTP depends on accurate time. |
| Contract tests say `Nothing is answering` | The stack is not up or the backend crashed. Run `docker compose ps` and `docker compose logs --tail 50 <service>`. |
| Contract tests stop with `COMPOSE_PROFILES=... expects port ...` | `COMPOSE_PROFILES` and `VITE_API_URL` in `.env` disagree. Fix one of them. |
| Python: `No module named 'app'` when running `pytest` or `alembic` | Run the command from inside `backend-python`. |
| Python: `ImportError` from `pydantic_core` or `psycopg_binary` | The virtual environment mixes packages built for different Python versions (usually after changing Python version on top of an old `.venv`). Delete `.venv`, recreate it with `py -3.14 -m venv .venv` and reinstall. |
| Go: `go` is not recognized right after installing it | The installer changed PATH. Close the terminal and open a new one. |
| Go: `invalid configuration` for variables that are in `.env` | You ran the command from outside `backend-go`, so `../.env` was not found. Run Go commands from inside `backend-go`. |
| Go: the integration tests say `database not reachable` | Postgres is not running, or the Go migrations have not been applied. Run `docker compose up -d postgres` and `go run ./cmd/migrate up`. |
| Go on Windows prints `unlinkat ... being used by another process` after `go test`, or `exit status 0xc000013a` after Ctrl+C | Both are harmless. The first is antivirus scanning Go's temporary test binary at the moment it is deleted. The second is how Windows reports Ctrl+C. |
| `docker compose exec backend-go sh` fails | The Go image is distroless and has no shell. Use `docker compose logs backend-go`, or temporarily switch the last stage of `backend-go/Dockerfile` to `gcr.io/distroless/static-debian13:debug-nonroot`, which includes one. |
| Java: `Unexpected +/- setting in VM option 'MaxRAMPercentage=75.0'` | `MaxRAMPercentage` is a numeric JVM flag (`-XX:MaxRAMPercentage=75.0`), not a boolean flag (`-XX:+...`). Remove the `+` in `Dockerfile` or `JAVA_TOOL_OPTIONS`. |
| Java: `mvnw.cmd` fails with `JAVA_HOME not found` | Install Java 25 (Eclipse Temurin) and ensure `JAVA_HOME` points to the JDK directory, then open a new terminal. |
| A tool cannot read `.env` or `requirements.txt` (garbled text or empty result) | The file was saved as UTF-16, which Windows PowerShell 5.1 does with `>`. Re-save it as UTF-8, or write it with `Out-File -Encoding ascii`. |

## Project Structure

```text
auth-rosetta/
├── docker-compose.yml       # Database, frontend and backend containers
├── init-database.sql        # Postgres initialization script
├── requests.http            # API test requests, shared by every backend
├── .env                     # Local secrets and backend selection (ignored in git)
├── .env.example             # Example .env file for reference
├── docs/
│   ├── api-contract.md      # The contract every backend must implement
│   └── comparison.md        # How each stack solves the same problems
├── contract-tests/          # Black-box suite that verifies any backend over HTTP
│   ├── test_contract.py     # The tests
│   ├── target.py            # Picks the backend to test from the root .env
│   ├── conftest.py          # Target report and reachability check
│   └── requirements.txt
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
│   ├── Dockerfile
│   ├── requirements.txt         # Runtime dependencies (pinned)
│   ├── requirements-dev.txt     # Runtime plus test tools
│   ├── alembic.ini
│   ├── migrations/              # Alembic migrations
│   ├── pytest.ini
│   ├── tests/                   # Unit and in-process HTTP tests (no database needed)
│   └── app/
│       ├── api/                 # Routes, request schemas, error handling, cookies, body limit
│       ├── security/            # Passwords, TOTP, tokens
│       ├── auth_service.py      # Use cases
│       ├── ports.py             # Repository interfaces (typing.Protocol)
│       ├── repositories.py      # SQLAlchemy implementations
│       ├── models.py            # Tables
│       ├── validation.py        # Business rules (email and password policy)
│       ├── db.py                # Engine, sessions, naming convention
│       ├── config.py            # Environment settings with startup validation
│       └── main.py
├── backend-go/              # Go implementation
│   ├── Dockerfile               # Multi-stage build onto a distroless image
│   ├── go.mod, go.sum           # Dependencies (pgx, goose, x/crypto, pquerna/otp, golang-jwt)
│   ├── cmd/
│   │   ├── api/                 # The server and its composition root
│   │   └── migrate/             # Migration runner (up, down, status)
│   ├── migrations/              # SQL migrations, compiled into the migrate command
│   └── internal/                # Tests sit next to the code as *_test.go
│       ├── api/                 # Routes, handlers, middleware, JSON decoding, cookies, errors
│       ├── auth/                # Use cases, shared types, repository interfaces
│       ├── security/            # Passwords, TOTP, tokens
│       ├── store/               # pgx repositories (Postgres)
│       ├── validation/          # Business rules (email and password policy)
│       └── config/              # Environment settings with startup validation
└── backend-java/            # Java implementation (Spring Boot 4.2 + Java 25)
    ├── Dockerfile               # Multi-stage build onto eclipse-temurin:25-jre-alpine
    ├── pom.xml                  # Dependencies (Spring Web, JDBC, Flyway, jBCrypt, java-jwt)
    ├── mvnw, mvnw.cmd           # Maven Wrapper
    └── src/
        ├── main/
        │   ├── java/com/authrosetta/java/
        │   │   ├── api/         # Controllers, filters, strict JSON reader, cookies, errors
        │   │   ├── auth/        # Use cases, domain records, repository interfaces
        │   │   ├── security/    # Passwords, RFC 6238 TOTP, HS256 JWTs
        │   │   ├── store/       # JdbcClient repositories (Postgres)
        │   │   ├── validation/  # Business rules (email and password policy)
        │   │   └── config/      # Environment settings with startup validation
        │   └── resources/
        │       └── db/migration/ # Flyway SQL migrations (V1__init.sql)
        └── test/                # Unit, MockMvc slice, and @Tag("integration") Postgres tests
```
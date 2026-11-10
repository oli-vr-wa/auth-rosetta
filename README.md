# Auth Rosetta

A multi-stack monorepo exploring how different backend ecosystems natively implement the exact same Two-Factor Authentication (TOTP) login flow.

This project serves as a "Rosetta Stone" for backend development. It uses a single React SPA that can be hot-swapped to point at different backend APIs. Each backend connects to its own isolated PostgreSQL database, allowing for a pure comparison of how each language and framework handles database migrations, password hashing, TOTP secret generation, JWT creation, and cookie handling.

## Architecture

* **Frontend:** React + Vite (TypeScript) + Tailwind CSS
* **Infrastructure:** Docker Compose (shared PostgreSQL container with logical DB isolation)
* **Backends:**

| Stack | Port | Database | Status |
|---|---|---|---|
| **.NET 10** (C# + Entity Framework Core) | 5001 | `auth_net` | Completed |
| **NestJS** (Node + TypeScript) | 5002 | `auth_nest` | Working |
| **Python** | 5003 | `auth_python` | Planned |
| **Go** | 5004 | `auth_go` | Planned |
| **Java** (Spring Boot) | 5005 | `auth_java` | Planned |

Every backend implements the same [API contract](docs/api-contract.md), so the frontend works with any of them by changing one environment variable.

## The flow

1. **Register** with an email and password. The API returns a TOTP secret and an `otpauth://` URI.
2. **Scan the QR code** with an authenticator app and enter a code to confirm. This enables 2FA.
3. **Log in** with email, password and a current 6-digit code.
4. The API sets the session in an **`HttpOnly` cookie**. The browser sends it automatically and JavaScript never sees the token.
5. The **success page** calls `/me` to prove the session works. **Logout** revokes the token server-side and clears the cookie.

## Prerequisites

* [Docker](https://www.docker.com/) with Docker Compose
* [.NET 10 SDK](https://dotnet.microsoft.com/download) (only for running the .NET backend outside Docker, and for migrations)
* [Node.js](https://nodejs.org/) (current LTS) and npm
* The EF Core CLI: `dotnet tool install --global dotnet-ef`

## Getting Started

### 1. Environment variables

Create a `.env` file in the root directory (copy `.env.example`):

```env
DB_USER=rosetta_admin
DB_PASSWORD=your_secure_password
JWT_KEY=paste-a-long-random-value-here
```

`JWT_KEY` signs the JWTs and must be at least 32 bytes. Generate one with either command:

```powershell
# PowerShell (Windows PowerShell 5.1 and 7+)
$b = New-Object byte[] 48; [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b); [Convert]::ToBase64String($b)
```

```bash
# macOS, Linux, Git Bash
openssl rand -base64 48
```

If the value ever gets mangled, wrap it in double quotes. The `.env` file is ignored by git, so never commit it.

### 2. Database

The project uses a single Postgres container that automatically initializes an isolated database for each stack (`auth_net`, `auth_nest`, `auth_python`, `auth_go`, `auth_java`).

```bash
docker compose up -d
```

Verify the databases exist:

```bash
docker exec -it rosetta_db psql -U rosetta_admin -c "\l"
```

The init script only runs when the `postgres_data` volume is first created. To start over from scratch (this **deletes all data**):

```bash
docker compose down -v
docker compose up -d
```

### 3. Backend (.NET)

Apply the migrations from your machine. This works because Postgres publishes port 5432:

```bash
dotnet ef database update --project backend-net/AuthRosetta.Infrastructure --startup-project backend-net/AuthRosetta.Api
```

Then run the API in one of two ways.

**Option A: Docker** (everything in containers):

```bash
docker compose --profile net up -d --build
docker compose logs -f backend-net
```

**Option B: locally** (hot reload and debugging):

```bash
dotnet run --project backend-net/AuthRosetta.Api
```

Both listen on `http://localhost:5001`. Only run one at a time, since they share the port.

To create a new migration after changing the model:

```bash
dotnet ef migrations add <Name> --project backend-net/AuthRosetta.Infrastructure --startup-project backend-net/AuthRosetta.Api
```

The API loads the root `.env` automatically when running locally, and receives the same variables from Docker Compose when containerized.

### 4. Frontend

```bash
cd frontend-react
npm install
cp .env.example .env     # Windows PowerShell: Copy-Item .env.example .env
npm run dev
```

Open **http://localhost:5173**.

The frontend `.env` points at the backend to use:

```env
VITE_API_URL=http://localhost:5001
```

To switch backends, change the port (see the table above), **restart `npm run dev`**, and log out or clear your cookies for `localhost` first. Cookies are shared across ports on the same host, so a session from one backend can confuse another.

Always use `localhost`, not `127.0.0.1`. The session cookie is `SameSite=Strict` and the API only allows the `http://localhost:5173` origin, so using `127.0.0.1` makes login look successful while the next request fails.

### 5. Testing the API without the frontend

[`requests.http`](requests.http) runs the whole flow (register, 2FA, login, logout, and token revocation) against any backend. Open it in VS Code with the [REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) extension, set `@baseUrl` to the stack under test, and send the requests top to bottom. The comments at the top explain the manual authenticator step.

## Security design

* Passwords are hashed with **bcrypt** (cost 12).
* TOTP follows RFC 6238: SHA-1, 6 digits, 30-second period, with a ±1 step verification window.
* Sessions are JWTs (HS256) delivered in an **`HttpOnly`, `SameSite=Strict`** cookie, so injected scripts cannot read the token.
* Logout stores the token's `jti` in a denylist, so a token is rejected even before it expires.
* Login failures always return the same generic `401`.

This is a learning project, and the [API contract](docs/api-contract.md#known-limitations-documented-on-purpose) lists its deliberate limitations (plaintext TOTP secrets, no rate limiting, no CSRF token beyond `SameSite`, and more).

## Troubleshooting

| Problem | Fix |
|---|---|
| DBeaver shows only the `postgres` database | Tick **Show all databases** in the connection's Main tab, or set the Database field to `auth_net`. |
| `auth_net` does not exist | The init script did not run. Reset with `docker compose down -v` then `docker compose up -d`. |
| Login succeeds but `/me` returns 401 | You are probably on `127.0.0.1`. Use `http://localhost:5173`. |
| CORS error in the browser console | Check the frontend runs on port 5173 and the API allows credentials. |
| Vite starts on port 5174 | Another process holds 5173. Stop it, since the API only allows 5173. |
| Port 5001 already in use | Stop either `dotnet run` or the container. |
| Valid code rejected | Check your computer's clock. TOTP depends on accurate time. |

## Project Structure

```text
auth-rosetta/
├── docker-compose.yml       # Shared database and backend containers
├── init-database.sql        # Postgres initialization script
├── requests.http            # API test requests, shared by every backend
├── .env                     # Local secrets (ignored in git)
├── .env.example             # Example .env file for reference
├── docs/
│   └── api-contract.md      # The contract every backend must implement
├── frontend-react/          # The shared React application
│   └── src/
│       ├── api/             # fetch wrapper and endpoint functions
│       ├── components/      # shared UI pieces
│       └── pages/           # Register, Login, Success
├── backend-net/             # .NET 10 implementation
│   ├── AuthRosetta.Api/             # Endpoints, middleware, composition root
│   ├── AuthRosetta.Application/     # Use cases and interfaces
│   └── AuthRosetta.Infrastructure/  # EF Core, BCrypt, Otp.NET, JWT
├── backend-nest/            # NestJS implementation
├── backend-python/          # Python implementation
├── backend-go/              # Go implementation
└── backend-java/            # Java implementation
```
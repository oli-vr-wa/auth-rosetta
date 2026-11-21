# Comparing the stacks
 
Every backend implements the same [API contract](api-contract.md) and passes the same black-box [contract tests](../contract-tests/). This document records how each stack gets there: what is similar, what differs, and which problems only showed up when we ran real requests.
 
Go and Java are not covered yet. Each will get its own column and notes when it is completed.
 
Versions at the time of writing (November 2026): .NET 10, NestJS 12 on Node 22, Python 3.14 with FastAPI 0.163, SQLAlchemy 2.1, Alembic 1.20, PyJWT 2.15, PyOTP 2.12 and bcrypt 5.0.
 
## At a glance
 
| | .NET | NestJS | Python |
|---|---|---|---|
| Web framework | ASP.NET Core Minimal APIs | NestJS 12 (CommonJS) | FastAPI on Uvicorn |
| Database access | EF Core | TypeORM | SQLAlchemy 2.1 (sync) with psycopg 3 |
| Migrations | `dotnet ef` | TypeORM CLI scripts | Alembic |
| Password hashing | BCrypt.Net | `bcrypt` (native module) | `bcrypt` package, used directly |
| TOTP | Otp.NET | otplib 13 | PyOTP |
| JWT | JwtBearer and IdentityModel | `@nestjs/jwt` | PyJWT |
| Request validation | System.Text.Json options plus a rules class | class-validator DTOs through a global pipe | Pydantic strict models plus a rules module |
| Startup config check | `AppConfig` record | `env.validation.ts` | pydantic-settings `Settings` |
| Organization | Layers across three projects | Feature folders | Flat modules grouped by role |
| Abstractions | Interfaces, enforced by the compiler | Concrete classes injected | `typing.Protocol`, checked only by an optional type checker |
| Password and TOTP calls | Synchronous | Asynchronous | Synchronous, run in a thread pool |
 
## Architecture and layout
 
**.NET** is organized by layer. `Application` holds the interfaces, `AuthService` and the DTOs, `Infrastructure` holds EF Core, BCrypt, Otp.NET and the JWT service, and `Api` holds endpoints, the cookie helper and middleware. The dependency container wires interfaces to implementations in `Program.cs`.
 
**NestJS** is organized by feature (`auth/`, `users/`, `common/`). Services inject concrete classes, because TypeScript interfaces vanish at runtime. TypeORM's repository replaces a hand-written repository class.
 
**Python** starts flat: `models.py`, `repositories.py`, `auth_service.py`, `ports.py`, with the HTTP layer in an `api/` package. This is normal in Python, where the unit of organization is the module and "one class per file" is not a convention. The usual advice is to split a module when it grows past a few hundred lines or mixes unrelated things.
 
What the abstractions buy and cost:
 
* In .NET, a repository that does not match its interface fails to compile.
* In Python, `ports.py` uses `typing.Protocol`, so a class satisfies the interface just by having the right methods, with no inheritance. Our test fakes need no base class. Only a type checker such as pyright verifies the match, and we do not run one. A mismatch would surface when the line runs.
* The practical payoff is the same in all three: the service can be tested with in-memory fakes, without a database.
* Python wires its dependencies by hand in `get_auth_service`, the equivalent of `AddScoped<IUserRepository, UserRepository>()`.
## Configuration and startup
 
The contract requires a backend to refuse to start when `DB_USER`, `DB_PASSWORD` or `JWT_KEY` is missing, when `JWT_KEY` is under 32 characters, or when `FRONTEND_ORIGIN` is not a plain origin. Each stack has one place that checks this: the `AppConfig` record in .NET, `env.validation.ts` in Nest, and the `Settings` class in Python.
 
Details worth knowing:
 
* **Secrets in error output.** pydantic prints the offending value by default, which would write a too-short `JWT_KEY` to the console. `hide_input_in_errors=True` prevents that. This is the kind of leak the contract's "never log secrets" rule is meant to stop.
* **Where `.env` comes from.** The root `.env` is read directly outside Docker (.NET via DotNetEnv, Python via pydantic-settings). Inside containers, Docker Compose supplies real environment variables and `.env` is not read.
* **Environment beats `.env`.** Docker Compose, the contract tests and the Python backend all let a variable set in your shell override the same name in `.env`. This cost us several hours: a stale `COMPOSE_PROFILES` made Compose build the wrong backend, and a shell copy of `FRONTEND_ORIGIN` containing Compose's `${VAR:-default}` syntax broke startup validation. The contract suite now warns when a shell variable differs from `.env`.
* **Compose syntax is not for `.env`.** `${VAR:-default}` belongs in `docker-compose.yml`. Every backend reads `.env` literally and rejects it.
## Database and migrations
 
All three stacks run migrations from the host against the published Postgres port, so containers never migrate on startup. That keeps failures visible and avoids replicas racing each other.
 
| | .NET | NestJS | Python |
|---|---|---|---|
| Apply | `dotnet ef database update` | `npm run migration:run` | `alembic upgrade head` |
| Generate | `dotnet ef migrations add` | `npm run migration:generate` | `alembic revision --autogenerate` |
| Schema sync at runtime | Never | Never (`synchronize: false`) | Never |
 
* **Always read generated migrations.** Autogenerate compares the models with the live database and makes a best guess.
* **Name constraints explicitly.** In Python, a `naming_convention` on the metadata gives the unique email constraint a stable name (`uq_users_email`). Without it, Postgres invents a name and migrations cannot refer to it reliably.
* **Alembic builds its connection from `Settings`**, not from `alembic.ini`, so the password never lives in a config file. This mirrors how .NET builds its connection string with `NpgsqlConnectionStringBuilder`.
* **Nest's migration chain needs relative imports.** Files used by `data-source.ts` (config, entities) cannot use path aliases, because ts-node does not resolve them.
### Duplicate emails under concurrency
 
Checking "does this email exist?" before inserting is not enough: two simultaneous requests can both pass that check. Every backend relies on the database's unique constraint and turns the violation into a `409`. The contract suite fires eight simultaneous registrations of one email and expects exactly one `201`.
 
In Python, the repository returns `False` only when the unique violation is for the email constraint, and re-raises any other integrity error so real bugs stay `500`s. It matches the constraint by name. A one-character typo in that name (`uq_user_email` against `uq_users_email`) passed every unit test, because the fakes never produce a constraint name, and only the real-database run caught it. A matching test now compiles the model's DDL and checks the name we match on.
 
## Passwords
 
All three use bcrypt at cost 12 and cap passwords at 72 bytes at registration.
 
* **The cap.** bcrypt cannot use more than 72 bytes. Older libraries silently truncate. bcrypt 5.x in Python raises `ValueError` instead. Either way, the register rules enforce the cap and the hashing call is never given a longer password. A password over 72 bytes at *login* is an ordinary failed login, never a `500`, so Python checks the length before calling bcrypt and spends dummy-hash time instead.
* **Unknown emails.** Login always does the expensive hash work. For an unknown email it verifies against a dummy hash, so response time does not reveal which accounts exist. The contract suite checks this with a loose timing comparison, which is a sanity check and not a benchmark.
* **NUL characters.** bcrypt implementations that treat NUL as the end of a string would silently shorten a password, so `pass\0one` and `pass\0two` would collide. We measured Python's bcrypt, and it does not truncate. The contract does not force a behaviour: a backend either rejects NUL in passwords or handles them correctly, and the suite fails only on truncation.
* **Text encoding.** Python strings can hold lone surrogates that cannot be encoded as UTF-8, and encoding raises an error. .NET and JavaScript strings can hold them too, but UTF-8 encoding substitutes U+FFFD by default, which would let two different passwords verify as equal. The contract therefore treats lone surrogates as invalid input.
* **Threading.** Hashing is a blocking call. FastAPI runs plain `def` endpoints in a worker thread, so bcrypt does not stall the server. In Node the same call is asynchronous.
## TOTP
 
All three use SHA-1, 6 digits, a 30-second period, a ±1 step window, a 20-byte Base32 secret and the issuer `AuthRosetta`.
 
* **Library shape.** otplib 13 uses a functional API (`generateSecret`, `generateURI`, `verify({secret, token, epochTolerance: 30})`) and differs from v12 tutorials. PyOTP uses `TOTP(secret).verify(code, valid_window=1)`, and `random_base32()` returns 32 characters, which is 20 bytes. The Python code writes the contract's digits, digest and interval out explicitly instead of relying on defaults.
* **A malformed code is just an invalid code.** Wrong length, non-digits and digits from other scripts must return `Invalid email or code`, never a `500`. Python checks `[0-9]{6}` before calling the library. `str.isdigit()` accepts digits from other scripts, and so does `\d` in Python's `re`, so the explicit class is deliberate.
* **Format rules belong in the TOTP check, not in request validation.** In Nest, a `@Matches` rule on the DTO's `code` field made the global pipe reject `12345` with `Invalid request` before the service ever ran, which contradicts the contract. The DTO now only checks that `code` is a string.
* **No replay protection**, as the contract states. PyOTP's docs describe `verify()` as stateless too.
## JWT
 
All three issue HS256 tokens with `iss`, `aud`, `sub`, `email`, `jti` and `exp`, and verify with a 30-second clock skew.
 
* **Only HS256 is accepted.** .NET uses `ValidAlgorithms`, Python passes `algorithms=["HS256"]` to PyJWT. The contract suite forges tokens with `alg: none`, a different HMAC algorithm, a wrong key and a tampered payload.
* **Required claims.** Python requires `exp`, `sub` and `jti`, and rejects a token whose claims have the wrong types.
* **Claim names.** .NET needs `MapInboundClaims = false` to keep `sub` and `email` as they are. Nest sets them with the `subject` and `jwtid` options.
* **Keep PyJWT current.** Several recent security fixes concern HMAC key handling and algorithm allow-lists, so Python pins `pyjwt>=2.15.1`.
## Cookies
 
The session is an `HttpOnly`, `SameSite=Strict` cookie named `access_token`, `Secure` only when the request was HTTPS.
 
* **A fractional `Max-Age` is silently discarded.** Python's `EXPIRY.total_seconds()` is a float, so the first version sent `Max-Age=3600.0`. Cookie parsers ignore a non-integer `Max-Age`, the client never stored the cookie, and `/me` returned `401` right after a successful login. Wrapping it in `int(...)` fixed it. The original test checked `"max-age=3600"` as a substring, which matched `3600.0` too, so the contract tests now match the whole number with a regular expression.
* **Cookies are scoped by host, not port.** A session from one backend is sent to every backend on `localhost`. Because all backends share one `JWT_KEY`, issuer and audience, one backend also accepts another's token, while revocation is stored per database.
* **HTTP editors have cookie jars.** Visual Studio's HTTP editor can turn an anonymous request into an authenticated one. Use `curl.exe` for anonymous and replay checks.
## Validation and errors
 
The split is the same everywhere: **wire-format problems** (broken JSON, a missing, null or wrongly typed field) are handled in the API layer and become `400 Invalid request`. **Business rules** (password and email policy) live in the application layer and raise a small exception that the API layer maps to `400` with the rule's message.
 
| | .NET | NestJS | Python |
|---|---|---|---|
| Wire format | `RespectNullableAnnotations` and `RespectRequiredConstructorParameters` | `ValidationPipe` with an `exceptionFactory` returning `Invalid request` | Pydantic models in strict mode |
| Business rules | `RegisterRules` throws `RequestValidationException` | Service-level checks | `validate_register` raises `RequestValidationError` |
| Central error mapping | `ErrorHandlingMiddleware` | `ApiExceptionFilter` | Exception handlers plus a catch-all middleware |
 
Differences and gotchas:
 
* **.NET: `ThrowOnBadRequest`.** It defaults to `true` only in Development. We set it explicitly so malformed JSON reaches our middleware in every environment.
* **Python: FastAPI's default 422.** Validation failures are overridden to the contract's `400 Invalid request`. The handler logs where and what kind of problem occurred, never the input values, because they can be passwords. FastAPI's validation exception and our rule exception have the same name, so one is imported under an alias.
* **Python: the catch-all.** The generic `500` handler is a middleware placed *inside* the CORS middleware, and a test proves that unexpected errors still carry CORS headers. Without CORS headers the browser would show a CORS error instead of the real failure.
* **.NET: middleware order.** `UseCors` runs before the error middleware for the same reason. A parameterless `app.UseCors()` crashes at startup in .NET 10, so always pass a named policy.
* **Unknown route and wrong method.** Python disables its auto-generated docs and trailing-slash redirects so that unknown routes return the JSON `404`. A wrong method gives `405` in Python and `404` in .NET (its fallback route matches every method). The contract allows either.
* **Text the database cannot store.** A NUL character, and lone surrogates, make Postgres or the driver fail, which would be a `500`. At register this is a `400`. At login and confirm it is treated as a failed attempt. .NET has a `TextRules` class, and Python has `is_storable_text`.
### Generic to the client, specific in the logs
 
Each stack logs the real reason at warning or debug level and returns the contract's generic message. Two leaks needed explicit attention in Python:
 
* SQLAlchemy errors include the SQL parameter values by default, which for a failed `INSERT INTO users` means the password hash and the TOTP secret. `hide_parameters=True` on the engine replaces them with a placeholder, and we confirmed it in the container logs.
* pydantic echoes invalid input unless told not to (see Configuration above).
## Request size limit
 
The contract says a body over 8 KiB gets `400 Invalid request`. Each stack needed something different:
 
* **NestJS.** The JSON parser's `limit` option rejects the body with `413`. The exception filter maps `413` to `400`.
* **.NET.** Kestrel has `MaxRequestBodySize`, but the framework's body handling did not turn that into the contract's `400` in our setup. We enforce the limit in a small middleware that checks `Content-Length` and wraps the request stream to count bytes, throwing our own exception that the error middleware maps to `400`. We did not confirm exactly how the framework handles Kestrel's own exception.
* **Python.** Starlette has no body limit at all. An ASGI middleware checks the declared length, and counts bytes while reading for chunked bodies. The contract suite tests both a declared-length body and a chunked one.
A single wrong operator in the Python middleware (`==` where `!=` was meant) made it skip every HTTP request, and the only symptom was that oversized bodies were processed normally, which is why the tests assert the `400` and not only the code path.
 
## Secure by default
 
| | Approach |
|---|---|
| .NET | A fallback authorization policy requires authentication everywhere, and public endpoints opt out with `AllowAnonymous`. A side effect: unmatched routes also returned `401` until we added `MapFallback(...).AllowAnonymous()` to give unknown paths the contract's `404`. Its `401` responses also carry a `WWW-Authenticate: Bearer` header, which the contract does not call for |
| NestJS | The guard is applied to the endpoints that need it. A global guard with a `@Public()` decorator is on the hardening list |
| Python | Protected endpoints depend on `get_current_claims`, so protection is opt-in per route |
 
## Testing
 
* **Python unit tests** use in-memory fakes through the `Protocol` interfaces, so the whole login flow runs in seconds without a database. They also test the service rules (normalization, duplicate handling, odd input).
* **Python HTTP tests** use Starlette's test client with the fakes injected, covering status codes, exact bodies, cookies, CORS and the logging rules (including that no secret appears in the log).
* **The contract suite** is the same file for every backend. It reads the root `.env` to find its target, stops if `COMPOSE_PROFILES` and the port disagree, and warns when a shell variable overrides `.env`.
What the contract suite caught that the other tests could not, because it talks to the real database and the real server:
 
* a constraint-name typo that turned every lost registration race into a `500`
* a fractional `Max-Age` that made the browser drop the cookie
* a middleware condition that silently disabled the size limit
* a validation rule on Nest's DTO that returned the wrong error message
* unhandled odd characters on Nest and .NET, which Python had already covered
Habits worth keeping:
 
* **Beware assertions that can never fail.** A trap such as `assert x is None or True` passes forever. Make wrong inputs deterministic instead.
* **Match whole values, not substrings,** when the difference matters (`max-age=3600` also matches `3600.0`).
* **Compare the stacks with exact literals for contract messages,** and import nothing from the app into the contract tests, so a changed message fails loudly.
## Docker
 
* **Image.** The Python image is a single stage, `python:3.14-slim`. Everything installs as a prebuilt wheel and nothing needs compiling, so there is no build stage to discard. The Nest image uses two stages on `node:22-slim` because its native bcrypt module has to be built.
* **Bind address.** Uvicorn must run with `--host 0.0.0.0`, because the default `127.0.0.1` inside a container cannot be reached through the published port.
* **Non-root, unbuffered.** The Python image runs as an unprivileged user, and `PYTHONUNBUFFERED` makes `docker compose logs` live.
* **Hosts.** Inside containers the database is `postgres`, not `localhost` (`DB_HOST`). The Postgres init script only runs when the data volume is first created.
* **Only the app is copied.** Tests, migrations and the virtual environment stay out of the image (`.dockerignore`).
## Framework and tooling surprises
 
| Stack | Surprise |
|---|---|
| .NET | `UseCors()` without a policy name crashes at startup in .NET 10. `ThrowOnBadRequest` is Development-only by default. `MapInboundClaims = false` is needed to keep `sub` and `email` |
| NestJS | The ESM versus CJS prompt and the Node 22.12+ requirement. otplib v13 differs from v12 tutorials. Decorated signatures need `import type` (TS1272). The `cors` package echoes a single string origin always, while a list omits the header for non-matching origins |
| Python | Windows installs without `pip` unless the box is ticked. Reusing an old `.venv` after a Python upgrade leaves compiled packages (`pydantic_core`, `psycopg_binary`) built for the old version, which fail with confusing `ImportError`s. `pytest` and `alembic` find the `app` package from the current directory. Starlette's test client now prefers the `httpx2` package and warns when it falls back to `httpx`. Windows PowerShell 5.1 writes UTF-16 with `>`, which breaks `requirements.txt` and `.env` |
 
## Still to compare
 
* Go and Java, with their migration tools, validation and error handling.
* The Nest and .NET Docker images, which are not described here yet.
* Per-stack JWT audiences, so that tokens from one backend are not accepted by another.
* A global secure-by-default guard in Nest.
 
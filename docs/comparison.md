# Comparing the stacks

Every backend implements the same [API contract](api-contract.md) and passes the same black-box [contract tests](../contract-tests/). This document records how each stack gets there: what is similar, what differs, and which problems only showed up when we ran real requests.

Versions at the time of writing (November 2026): .NET 10, NestJS 12 on Node 22, Python 3.14 with FastAPI 0.163, SQLAlchemy 2.1, Alembic 1.20, PyJWT 2.15, PyOTP 2.12 and bcrypt 5.0, Go 1.27.4 with pgx 5.13, goose 3.30, `golang.org/x/crypto` 0.59, `pquerna/otp` 1.5 and `golang-jwt/jwt` 5.3.1, and Java 25 (Eclipse Temurin LTS) with Spring Boot 4.2.0, Flyway, Jackson 3 (`tools.jackson.core`), jBCrypt 0.4 and Auth0 `java-jwt` 4.5.

## At a glance

| | .NET | NestJS | Python | Go | Java |
|---|---|---|---|---|---|
| Web framework | ASP.NET Core Minimal APIs | NestJS 12 (CommonJS) | FastAPI on Uvicorn | Standard library `net/http` (method and path patterns in `ServeMux`) | Spring Boot 4.2 WebMVC on embedded Tomcat |
| Database access | EF Core | TypeORM | SQLAlchemy 2.1 (sync) with psycopg 3 | pgx v5 with `pgxpool` and hand-written SQL | Spring `JdbcClient` with HikariCP and hand-written SQL |
| Migrations | `dotnet ef` | TypeORM CLI scripts | Alembic | goose (SQL files compiled into `cmd/migrate`) | Flyway (embedded SQL scripts run automatically at startup) |
| Password hashing | BCrypt.Net | `bcrypt` (native module) | `bcrypt` package, used directly | `golang.org/x/crypto/bcrypt` | jBCrypt (`org.mindrot.jbcrypt`) |
| TOTP | Otp.NET | otplib 13 | PyOTP | `pquerna/otp` | Hand-written RFC 6238 (`javax.crypto.Mac` HmacSHA1 + `java.security.SecureRandom`) |
| JWT | JwtBearer and IdentityModel | `@nestjs/jwt` | PyJWT | `golang-jwt/jwt/v5` | Auth0 `java-jwt` |
| Request validation | System.Text.Json options plus a rules class | class-validator DTOs through a global pipe | Pydantic strict models plus a rules module | `encoding/json/v2` strict decoding into pointer fields, plus a rules package | Streaming Jackson 3 `JsonFactory` (`StrictJsonReader`) plus `ValidationRules` |
| Startup config check | `AppConfig` record | `env.validation.ts` | pydantic-settings `Settings` | `config.Load`, which reports every problem at once | `AppConfig.load()`, collecting all errors before failing startup |
| Organization | Layers across three projects | Feature folders | Flat modules grouped by role | `cmd/` for programs, `internal/` packages grouped by role | Single Maven module with packages grouped by role |
| Abstractions | Interfaces, enforced by the compiler | Concrete classes injected | `typing.Protocol`, checked only by an optional type checker | Small interfaces declared by the code that uses them, satisfied structurally and checked by the compiler where they are wired | Interfaces in `auth/` (`UserRepository`, `RevokedTokenRepository`), implemented by `@Repository` classes in `store/` |
| Password and TOTP calls | Synchronous | Asynchronous | Synchronous, run in a thread pool | Synchronous, each request in its own goroutine | Synchronous, each request on a Tomcat worker thread |

## Architecture and layout

**.NET** is organized by layer. `Application` holds the interfaces, `AuthService` and the DTOs, `Infrastructure` holds EF Core, BCrypt, Otp.NET and the JWT service, and `Api` holds endpoints, the cookie helper and middleware. The dependency container wires interfaces to implementations in `Program.cs`.

**NestJS** is organized by feature (`auth/`, `users/`, `common/`). Services inject concrete classes, because TypeScript interfaces vanish at runtime. TypeORM's repository replaces a hand-written repository class.

**Python** starts flat: `models.py`, `repositories.py`, `auth_service.py`, `ports.py`, with the HTTP layer in an `api/` package. This is normal in Python, where the unit of organization is the module and "one class per file" is not a convention. The usual advice is to split a module when it grows past a few hundred lines or mixes unrelated things.

**Go** puts each runnable program under `cmd/` (`cmd/api` is the server and the composition root, `cmd/migrate` runs migrations) and everything else under `internal/`, which the compiler forbids other modules from importing. A package is a folder, and the packages are grouped by role: `api` (routes, handlers, middleware), `auth` (the service and shared types), `security` (passwords, TOTP, tokens), `store` (pgx repositories), `validation` and `config`. `main.go` is the only file that knows every concrete type.

**Java** uses a single Maven project (`backend-java`) with packages grouped by role under `com.authrosetta.java`: `api` (`AuthController`, `HealthController`, `HttpContractFilter`, `StrictJsonReader`, `ApiExceptionHandler`, `AuthCookies`), `auth` (`AuthService`, domain records, and repository interfaces), `security` (`PasswordHasher`, `TotpService`, `JwtService`), `store` (`PostgresUserRepository`, `PostgresRevokedTokenRepository`), `validation` (`ValidationRules`), and `config` (`AppConfig`). Spring's component scan wires constructor-injected beans automatically, while `AppConfig` is registered via a `@Bean` factory method on the main application class so startup fails immediately if environment configuration is invalid.

What the abstractions buy and cost:

- In .NET and Java, a repository that does not match its interface fails to compile.
- In Python, `ports.py` uses `typing.Protocol`, so a class satisfies the interface just by having the right methods, with no inheritance. Our test fakes need no base class. Only a type checker such as pyright verifies the match, and we do not run one. A mismatch would surface when the line runs.
- In Go, interfaces are structural like Python's `Protocol`, but the compiler checks them wherever a value is assigned to an interface type. `store.Users` never mentions `auth.UserStore`; the compiler verifies the match when `main.go` wires them together. The handlers' `AuthService` interface also has a compile-time assertion, `var _ AuthService = (*auth.Service)(nil)`, which costs nothing at runtime.
- Go's convention is that an interface belongs to the code that **uses** it, not the code that implements it, and it stays small. Only the I/O boundaries are interfaces here (the two stores, and the service seen by the handlers). Passwords and tokens are plain computation, so the service holds the concrete types and tests build them with a cheap bcrypt cost and a pinned clock.
- The practical payoff is the same in all five: the service can be tested with in-memory fakes, without a database.
- Python wires its dependencies by hand in `get_auth_service`, the equivalent of `AddScoped<IUserRepository, UserRepository>()`. Go does the same in `main.go`, once, at startup. Spring Boot auto-wires constructors across packages, which means missing `@Repository` or `@Component` stereotypes surface at context initialization time (and `@WebMvcTest` slices require importing `TestConfig` when filters depend on `AppConfig`).

## Configuration and startup

The contract requires a backend to refuse to start when `DB_USER`, `DB_PASSWORD` or `JWT_KEY` is missing, when `JWT_KEY` is under 32 characters, or when `FRONTEND_ORIGIN` is not a plain origin. Each stack has one place that checks this: the `AppConfig` record in .NET, `env.validation.ts` in Nest, the `Settings` class in Python, `config.Load` in Go, and `AppConfig.load()` in Java.

Details worth knowing:

- **Secrets in error output.** pydantic prints the offending value by default, which would write a too-short `JWT_KEY` to the console. `hide_input_in_errors=True` prevents that. This is the kind of leak the contract's "never log secrets" rule is meant to stop. In Go and Java, the validation errors never include a value, tests check that a short key is not echoed, and `Settings` / `AppConfig` override `String()` / `toString()` to redact `dbPassword` and `jwtKey` so printing the config object cannot leak secrets.
- **Where `.env` comes from.** The root `.env` is read directly outside Docker (.NET via DotNetEnv, Python via pydantic-settings, Go and Java through small hand-written `.env` loaders that read `../.env` when present without overriding real environment variables). Inside containers, Docker Compose supplies real environment variables and `.env` is not read. Go and Java skip a missing `.env` file silently, which makes the same binary/JAR work in both places.
- **Environment beats `.env`.** Docker Compose, the contract tests, and all five backends let a variable set in your shell override the same name in `.env`. This cost us several hours: a stale `COMPOSE_PROFILES` made Compose build the wrong backend, and a shell copy of `FRONTEND_ORIGIN` containing Compose's `${VAR:-default}` syntax broke startup validation. The contract suite now warns when a shell variable differs from `.env`. - **Compose syntax is not for `.env`.** `${VAR:-default}` belongs in `docker-compose.yml`. Every backend reads `.env` literally and rejects it.
- **Reporting every problem at once.** Both Go (`errors.Join`) and Java (`AppConfig.load()`) collect every configuration failure into one error list, so a single startup attempt lists a missing password and a short key together.
- **Relative `.env` paths in Go and Java.** Both look for `../.env` from the current working directory, so local runs should be started from inside `backend-go` or `backend-java`.

## Database and migrations

The first four stacks run migrations from the host against the published Postgres port, so containers never migrate on startup. Java is the one exception: Spring Boot's Flyway integration runs `src/main/resources/db/migration/V1__init.sql` automatically at application startup (after `rosetta_db` passes its health check in Compose).

| | .NET | NestJS | Python | Go | Java |
|---|---|---|---|---|---|
| Apply | `dotnet ef database update` | `npm run migration:run` | `alembic upgrade head` | `go run ./cmd/migrate up` | Automatic at startup via Flyway (or `./mvnw flyway:migrate`) |
| Generate | `dotnet ef migrations add` | `npm run migration:generate` | `alembic revision --autogenerate` | Written by hand: the next numbered `.sql` file | Written by hand: `V<N>__<name>.sql` in `db/migration/` |
| Schema sync at runtime | Never | Never (`synchronize: false`) | Never | Never (there is no ORM) | Never (Flyway validates checksums and applies versioned SQL only) |

- **Always read generated migrations.** Autogenerate compares the models with the live database and makes a best guess. Go and Java have no generator, so there is nothing to misread, and the cost is writing the SQL yourself.
- **Name constraints explicitly.** In Python, a `naming_convention` on the metadata gives the unique email constraint a stable name (`uq_users_email`). Without it, Postgres invents a name and migrations cannot refer to it reliably. In Go and Java the SQL is hand-written, so `CONSTRAINT uq_users_email UNIQUE (email)` is spelled out directly in the migration.
- **No plaintext passwords in config files.** Alembic builds its connection from `Settings`, .NET uses `NpgsqlConnectionStringBuilder`, Go's `Settings.DatabaseURL()` uses `net/url` to escape special characters, and Java builds the JDBC URL programmatically from `AppConfig` and passes `username`/`password` separately to `DataSourceBuilder` (avoiding URL-escaping issues altogether).
- **Nest's migration chain needs relative imports.** Files used by `data-source.ts` (config, entities) cannot use path aliases, because ts-node does not resolve them.
- **Go and Java package migrations into the artifact.** Go uses `//go:embed *.sql` into `cmd/migrate`. Java packages `db/migration/*.sql` inside the Spring Boot fat JAR on the classpath, so the container needs no external migration files.

### Duplicate emails under concurrency

Checking "does this email exist?" before inserting is not enough: two simultaneous requests can both pass that check. Every backend relies on the database's unique constraint and turns the violation into a `409`. The contract suite fires eight simultaneous registrations of one email and expects exactly one `201`.

In Python, the repository returns `False` only when the unique violation is for the email constraint, and re-raises any other integrity error so real bugs stay `500`s. It matches the constraint by name. A one-character typo in that name (`uq_user_email` against `uq_users_email`) passed every unit test, because the fakes never produce a constraint name, and only the real-database run caught it. A matching test now compiles the model's DDL and checks the name we match on.

Go and Java follow the same rule: `Add` / `insert` returns `false` only for SQLSTATE `23505` on `uq_users_email` (found by unwrapping the `PgException` / `PSQLException` cause chain), and rethrows any other failure (including primary-key clashes) as a real error. Both Go (`-tags integration`) and Java (`-Dgroups=integration`) verify this against real Postgres in their own test suites.

The same idea applies to logout. Revoking a token uses `INSERT ... ON CONFLICT (jti) DO NOTHING` in Python, Go and Java, so two simultaneous logouts with one token are idempotent and cannot fail.

## Passwords

All five use bcrypt at cost 12 and cap passwords at 72 bytes at registration.

- **The cap.** bcrypt cannot use more than 72 bytes. Older libraries silently truncate. bcrypt 5.x in Python raises `ValueError` instead, and Go's `Hash` returns an error for a 73-byte password (a test documents it). Either way, the register rules enforce the 72-byte UTF-8 cap and the hashing call is never given a longer password. A password over 72 bytes at *login* is an ordinary failed login, never a `500`, so Python, Go and Java check the UTF-8 byte length before calling bcrypt and spend dummy-hash time instead.
- **Unknown emails.** Login always does the expensive hash work. For an unknown email it verifies against a dummy hash, so response time does not reveal which accounts exist. The contract suite checks this with a loose timing comparison, which is a sanity check and not a benchmark. In Go and Java the dummy hash is generated at startup using the configured cost (cost 12 in production, lower cost in unit tests) rather than hardcoded as a static string, ensuring the dummy check always matches the active cost factor.
- **NUL characters.** bcrypt implementations that treat NUL as the end of a string would silently shorten a password, so `pass\0one` and `pass\0two` would collide. Python's bcrypt and Go's `x/crypto/bcrypt` handle NUL bytes without truncation, while Java's `ValidationRules` rejects NUL characters (`\u0000`) outright at registration and treats them as failed credentials at login.
- **Text encoding.** Python strings can hold lone surrogates that cannot be encoded as UTF-8, and encoding raises an error. .NET, JavaScript and Java strings are UTF-16 code units and can hold unpaired surrogates, where UTF-8 encoding substitutes `U+FFFD` by default—which would let two different passwords verify as equal. The contract therefore treats lone surrogates as invalid input. Go strings are plain bytes (`utf8.ValidString`), while .NET (`TextRules`), Python (`is_storable_text`), and Java (`ValidationRules.isStorableText` checking `Character.isSurrogate` and `\0`) reject lone surrogates before persistence or hashing. Java also distinguishes `password.codePointCount(0, password.length())` (for the 8-character minimum) from `password.getBytes(StandardCharsets.UTF_8).length` (for the 72-byte bcrypt limit), matching Go's `utf8.RuneCountInString` vs `len(s)`.
- **Threading.** Hashing is a blocking call. FastAPI runs plain `def` endpoints in a worker thread, so bcrypt does not stall the server. In Node the same call is asynchronous. In Go every request runs in its own goroutine, and in Java each request runs on an embedded Tomcat worker thread.

## TOTP

All use SHA-1, 6 digits, a 30-second period, a ±1 step window and a 20-byte Base32 secret. The issuer label names the stack (for example `AuthRosetta-Go`, `AuthRosetta-Java`), so authenticator entries from different backends are easy to tell apart. The label is not part of the contract, and it is a different thing from the JWT `iss` claim, which is `AuthRosetta` everywhere.

- **Library shape.** otplib 13 uses a functional API (`generateSecret`, `generateURI`, `verify({secret, token, epochTolerance: 30})`) and differs from v12 tutorials. PyOTP uses `TOTP(secret).verify(code, valid_window=1)`, and `random_base32()` returns 32 characters, which is 20 bytes. Go's `pquerna/otp` has `totp.Generate` and `totp.ValidateCustom`. For Java, rather than pulling in a unmaintained third-party TOTP library or a QR-rendering dependency we don't need, `TotpService` implements RFC 6238 / RFC 4226 directly in ~80 lines using the JDK's built-in `javax.crypto.Mac` (`HmacSHA1`), `java.security.SecureRandom`, and a Base32 encoder/decoder.
- **Prove the configuration with published test vectors.** A "generate a code, then verify it" test passes even if the settings are wrong, because both sides use the same wrong settings. Both Go and Java tests verify against the RFC 6238 Appendix B SHA-1 test secret (`12345678901234567890`) and timestamps (`59`, `1111111109`, `1234567890`), whose 6-digit codes are `287082`, `081804` and `005924`. Two of those have leading zeros, which catches padding mistakes, and a wrong digest, digit count or period makes all three fail. Both take an injectable clock (`Instant` in Java, `time.Time` in Go) so the ±1 step window boundaries can be tested deterministically.
- **A malformed code is just an invalid code.** Wrong length, non-digits and digits from other scripts must return `Invalid email or code`, never a `500`. Python checks `[0-9]{6}` before calling the library (`str.isdigit()` and `\d` accept non-ASCII digits). Go checks each byte is `'0'..'9'`, and Java checks `code.length() == 6` with every `char` between `'0'` and `'9'`.
- **Format rules belong in the TOTP check, not in request validation.** In Nest, a `@Matches` rule on the DTO's `code` field made the global pipe reject `12345` with `Invalid request` before the service ever ran, which contradicts the contract. In all five stacks, request validation checks only that `code` is a string.
- **No replay protection**, as the contract states.

## JWT

All five issue HS256 tokens with `iss`, `aud`, `sub`, `email`, `jti` and `exp`, and verify with a 30-second clock skew.

- **Only HS256 is accepted.** .NET uses `ValidAlgorithms`, Python passes `algorithms=["HS256"]` to PyJWT, Go passes `jwt.WithValidMethods([]string{"HS256"})`, and Java builds its verifier with `JWT.require(Algorithm.HMAC256(secret))` (which rejects `alg: none` and any other algorithm). All suites test forged tokens with `alg: none`, wrong keys, and tampered payloads.
- **Required claims.** Python requires `exp`, `sub` and `jti`. Go requires `exp` via `WithExpirationRequired` and checks `sub`, `email` and `jti` are non-empty strings. Java configures `.withIssuer("AuthRosetta")`, `.withAudience("AuthRosetta")`, `.acceptLeeway(30)` (which enforces `exp` with a 30-second leeway), and explicitly rejects tokens missing `sub`, `email` or `jti` or where `aud` is not a single string.
- **Claim names and audience format.** .NET needs `MapInboundClaims = false` to keep `sub` and `email` as they are. Go sets `jwt.MarshalSingleStringAsArray = false` so `aud` is written as a plain string rather than a 1-element array. Auth0 `java-jwt` writes `.withAudience("AuthRosetta")` as a single string by default when one audience is passed, matching the other stacks.
- **Any failure is the same answer.** Go's `Decode` returns `false` and Java's `verify` returns `Optional.empty()` for every kind of failure, logging the reason at debug level without leaking token contents.

## Cookies

The session is an `HttpOnly`, `SameSite=Strict` cookie named `access_token`, `Secure` only when the request was HTTPS.

- **A fractional `Max-Age` is silently discarded.** Python's `EXPIRY.total_seconds()` is a float, so the first version sent `Max-Age=3600.0`. Cookie parsers ignore a non-integer `Max-Age`, the client never stored the cookie, and `/me` returned `401` right after a successful login. Wrapping it in `int(...)` fixed it. Go's `http.Cookie.MaxAge` is an `int`, and Java uses Spring's `ResponseCookie.from("access_token", token).maxAge(Duration.ofHours(1))`, which formats `Max-Age=3600` as an integer.
- **Clearing the cookie on logout.** Go deletes a cookie with `MaxAge: -1` (since `0` means omit in `net/http`). Spring's `ResponseCookie` writes `Max-Age=0` when passed `.maxAge(0)` (or `Duration.ZERO`), repeating `Path=/`, `HttpOnly`, and `SameSite=Strict`.
- **The `Secure` flag follows the request scheme.** Go checks `r.TLS != nil`, Python checks `request.url.scheme == "https"`, and Java checks `request.isSecure() || "https".equalsIgnoreCase(request.getHeader("X-Forwarded-Proto"))`.
- **Cookies are scoped by host, not port.** A session from one backend is sent to every backend on `localhost`. Because all five backends share one `JWT_KEY`, issuer and audience, one backend also accepts another's token, while revocation is stored per database.

## Validation and errors

The split is the same everywhere: **wire-format problems** (broken JSON, a missing, null or wrongly typed field) are handled in the API layer and become `400 Invalid request`. **Business rules** (password and email policy) live in the application layer and raise a small exception that the API layer maps to `400` with the rule's message.

| | .NET | NestJS | Python | Go | Java |
|---|---|---|---|---|---|
| Wire format | `RespectNullableAnnotations` and `RespectRequiredConstructorParameters` | `ValidationPipe` with an `exceptionFactory` returning `Invalid request` | Pydantic models in strict mode | `encoding/json/v2`, strict by default, into `*string` fields | Streaming Jackson 3 `JsonFactory` (`StrictJsonReader`) requiring string tokens |
| Business rules | `RegisterRules` throws `RequestValidationException` | Service-level checks | `validate_register` raises `RequestValidationError` | `ValidateRegister` returns a `*ValidationError` | `ValidationRules` throws `ValidationException` |
| Central error mapping | `ErrorHandlingMiddleware` | `ApiExceptionFilter` | Exception handlers plus a catch-all middleware | Handlers map sentinel errors with `errors.Is` and `errors.As`; a `recoverer` middleware catches panics | `@RestControllerAdvice` (`ApiExceptionHandler`) |

Differences and gotchas:

- **.NET: `ThrowOnBadRequest`.** It defaults to `true` only in Development. We set it explicitly so malformed JSON reaches our middleware in every environment.
- **Python: FastAPI's default 422.** Validation failures are overridden to the contract's `400 Invalid request`.
- **Middleware / filter ordering and CORS.** In Python, .NET, Go and Java, CORS headers must be written *before* error handling or body-limit rejection so browsers receive `Access-Control-Allow-Origin` on `400`, `401`, `413->400`, and `500` responses. In Java, `HttpContractFilter` runs as a single `OncePerRequestFilter` that writes CORS headers first, handles `OPTIONS` preflight, enforces the 8,192-byte body cap before Spring MVC ever reads the stream, and catches uncaught `Throwable`s into a JSON `500`.
- **Unknown route and wrong method.** Python returns `405` for wrong methods and `404` for unknown paths. .NET and Go return JSON `404` for both. In Java, Spring Boot's `ApiExceptionHandler` maps both `NoResourceFoundException` (`404`) and `HttpRequestMethodNotSupportedException` (`405`) to the contract's `{"error":"Invalid request"}` JSON shape.
- **Text the database cannot store.** A NUL character, and lone surrogates, make Postgres or the driver fail, which would be a `500`. At register this is a `400`. At login and confirm it is treated as a failed attempt. .NET (`TextRules`), Python (`is_storable_text`), Go (`validation.IsStorable`), and Java (`ValidationRules.isStorableText`) all check this before touching Postgres.
- **Strict JSON typing in Go and Java.** Default databinding in Jackson (even Jackson 3) coerces numbers or booleans into `String` fields (e.g. `{"email": 123}` becomes `"123"`) and ignores trailing JSON tokens. Java's `StrictJsonReader` uses Jackson 3's low-level streaming `JsonParser` with `StreamReadFeature.STRICT_DUPLICATE_DETECTION`, verifying `Content-Type` is `application/json`, the root token is `{`, every required field has `JsonToken.VALUE_STRING` (rejecting numbers, booleans, `null`, arrays, and objects), duplicate keys fail immediately, and no trailing tokens follow `}`.
- **Go: odd text at login and confirm.** Go's `decodeAuthBody` decodes strictly first and falls back to a lenient check solely to distinguish lone surrogates (which return `401` / `Invalid email or code`) from structural JSON errors (`400 Invalid request`). In Java, lone surrogates inside JSON `\uD800` escapes are valid UTF-16 `String` values in Jackson, so they pass `StrictJsonReader` and reach `AuthService`, where `ValidationRules.isStorableText` rejects them as failed credentials (`401` / `Invalid email or code`) naturally.

### Generic to the client, specific in the logs

Each stack logs the real reason at warning or debug level and returns the contract's generic message:

- In Python, SQLAlchemy errors include SQL parameter values by default; `hide_parameters=True` on the engine replaces them with placeholders, and `hide_input_in_errors=True` prevents pydantic from echoing invalid input.
- Go's pgx errors do not include parameter values, and `AppConfig` / `Settings` redact secrets in `String()`.
- In Java, Spring's `JdbcClient` logs SQL statements at `INFO`/`WARN` without bound parameter values unless `org.springframework.jdbc.core.StatementCreatorUtils` is explicitly set to `TRACE`, and `ApiExceptionHandler` never echoes exception messages from unhandled exceptions (`500 {"error":"Internal server error"}`).

## Request size limit

The contract says a body over 8 KiB (8,192 bytes) gets `400 Invalid request`. Each stack enforces it before JSON parsing:

- **NestJS.** The JSON parser's `limit` option rejects the body with `413`, which `ApiExceptionFilter` maps to `400`.
- **.NET.** A middleware checks `Content-Length` and wraps the request stream to count bytes, throwing a custom exception mapped to `400`.
- **Python.** An ASGI middleware checks declared `Content-Length` and counts bytes while reading chunked bodies.
- **Go.** A middleware rejects declared `Content-Length > 8192` immediately and wraps the body in `http.MaxBytesReader` for chunked streams.
- **Java.** `HttpContractFilter` checks `request.getContentLengthLong() > 8192` upfront and wraps `HttpServletRequest` in a `BoundedBodyRequestWrapper` whose `ServletInputStream` counts bytes as `StrictJsonReader` reads them, throwing `BodyTooLargeException` -> `400 {"error":"Invalid request"}` for chunked payloads exceeding 8,192 bytes.

## Secure by default

| | Approach |
|---|---|
| .NET | A fallback authorization policy requires authentication everywhere, and public endpoints opt out with `AllowAnonymous`. Unmatched routes needed `MapFallback(...).AllowAnonymous()` so they return `404` instead of `401` |
| NestJS | The guard is applied to the endpoints that need it (`/me`, `/logout`). A global guard with a `@Public()` decorator is on the hardening list |
| Python | Protected endpoints depend on `get_current_claims`, so protection is opt-in per route |
| Go | Protected handlers (`me`, `logout`) call one `authenticate` helper, so protection is opt-in per handler |
| Java | Protected controller endpoints (`me`, `logout`) extract `access_token` via `AuthCookies.readAccessToken(request)` and call `authService.authenticate(token)` / `authService.logout(token)` without pulling in Spring Security's default login form, session state, or CSRF filter chain |

## Testing

- **Python unit & HTTP tests** use in-memory fakes through `Protocol` interfaces and Starlette's test client, covering status codes, exact bodies, cookies, CORS and logging rules without needing a database.
- **Go unit & integration tests** sit next to the code (`*_test.go`), using fakes for the store interfaces in unit tests and `-tags integration` for real Postgres repository and concurrency tests.
- **Java unit, slice & integration tests** run in two tiers via JUnit 5:
  - Default `.\mvnw.cmd test` runs 69 fast tests with zero external dependencies: unit tests for `AppConfig`, `ValidationRules`, `PasswordHasher`, `TotpService` (including RFC 6238 vectors), `JwtService` (forged `alg: none`, wrong key, tampered claims), `AuthService` (with in-memory fake repositories and cheap bcrypt cost), plus `@WebMvcTest` MockMvc slice tests for `HealthController`, `HttpContractFilter` (8,192 vs 8,193 byte boundary, CORS), `StrictJsonReader`, and `AuthController`.
  - `.\mvnw.cmd test -Dgroups=integration` runs `@Tag("integration")` tests against real PostgreSQL (`auth_java`): `PostgresStoreIntegrationTest` (constraint handling, eight-way concurrent duplicate registration, idempotent concurrent token revocation) and `EndToEndAuthIntegrationTest` (full HTTP lifecycle over real embedded Tomcat sockets with `java.net.http.HttpClient`).
- **The contract suite** (`contract-tests/`) is the same black-box Python pytest file for all five backends. It reads the root `.env` to find its target, stops if `COMPOSE_PROFILES` and the port disagree, and warns when a shell variable overrides `.env`.

What the contract suite caught that unit tests could not, because it talks to the real database and the real server:

- a constraint-name typo in Python that turned every lost registration race into a `500`
- a fractional `Max-Age` in Python that made the browser drop the cookie
- a middleware condition in Python that silently disabled the size limit
- a validation rule on Nest's DTO that returned the wrong error message
- unhandled odd characters on Nest and .NET

Habits worth keeping:

- **Beware assertions that can never fail.** A trap such as `assert x is None or True` passes forever. Make wrong inputs deterministic instead.
- **Match whole values, not substrings,** when the difference matters (`max-age=3600` also matches `3600.0`).
- **Compare the stacks with exact literals for contract messages,** and import nothing from the app into the contract tests, so a changed message fails loudly.
- **Test the test builder.** Always include one case proving that the valid token/payload passes so a broken test helper cannot mask failures.
- **Measure library behaviour instead of assuming it.** How each parser handles lone surrogates, case sensitivity, duplicate keys, and NUL bytes was verified with tests rather than assumed from docs.
- **Turn off the test cache when tests touch outside state.** Go caches passing results (`(cached)`), so use `go test -count=1` for database tests.

## Docker

- **Images.**
  - **Python:** Single stage on `python:3.14-slim` (prebuilt wheels, non-root user, `PYTHONUNBUFFERED=1`).
  - **NestJS:** Two stages on `node:22-slim` to compile the native `bcrypt` addon and TypeScript output.
  - **Go:** Two stages (`golang:1.27` static binary with `CGO_ENABLED=0 -trimpath` -> `gcr.io/distroless/static-debian13:nonroot`, running as UID 65532 with no shell).
  - **Java:** Two stages (`eclipse-temurin:25-jdk-alpine` caching Maven dependencies with `./mvnw dependency:go-offline` and building the Spring Boot fat JAR -> `eclipse-temurin:25-jre-alpine` running as non-root `appuser` with `-XX:MaxRAMPercentage=75.0`).
- **Layer caching.** Both Go (`go.mod`/`go.sum`) and Java (`pom.xml`/`.mvn`) download dependencies in an earlier layer before copying `src`, so code edits rebuild quickly.
- **Bind address.** Uvicorn runs with `--host 0.0.0.0`, Go listens on `:5004`, and Spring Boot listens on `0.0.0.0:5005` (`server.port=${PORT:5005}`).
- **Hosts.** Inside containers the database host is `postgres` (service name) / `rosetta_db` (`DB_HOST`), not `localhost`. The Postgres init script creates `auth_net`, `auth_nest`, `auth_python`, `auth_go` and `auth_java` only when the `postgres_data` volume is first created.

## Framework and tooling surprises

| Stack | Surprise |
|---|---|
| .NET | `UseCors()` without a policy name crashes at startup in .NET 10. `ThrowOnBadRequest` is Development-only by default. `MapInboundClaims = false` is needed to keep `sub` and `email` |
| NestJS | The ESM versus CJS prompt and the Node 22.12+ requirement. otplib v13 differs from v12 tutorials. Decorated signatures need `import type` (TS1272). The `cors` package echoes a single string origin always, while a list omits the header for non-matching origins |
| Python | Windows installs without `pip` unless the box is ticked. Reusing an old `.venv` after a Python upgrade leaves compiled packages (`pydantic_core`, `psycopg_binary`) built for the old version, which fail with confusing `ImportError`s. `pytest` and `alembic` find the `app` package from the current directory. Starlette's test client now prefers the `httpx2` package and warns when it falls back to `httpx`. Windows PowerShell 5.1 writes UTF-16 with `>`, which breaks `requirements.txt` and `.env` |
| Go | The standard library covers much more than before: method and path patterns in `ServeMux` (so no router library), `log/slog`, `uuid` (new in 1.27), `crypto/rand.Text` and `encoding/json/v2` (stable in 1.27). `go get` before any code imports a package marks it `// indirect` until `go mod tidy` runs. Build-tagged files are invisible to the editor's language server until `gopls` is given `-tags=integration`. `http.Cookie.MaxAge` of 0 means "omit" and -1 means "Max-Age=0". Headers set after `WriteHeader` are dropped. On Windows, `go test` can print `unlinkat ... being used by another process` (antivirus scanning the temporary test binary) and Ctrl+C shows `exit status 0xc000013a`, both harmless. The standard JSON encoder escapes `&` as `\u0026` unless told not to |
| Java | Spring Boot 4.2 upgrades to Jackson 3 under the `tools.jackson.*` package namespace (breaking `com.fasterxml.jackson.*` imports, renaming `JsonParser.getText()` to `getString()`, and requiring `ObjectReadContext.empty()` in `JsonFactory.createParser`). `@WebMvcTest` was relocated to `org.springframework.boot.webmvc.test.autoconfigure.WebMvcTest`, and `@MockBean` was replaced by `@MockitoBean`. In `@WebMvcTest` slices, servlet filters like `HttpContractFilter` are still loaded, so slice tests need `@Import(TestConfig.class)` to supply `AppConfig`, and nested test controllers must be registered via `@TestConfiguration` `@Bean` methods. In HotSpot JVM flags, `MaxRAMPercentage` is a numeric option (`-XX:MaxRAMPercentage=75.0`), whereas passing `-XX:+MaxRAMPercentage=75.0` fails JVM startup with `Unexpected +/- setting` |

## Still to compare

- The Nest and .NET Docker images, which are not described in detail here yet.
- Per-stack JWT audiences, so that tokens from one backend are not accepted by another.
- A global secure-by-default guard in Nest, Go, and Java.
- Odd text at login and confirm: the contract requires `401` and `Invalid email or code`, which the suite should assert strictly across all five stacks.
- Whether duplicate JSON member names should be a contract rule (Go and Java reject them strictly, while the other parsers may accept the last key).
- Purging expired rows from the `revoked_tokens` denylist across all five stacks.
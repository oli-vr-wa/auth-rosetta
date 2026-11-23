# Comparing the stacks
 
Every backend implements the same [API contract](api-contract.md) and passes the same black-box [contract tests](../contract-tests/). This document records how each stack gets there: what is similar, what differs, and which problems only showed up when we ran real requests.
 
Java is not covered yet. It will get its own column and notes when it is completed.
 
Versions at the time of writing (November 2026): .NET 10, NestJS 12 on Node 22, Python 3.14 with FastAPI 0.163, SQLAlchemy 2.1, Alembic 1.20, PyJWT 2.15, PyOTP 2.12 and bcrypt 5.0. Go 1.27.4 with pgx 5.13, goose 3.30, `golang.org/x/crypto` 0.59, `pquerna/otp` 1.5 and `golang-jwt/jwt` 5.3.1.
 
## At a glance
 
| | .NET | NestJS | Python | Go |
|---|---|---|---|---|
| Web framework | ASP.NET Core Minimal APIs | NestJS 12 (CommonJS) | FastAPI on Uvicorn | Standard library `net/http` (method and path patterns in `ServeMux`) |
| Database access | EF Core | TypeORM | SQLAlchemy 2.1 (sync) with psycopg 3 | pgx v5 with `pgxpool` and hand-written SQL |
| Migrations | `dotnet ef` | TypeORM CLI scripts | Alembic | goose (SQL files compiled into `cmd/migrate`) |
| Password hashing | BCrypt.Net | `bcrypt` (native module) | `bcrypt` package, used directly | `golang.org/x/crypto/bcrypt` |
| TOTP | Otp.NET | otplib 13 | PyOTP | `pquerna/otp` |
| JWT | JwtBearer and IdentityModel | `@nestjs/jwt` | PyJWT | `golang-jwt/jwt/v5` |
| Request validation | System.Text.Json options plus a rules class | class-validator DTOs through a global pipe | Pydantic strict models plus a rules module | `encoding/json/v2` strict decoding into pointer fields, plus a rules package |
| Startup config check | `AppConfig` record | `env.validation.ts` | pydantic-settings `Settings` | `config.Load`, which reports every problem at once |
| Organization | Layers across three projects | Feature folders | Flat modules grouped by role | `cmd/` for programs, `internal/` packages grouped by role |
| Abstractions | Interfaces, enforced by the compiler | Concrete classes injected | `typing.Protocol`, checked only by an optional type checker | Small interfaces declared by the code that uses them, satisfied structurally and checked by the compiler where they are wired |
| Password and TOTP calls | Synchronous | Asynchronous | Synchronous, run in a thread pool | Synchronous, each request in its own goroutine |
 
## Architecture and layout
 
**.NET** is organized by layer. `Application` holds the interfaces, `AuthService` and the DTOs, `Infrastructure` holds EF Core, BCrypt, Otp.NET and the JWT service, and `Api` holds endpoints, the cookie helper and middleware. The dependency container wires interfaces to implementations in `Program.cs`.
 
**NestJS** is organized by feature (`auth/`, `users/`, `common/`). Services inject concrete classes, because TypeScript interfaces vanish at runtime. TypeORM's repository replaces a hand-written repository class.
 
**Python** starts flat: `models.py`, `repositories.py`, `auth_service.py`, `ports.py`, with the HTTP layer in an `api/` package. This is normal in Python, where the unit of organization is the module and "one class per file" is not a convention. The usual advice is to split a module when it grows past a few hundred lines or mixes unrelated things.
 
**Go** puts each runnable program under `cmd/` (`cmd/api` is the server and the composition root, `cmd/migrate` runs migrations) and everything else under `internal/`, which the compiler forbids other modules from importing. A package is a folder, and the packages are grouped by role: `api` (routes, handlers, middleware), `auth` (the service and shared types), `security` (passwords, TOTP, tokens), `store` (pgx repositories), `validation` and `config`. `main.go` is the only file that knows every concrete type.
 
What the abstractions buy and cost:
 
* In .NET, a repository that does not match its interface fails to compile.
* In Python, `ports.py` uses `typing.Protocol`, so a class satisfies the interface just by having the right methods, with no inheritance. Our test fakes need no base class. Only a type checker such as pyright verifies the match, and we do not run one. A mismatch would surface when the line runs.
* In Go, interfaces are structural like Python's `Protocol`, but the compiler checks them wherever a value is assigned to an interface type. `store.Users` never mentions `auth.UserStore`; the compiler verifies the match when `main.go` wires them together. The handlers' `AuthService` interface also has a compile-time assertion, `var _ AuthService = (*auth.Service)(nil)`, which costs nothing at runtime.
* Go's convention is that an interface belongs to the code that **uses** it, not the code that implements it, and it stays small. Only the I/O boundaries are interfaces here (the two stores, and the service seen by the handlers). Passwords and tokens are plain computation, so the service holds the concrete types and tests build them with a cheap bcrypt cost and a pinned clock.
* The practical payoff is the same in all four: the service can be tested with in-memory fakes, without a database.
* Python wires its dependencies by hand in `get_auth_service`, the equivalent of `AddScoped<IUserRepository, UserRepository>()`. Go does the same in `main.go`, once, at startup.
## Configuration and startup
 
The contract requires a backend to refuse to start when `DB_USER`, `DB_PASSWORD` or `JWT_KEY` is missing, when `JWT_KEY` is under 32 characters, or when `FRONTEND_ORIGIN` is not a plain origin. Each stack has one place that checks this: the `AppConfig` record in .NET, `env.validation.ts` in Nest, the `Settings` class in Python, and `config.Load` in Go.
 
Details worth knowing:
 
* **Secrets in error output.** pydantic prints the offending value by default, which would write a too-short `JWT_KEY` to the console. `hide_input_in_errors=True` prevents that. This is the kind of leak the contract's "never log secrets" rule is meant to stop. In Go, the validation errors never include a value, a test checks that a short key is not echoed, and `Settings` has a `String()` method that prints `secrets=redacted`, so even `fmt.Printf("%+v", settings)` cannot leak the password.
* **Where `.env` comes from.** The root `.env` is read directly outside Docker (.NET via DotNetEnv, Python via pydantic-settings, Go through a small hand-written parser of about 25 lines, since the standard library has none). Inside containers, Docker Compose supplies real environment variables and `.env` is not read. Go skips a missing file silently, which is what makes the same binary work in both places.
* **Environment beats `.env`.** Docker Compose, the contract tests, the Python backend and the Go backend all let a variable set in your shell override the same name in `.env`. This cost us several hours: a stale `COMPOSE_PROFILES` made Compose build the wrong backend, and a shell copy of `FRONTEND_ORIGIN` containing Compose's `${VAR:-default}` syntax broke startup validation. The contract suite now warns when a shell variable differs from `.env`.
* **Compose syntax is not for `.env`.** `${VAR:-default}` belongs in `docker-compose.yml`. Every backend reads `.env` literally and rejects it.
* **Go reports every problem at once.** `errors.Join` collects all the failures, so one start-up attempt lists a missing password and a short key together. The Go tests also set every relevant variable with `t.Setenv` before reading a file, so stray variables on the developer's machine cannot change the result, the same job as the pinned environment in Python's `conftest.py`.
* **Go's `.env` path is relative.** The programs look for `../.env` from the working directory, so they must be run from `backend-go`.
## Database and migrations
 
All four stacks run migrations from the host against the published Postgres port, so containers never migrate on startup. That keeps failures visible and avoids replicas racing each other.
 
| | .NET | NestJS | Python | Go |
|---|---|---|---|---|
| Apply | `dotnet ef database update` | `npm run migration:run` | `alembic upgrade head` | `go run ./cmd/migrate up` |
| Generate | `dotnet ef migrations add` | `npm run migration:generate` | `alembic revision --autogenerate` | Written by hand: the next numbered `.sql` file |
| Schema sync at runtime | Never | Never (`synchronize: false`) | Never | Never (there is no ORM) |
 
* **Always read generated migrations.** Autogenerate compares the models with the live database and makes a best guess. Go has no generator, so there is nothing to misread, and the cost is writing the SQL yourself.
* **Name constraints explicitly.** In Python, a `naming_convention` on the metadata gives the unique email constraint a stable name (`uq_users_email`). Without it, Postgres invents a name and migrations cannot refer to it reliably. In Go the SQL is hand-written, so `CONSTRAINT uq_users_email UNIQUE (email)` is simply spelled out in the migration.
* **Alembic builds its connection from `Settings`**, not from `alembic.ini`, so the password never lives in a config file. This mirrors how .NET builds its connection string with `NpgsqlConnectionStringBuilder`. Go's `Settings.DatabaseURL()` uses `net/url`, which escapes special characters in the password, and a test round-trips a password containing `@ / : # ?`.
* **Nest's migration chain needs relative imports.** Files used by `data-source.ts` (config, entities) cannot use path aliases, because ts-node does not resolve them.
* **Go embeds its migrations.** A `//go:embed *.sql` directive compiles the SQL files into the `migrate` program, which needs no files beside it. goose is used through its `Provider` API, which avoids global state. It speaks the standard `database/sql` interface, so `cmd/migrate` uses pgx through its `database/sql` adapter (a blank import registers the driver), while the application itself uses pgx natively.
### Duplicate emails under concurrency
 
Checking "does this email exist?" before inserting is not enough: two simultaneous requests can both pass that check. Every backend relies on the database's unique constraint and turns the violation into a `409`. The contract suite fires eight simultaneous registrations of one email and expects exactly one `201`.
 
In Python, the repository returns `False` only when the unique violation is for the email constraint, and re-raises any other integrity error so real bugs stay `500`s. It matches the constraint by name. A one-character typo in that name (`uq_user_email` against `uq_users_email`) passed every unit test, because the fakes never produce a constraint name, and only the real-database run caught it. A matching test now compiles the model's DDL and checks the name we match on.
 
Go follows the same rule: `Add` returns `false` only for SQLSTATE `23505` on `uq_users_email`, found by unwrapping the pgx error with `errors.As`, and any other failure, including a primary-key clash, is a real error. The difference is where the typo is caught. Go's repository tests run against the real Postgres (behind the `integration` build tag), so the same typo makes the duplicate-email test and the eight-way concurrency test fail inside the Go suite, before the contract suite is involved. We tried it on purpose by changing the constant: the concurrency test reported seven "unexpected error" attempts.
 
The same idea applies to logout. Revoking a token uses `INSERT ... ON CONFLICT (jti) DO NOTHING` in Python and Go, so two simultaneous logouts with one token cannot fail. Go has a concurrency test for it too.
 
## Passwords
 
All four use bcrypt at cost 12 and cap passwords at 72 bytes at registration.
 
* **The cap.** bcrypt cannot use more than 72 bytes. Older libraries silently truncate. bcrypt 5.x in Python raises `ValueError` instead, and Go's `Hash` returns an error for a 73-byte password (a test documents it). Either way, the register rules enforce the cap and the hashing call is never given a longer password. A password over 72 bytes at *login* is an ordinary failed login, never a `500`, so Python and Go check the length before calling bcrypt and spend dummy-hash time instead. Go also has a test that a 73-byte password does not match the hash of its 72-byte prefix.
* **Unknown emails.** Login always does the expensive hash work. For an unknown email it verifies against a dummy hash, so response time does not reveal which accounts exist. The contract suite checks this with a loose timing comparison, which is a sanity check and not a benchmark. In Go the dummy hash is created when the `Passwords` value is built at startup, with the real cost, and a test asserts that its cost is 12. Making it a field of a struct, rather than a package-level variable that tests patch, avoids a trap: a fast test could otherwise create the cached dummy hash at a cheap cost.
* **NUL characters.** bcrypt implementations that treat NUL as the end of a string would silently shorten a password, so `pass\0one` and `pass\0two` would collide. We measured Python's bcrypt and Go's, and neither truncates (Go has a test for it). The contract does not force a behaviour: a backend either rejects NUL in passwords or handles them correctly, and the suite fails only on truncation.
* **Text encoding.** Python strings can hold lone surrogates that cannot be encoded as UTF-8, and encoding raises an error. .NET and JavaScript strings can hold them too, but UTF-8 encoding substitutes U+FFFD by default, which would let two different passwords verify as equal. The contract therefore treats lone surrogates as invalid input. Go strings are plain bytes, so a lone surrogate cannot exist in one; the equivalent problem is invalid UTF-8, which `utf8.ValidString` checks. Go also has two different lengths, and the code must pick the right one: `len(s)` counts bytes (the 72-byte cap) and `utf8.RuneCountInString(s)` counts characters (the 8-character minimum and the 256-character email limit).
* **Threading.** Hashing is a blocking call. FastAPI runs plain `def` endpoints in a worker thread, so bcrypt does not stall the server. In Node the same call is asynchronous. In Go every request already runs in its own goroutine, so a blocking hash holds up only its own request.
## TOTP
 
All use SHA-1, 6 digits, a 30-second period, a ±1 step window and a 20-byte Base32 secret. The issuer label names the stack (for example `AuthRosetta-Go`), so authenticator entries from different backends are easy to tell apart. The label is not part of the contract, and it is a different thing from the JWT `iss` claim, which is `AuthRosetta` everywhere. (The two were briefly confused during the Go work, and the token tests failed until the JWT constant was put back.)
 
* **Library shape.** otplib 13 uses a functional API (`generateSecret`, `generateURI`, `verify({secret, token, epochTolerance: 30})`) and differs from v12 tutorials. PyOTP uses `TOTP(secret).verify(code, valid_window=1)`, and `random_base32()` returns 32 characters, which is 20 bytes. The Python code writes the contract's digits, digest and interval out explicitly instead of relying on defaults. Go's `pquerna/otp` has `totp.Generate` (which returns a key with `Secret()` and `URL()`) and `totp.ValidateCustom`, and the Go code passes the algorithm, digits, period and skew explicitly as well.
* **A dependency we only half use.** `pquerna/otp` can also render QR code images, so it pulls in a QR library, although the frontend draws the QR code itself from the URI. The alternative is about 25 lines of hand-written RFC 6238 using `crypto/hmac`, which is a good exercise but riskier in security code. We chose the library.
* **Prove the configuration with published test vectors.** A "generate a code, then verify it" test passes even if the settings are wrong, because both sides use the same wrong settings. The Go tests use the RFC 6238 secret and times (59, 1111111109 and 1234567890), whose 6-digit codes are `287082`, `081804` and `005924`. Two of those have leading zeros, which catches a padding mistake, and a wrong digest, digit count or period would make all three fail. The clock is a parameter (`at time.Time`), so the window test can say exactly which step is accepted.
* **A malformed code is just an invalid code.** Wrong length, non-digits and digits from other scripts must return `Invalid email or code`, never a `500`. Python checks `[0-9]{6}` before calling the library. `str.isdigit()` accepts digits from other scripts, and so does `\d` in Python's `re`, so the explicit class is deliberate. Go checks the bytes directly: the string must be exactly 6 bytes, each between `'0'` and `'9'`, so a multi-byte digit can never pass.
* **Format rules belong in the TOTP check, not in request validation.** In Nest, a `@Matches` rule on the DTO's `code` field made the global pipe reject `12345` with `Invalid request` before the service ever ran, which contradicts the contract. The DTO now only checks that `code` is a string.
* **No replay protection**, as the contract states. PyOTP's docs describe `verify()` as stateless too.
## JWT
 
All four issue HS256 tokens with `iss`, `aud`, `sub`, `email`, `jti` and `exp`, and verify with a 30-second clock skew.
 
* **Only HS256 is accepted.** .NET uses `ValidAlgorithms`, Python passes `algorithms=["HS256"]` to PyJWT, and Go passes `jwt.WithValidMethods([]string{"HS256"})`. In Go the key comes from a closure that never looks at the token's own header. The contract suite forges tokens with `alg: none`, a different HMAC algorithm, a wrong key and a tampered payload, and the Go unit tests do the same.
* **Required claims.** Python requires `exp`, `sub` and `jti`, and rejects a token whose claims have the wrong types. Go requires `exp` through the library (`WithExpirationRequired`) and then checks that `sub`, `email` and `jti` are non-empty strings.
* **Claim names.** .NET needs `MapInboundClaims = false` to keep `sub` and `email` as they are. Nest sets them with the `subject` and `jwtid` options. In Go, a struct embeds the library's `RegisteredClaims` and adds an `email` field, so `sub`, `jti`, `iss`, `aud` and `exp` come from the embedded struct.
* **The audience is written as a plain string in Go.** The library writes `aud` as a one-element array by default. A global switch (`jwt.MarshalSingleStringAsArray = false`, set in `init()`) makes it a plain string like the other stacks, and a test inspects the raw payload: exactly six claims, and `aud` is a string.
* **Any failure is the same answer.** Go's `Decode` returns just `false` for every kind of failure and logs the reason at debug level, the same as Python returning `None`.
* **Keep the JWT library current.** Several recent security fixes concern HMAC key handling, algorithm allow-lists and header parsing. Python pins `pyjwt>=2.15.1`. In Go, an advisory (CVE-2025-30204, excessive memory use while parsing headers) was fixed in jwt v5.2.2, we are on 5.3.1, and a test that parses a token made of 100,000 dots guards against a regression.
## Cookies
 
The session is an `HttpOnly`, `SameSite=Strict` cookie named `access_token`, `Secure` only when the request was HTTPS.
 
* **A fractional `Max-Age` is silently discarded.** Python's `EXPIRY.total_seconds()` is a float, so the first version sent `Max-Age=3600.0`. Cookie parsers ignore a non-integer `Max-Age`, the client never stored the cookie, and `/me` returned `401` right after a successful login. Wrapping it in `int(...)` fixed it. The original test checked `"max-age=3600"` as a substring, which matched `3600.0` too, so the contract tests now match the whole number with a regular expression. Go's `http.Cookie.MaxAge` is an `int`, so it cannot produce a fraction, and the Go test still checks the raw header with an exact pattern.
* **Go deletes a cookie with `MaxAge: -1`.** In Go's `http.Cookie`, `MaxAge: 0` means "omit the attribute", and `-1` is what writes `Max-Age=0`. Passing 0 to log out would leave the cookie in the browser. The clearing cookie also repeats the original `Path`, `HttpOnly` and `SameSite`.
* **Go's `Secure` flag follows the connection.** It is set when `r.TLS != nil`, the same idea as Python's `request.url.scheme == "https"`. Behind a proxy that ends TLS, Go would see plain HTTP and omit it, which is fine for local development.
* **Go sends headers once.** After `WriteHeader`, further headers are silently dropped. The handlers therefore set the cookie before writing the body (or the `204`), and the handler tests read the cookie from the response.
* **Cookies are scoped by host, not port.** A session from one backend is sent to every backend on `localhost`. Because all backends share one `JWT_KEY`, issuer and audience, one backend also accepts another's token, while revocation is stored per database.
* **HTTP editors have cookie jars.** Visual Studio's HTTP editor can turn an anonymous request into an authenticated one. Use `curl.exe` for anonymous and replay checks.
## Validation and errors
 
The split is the same everywhere: **wire-format problems** (broken JSON, a missing, null or wrongly typed field) are handled in the API layer and become `400 Invalid request`. **Business rules** (password and email policy) live in the application layer and raise a small exception that the API layer maps to `400` with the rule's message.
 
| | .NET | NestJS | Python | Go |
|---|---|---|---|---|
| Wire format | `RespectNullableAnnotations` and `RespectRequiredConstructorParameters` | `ValidationPipe` with an `exceptionFactory` returning `Invalid request` | Pydantic models in strict mode | `encoding/json/v2`, strict by default, into `*string` fields |
| Business rules | `RegisterRules` throws `RequestValidationException` | Service-level checks | `validate_register` raises `RequestValidationError` | `ValidateRegister` returns a `*ValidationError` |
| Central error mapping | `ErrorHandlingMiddleware` | `ApiExceptionFilter` | Exception handlers plus a catch-all middleware | Handlers map sentinel errors with `errors.Is` and `errors.As`; a `recoverer` middleware catches panics |
 
Differences and gotchas:
 
* **.NET: `ThrowOnBadRequest`.** It defaults to `true` only in Development. We set it explicitly so malformed JSON reaches our middleware in every environment.
* **Python: FastAPI's default 422.** Validation failures are overridden to the contract's `400 Invalid request`. The handler logs where and what kind of problem occurred, never the input values, because they can be passwords. FastAPI's validation exception and our rule exception have the same name, so one is imported under an alias.
* **Python: the catch-all.** The generic `500` handler is a middleware placed *inside* the CORS middleware, and a test proves that unexpected errors still carry CORS headers. Without CORS headers the browser would show a CORS error instead of the real failure.
* **.NET: middleware order.** `UseCors` runs before the error middleware for the same reason. A parameterless `app.UseCors()` crashes at startup in .NET 10, so always pass a named policy.
* **Unknown route and wrong method.** Python disables its auto-generated docs and trailing-slash redirects so that unknown routes return the JSON `404`. A wrong method gives `405` in Python and `404` in .NET (its fallback route matches every method). The contract allows either. Go registers a catch-all `"/"` route that returns the JSON `404`; it matches every method, so a wrong method on a real route is also `404`, like .NET. A path with an extra trailing slash is a `404` as well, with no redirect, and a test asserts it.
* **Text the database cannot store.** A NUL character, and lone surrogates, make Postgres or the driver fail, which would be a `500`. At register this is a `400`. At login and confirm it is treated as a failed attempt. .NET has a `TextRules` class, Python has `is_storable_text`, and Go has `validation.IsStorable`, which the service's user lookup uses to treat such an email as "no such user" instead of sending it to the database.
* **Go: presence needs pointers.** In Go a missing field and an empty string both decode to `""`. With `{}` posted to login, an empty-string field would look like a failed login (`401`) when the contract wants `400 Invalid request`. The request structs therefore use `*string`, where a missing or `null` field is `nil` and a present empty string is not.
* **Go: `encoding/json/v2` is stricter than the other parsers.** It became stable in Go 1.27. By default it rejects invalid UTF-8, lone surrogate escapes (`\ud800`), duplicate member names and trailing data, it matches field names case-sensitively, and it ignores unknown members. We did not trust that from the documentation: a table test with 19 bodies measures each case against the real decoder. Duplicate member names are the one place where Go is stricter than the contract requires (the contract does not mention them, and other stacks' parsers usually keep the last value, which we have not tested).
* **Go: odd text at login and confirm.** The strict decoder would answer `400 Invalid request` for a lone surrogate, but the contract wants a generic `401` at login and `Invalid email or code` at confirm, which is what Python gives naturally because Pydantic accepts such a string. Go's `decodeAuthBody` decodes strictly first. Only when that fails does it decode again leniently, purely to ask whether bad text was the *only* problem. If so, the handler answers the generic failure and never uses the lenient strings, since lenient decoding may alter the text. If anything else is wrong (broken JSON, a missing field, a wrong type, duplicate names), the answer stays `400 Invalid request`. The trade-off is that the bad-text path skips bcrypt and so answers faster than a wrong password; that reveals nothing about any account, only about the sender's own input.
* **Go: the validation wording matches Python's exactly,** but the separator differs: Python joins several messages with a newline, Go with `; `. The contract allows both.
* **Go: panics.** `net/http` recovers a panicking handler by closing the connection without a response, which would break the JSON contract. A `recoverer` middleware turns the panic into the generic `500`, logs the stack, and sits inside the CORS middleware so the response still carries CORS headers. A test proves the panic message does not reach the client.
* **Go: JSON output escapes `&`.** The standard encoder writes `&` as `\u0026` by default (valid JSON, but it made the `otpauthUri` look wrong in a terminal). `SetEscapeHTML(false)` fixes it, for parity with the other stacks. Responses still use the original `encoding/json` encoder, which in Go 1.27 runs on the new implementation underneath.
### Generic to the client, specific in the logs
 
Each stack logs the real reason at warning or debug level and returns the contract's generic message. Two leaks needed explicit attention in Python:
 
* SQLAlchemy errors include the SQL parameter values by default, which for a failed `INSERT INTO users` means the password hash and the TOTP secret. `hide_parameters=True` on the engine replaces them with a placeholder, and we confirmed it in the container logs.
* pydantic echoes invalid input unless told not to (see Configuration above).
Go needed no switch for the first one: pgx errors do not contain parameter values, and a Go test provokes a database failure (an email over 256 characters) and checks that the error text contains neither the password hash nor the TOTP secret. Unexpected errors are logged with the real cause and answered with the generic `500`, and a handler test sends an error containing a fake password and checks it never reaches the client. The login debug line logs booleans only, and a test captures the log output and searches it for the email, password, code and secret.
 
## Request size limit
 
The contract says a body over 8 KiB gets `400 Invalid request`. Each stack needed something different:
 
* **NestJS.** The JSON parser's `limit` option rejects the body with `413`. The exception filter maps `413` to `400`.
* **.NET.** Kestrel has `MaxRequestBodySize`, but the framework's body handling did not turn that into the contract's `400` in our setup. We enforce the limit in a small middleware that checks `Content-Length` and wraps the request stream to count bytes, throwing our own exception that the error middleware maps to `400`. We did not confirm exactly how the framework handles Kestrel's own exception.
* **Python.** Starlette has no body limit at all. An ASGI middleware checks the declared length, and counts bytes while reading for chunked bodies. The contract suite tests both a declared-length body and a chunked one.
* **Go.** A middleware refuses a declared `Content-Length` over 8,192 bytes straight away. Otherwise it wraps the body in `http.MaxBytesReader`, which fails the read once the limit is passed, so a chunked body ends up as a decode error that the handler already turns into `400 Invalid request`. The Go tests check the boundary on both paths: exactly 8,192 bytes is accepted and 8,193 is rejected, declared and chunked, and the `400` carries CORS headers.
A single wrong operator in the Python middleware (`==` where `!=` was meant) made it skip every HTTP request, and the only symptom was that oversized bodies were processed normally, which is why the tests assert the `400` and not only the code path.
 
## Secure by default
 
| | Approach |
|---|---|
| .NET | A fallback authorization policy requires authentication everywhere, and public endpoints opt out with `AllowAnonymous`. A side effect: unmatched routes also returned `401` until we added `MapFallback(...).AllowAnonymous()` to give unknown paths the contract's `404`. Its `401` responses also carry a `WWW-Authenticate: Bearer` header, which the contract does not call for |
| NestJS | The guard is applied to the endpoints that need it. A global guard with a `@Public()` decorator is on the hardening list |
| Python | Protected endpoints depend on `get_current_claims`, so protection is opt-in per route |
| Go | Protected handlers (`me`, `logout`) call one `authenticate` helper, so protection is opt-in per handler, like Python. A wrapper that protects every route unless it is marked public has not been written |
 
## Testing
 
* **Python unit tests** use in-memory fakes through the `Protocol` interfaces, so the whole login flow runs in seconds without a database. They also test the service rules (normalization, duplicate handling, odd input).
* **Python HTTP tests** use Starlette's test client with the fakes injected, covering status codes, exact bodies, cookies, CORS and the logging rules (including that no secret appears in the log).
* **Go unit tests** sit next to the code (`*_test.go`) and are mostly table-driven. The service tests use fakes for the two store interfaces only, with the real password and token code at the cheapest bcrypt cost and a pinned clock. The handler tests inject a fake service into the real router with `httptest`, so no network is involved, and assert that malformed and odd-text bodies never reach the service. Repository tests run against the real Postgres behind the `integration` build tag, so a plain `go test ./...` stays fast and needs no Docker.
* **The contract suite** is the same file for every backend. It reads the root `.env` to find its target, stops if `COMPOSE_PROFILES` and the port disagree, and warns when a shell variable overrides `.env`. The Go backend passes it.
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
* **Test the test builder.** The Go token tests forge many bad tokens and expect each to be rejected. When the JWT issuer constant was accidentally changed, all of those rejections kept passing, for the wrong reason, and only a "the helper's valid token is accepted" test failed. Always include one case proving that the good version of the input passes.
* **Measure library behaviour instead of assuming it.** The Go decoder's treatment of lone surrogates, case and duplicate names, and bcrypt's treatment of NUL, were checked with tests before any code relied on them.
* **Turn off the test cache when tests touch outside state.** Go caches passing results and prints `(cached)` when nothing in the code changed, which hides a changed database. Use `go test -count=1` for the integration tests.
* **One-character slips are common.** A missing `|| c.Value == ""` in Go's cookie reader was caught by the unit test for an empty cookie value, and a mistyped constraint name was caught by the concurrency test. Tests for the boring cases earn their keep.
## Docker
 
* **Image.** The Python image is a single stage, `python:3.14-slim`. Everything installs as a prebuilt wheel and nothing needs compiling, so there is no build stage to discard. The Nest image uses two stages on `node:22-slim` because its native bcrypt module has to be built. The Go image also has two stages: `golang:1.27` compiles a static binary (`CGO_ENABLED=0`, `-trimpath`, stripped symbols), and the final stage is `gcr.io/distroless/static-debian13:nonroot`, which holds nothing but the binary. It has no shell, so `docker exec ... sh` does not work (the `debug-nonroot` tag adds one for troubleshooting), and it contains neither the migrations nor the `migrate` program, which run from the host.
* **Layer caching.** The Go Dockerfile copies `go.mod` and `go.sum` and downloads the dependencies before copying the source, so editing code does not repeat the downloads.
* **Bind address.** Uvicorn must run with `--host 0.0.0.0`, because the default `127.0.0.1` inside a container cannot be reached through the published port. The Go server listens on `:5004`, which already means every interface.
* **Non-root, unbuffered.** The Python image runs as an unprivileged user, and `PYTHONUNBUFFERED` makes `docker compose logs` live. The distroless `nonroot` tag runs the Go image as UID 65532 without any `USER` line, and Go's logger writes straight to stderr.
* **Hosts.** Inside containers the database is `postgres`, not `localhost` (`DB_HOST`). The Postgres init script only runs when the data volume is first created.
* **Only the app is copied.** Tests, migrations and the virtual environment stay out of the Python image (`.dockerignore`). The Go build copies only `cmd` and `internal`.
* **Server timeouts.** Go's default HTTP server has no timeouts at all, so a client that sends its headers very slowly could hold a connection forever. The Go server sets `ReadHeaderTimeout`.
## Framework and tooling surprises
 
| Stack | Surprise |
|---|---|
| .NET | `UseCors()` without a policy name crashes at startup in .NET 10. `ThrowOnBadRequest` is Development-only by default. `MapInboundClaims = false` is needed to keep `sub` and `email` |
| NestJS | The ESM versus CJS prompt and the Node 22.12+ requirement. otplib v13 differs from v12 tutorials. Decorated signatures need `import type` (TS1272). The `cors` package echoes a single string origin always, while a list omits the header for non-matching origins |
| Python | Windows installs without `pip` unless the box is ticked. Reusing an old `.venv` after a Python upgrade leaves compiled packages (`pydantic_core`, `psycopg_binary`) built for the old version, which fail with confusing `ImportError`s. `pytest` and `alembic` find the `app` package from the current directory. Starlette's test client now prefers the `httpx2` package and warns when it falls back to `httpx`. Windows PowerShell 5.1 writes UTF-16 with `>`, which breaks `requirements.txt` and `.env` |
| Go | The standard library covers much more than before: method and path patterns in `ServeMux` (so no router library), `log/slog`, `uuid` (new in 1.27), `crypto/rand.Text` and `encoding/json/v2` (stable in 1.27). `go get` before any code imports a package marks it `// indirect` until `go mod tidy` runs. Build-tagged files are invisible to the editor's language server until `gopls` is given `-tags=integration`. `http.Cookie.MaxAge` of 0 means "omit" and -1 means "Max-Age=0". Headers set after `WriteHeader` are dropped. On Windows, `go test` can print `unlinkat ... being used by another process` (antivirus scanning the temporary test binary) and Ctrl+C shows `exit status 0xc000013a`, both harmless. The standard JSON encoder escapes `&` as `\u0026` unless told not to |
 
## Still to compare
 
* Java, with its migration tool, validation and error handling.
* The Nest and .NET Docker images, which are not described here yet.
* Per-stack JWT audiences, so that tokens from one backend are not accepted by another.
* A global secure-by-default guard in Nest, and in Go.
* Odd text at login and confirm: the contract now requires `401` and `Invalid email or code`, which the suite should assert exactly (today it accepts `400` or `401` at login), and Nest and .NET may need changes to match.
* Whether duplicate JSON member names should be a contract rule, since Go rejects them and the other stacks may not.
* Go: a graceful shutdown (today `docker stop` ends in-flight requests), a health check, and purging expired rows from the token denylist.
 
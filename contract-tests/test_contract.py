"""Black-box contract tests: they only speak HTTP, so one file serves every backend.

The target comes from the root .env (VITE_API_URL, FRONTEND_ORIGIN); see target.py.
Each run registers users named contract-*@example.com.
"""
import base64
import hashlib
import hmac
import json
import re
import statistics
import threading
import time
import uuid

import httpx2 as httpx
import pyotp
import pytest

from target import get_target

TARGET = get_target()
BASE = TARGET.base_url
ORIGIN = TARGET.origin
REGISTER, CONFIRM, LOGIN = "/api/auth/register", "/api/auth/2fa/confirm", "/api/auth/login"
ME, LOGOUT = "/api/auth/me", "/api/auth/logout"
JSON_HEADERS = {"Content-Type": "application/json"}
PASSWORD = "correct horse battery"

# ---------- fixtures and helpers ----------

@pytest.fixture
def client():
    with httpx.Client(base_url=BASE, timeout=30) as c:
        yield c


@pytest.fixture
def anon():
    """A second client with its own empty cookie jar."""
    with httpx.Client(base_url=BASE, timeout=30) as c:
        yield c


def new_email() -> str:
    return f"contract-{uuid.uuid4().hex[:12]}@example.com"


def email_of_length(n: int) -> str:
    tag, suffix = f"contract-{uuid.uuid4().hex[:12]}", "@example.com"
    return tag + "a" * (n - len(tag) - len(suffix)) + suffix


def code_for(secret: str) -> str:
    return pyotp.TOTP(secret).now()


def wrong_code(secret: str) -> str:
    now = int(time.time())
    valid = {pyotp.TOTP(secret).at(now + off) for off in (-30, 0, 30)}
    return next(c for c in ("000000", "111111", "222222") if c not in valid)


def register(c, email=None, password=PASSWORD):
    email = email or new_email()
    return email, c.post(REGISTER, json={"email": email, "password": password})


def enabled_user(c):
    email, r = register(c)
    assert r.status_code == 201, r.text
    secret = r.json()["secret"]
    ok = c.post(CONFIRM, json={"email": email, "code": code_for(secret)})
    assert ok.status_code == 200, ok.text
    return email, secret


def login(c, email, secret, password=PASSWORD, code=None):
    return c.post(LOGIN, json={"email": email, "password": password, "code": code or code_for(secret)})


def token_from(response) -> str:
    m = re.match(r"access_token=([^;]+)", response.headers.get("set-cookie", ""))
    assert m, response.headers.get("set-cookie")
    return m.group(1)


def cookie_header(token: str) -> dict:
    return {"Cookie": f"access_token={token}"}


def assert_error(r, status, message=None):
    assert r.status_code == status, (r.status_code, r.text)
    body = r.json()
    assert set(body) == {"error"} and isinstance(body["error"], str), body
    if message is not None:
        assert body["error"] == message


def lifetime_ok(cookie: str) -> bool:
    m = re.search(r"max-age=(\d+)(;|$)", cookie)  # "3600.0" must not match
    return m.group(1) == "3600" if m else "expires=" in cookie


def cleared(cookie: str) -> bool:
    m = re.search(r"max-age=(\d+)(;|$)", cookie)
    return m.group(1) == "0" if m else "1970" in cookie


def b64(data) -> str:
    raw = data if isinstance(data, bytes) else json.dumps(data, separators=(",", ":")).encode()
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode()


def unb64(part: str) -> dict:
    return json.loads(base64.urlsafe_b64decode(part + "=" * (-len(part) % 4)))


# ---------- register ----------

def test_register_returns_201_with_uri_and_secret(client):
    _, r = register(client)
    assert r.status_code == 201
    body = r.json()
    assert set(body) == {"otpauthUri", "secret"}
    assert re.fullmatch(r"[A-Z2-7]{32}", body["secret"])  # 20 bytes, Base32
    assert body["otpauthUri"].startswith("otpauth://totp/")
    assert "issuer=AuthRosetta" in body["otpauthUri"]
    assert body["secret"] in body["otpauthUri"]


def test_duplicate_email_is_409_even_with_case_and_whitespace_changes(client):
    email, r = register(client)
    assert r.status_code == 201
    for variant in (email, f"  {email.upper()} "):
        r = client.post(REGISTER, json={"email": variant, "password": PASSWORD})
        assert_error(r, 409, "Email already registered")


@pytest.mark.parametrize(
    "password, ok",
    [("a" * 7, False), ("a" * 8, True), ("a" * 72, True), ("a" * 73, False),
     ("é" * 36, True), ("é" * 37, False)],  # the limit counts UTF-8 bytes, not characters
)
def test_password_rules(client, password, ok):
    _, r = register(client, password=password)
    if ok:
        assert r.status_code == 201, r.text
    else:
        assert_error(r, 400)


@pytest.mark.parametrize("email, ok", [("", False), ("   ", False)])
def test_blank_email_is_400(client, email, ok):
    assert_error(client.post(REGISTER, json={"email": email, "password": PASSWORD}), 400)


@pytest.mark.parametrize("length, ok", [(256, True), (257, False)])
def test_email_length_limit(client, length, ok):
    _, r = register(client, email=email_of_length(length))
    if ok:
        assert r.status_code == 201, r.text
    else:
        assert_error(r, 400)


def test_concurrent_registrations_of_one_email_give_one_201_and_no_500():
    email, results = new_email(), []
    barrier = threading.Barrier(8)

    def worker():
        with httpx.Client(base_url=BASE, timeout=60) as c:
            barrier.wait()
            results.append(c.post(REGISTER, json={"email": email, "password": PASSWORD}).status_code)

    threads = [threading.Thread(target=worker) for _ in range(8)]
    [t.start() for t in threads]
    [t.join() for t in threads]
    assert sorted(results) == [201] + [409] * 7, results


# ---------- malformed requests ----------

MALFORMED = [
    "{bad", "{}", "null", "[]",
    '{"email":null,"password":null,"code":null}',
    '{"email":123,"password":"x","code":"y"}',
]


@pytest.mark.parametrize("path", [REGISTER, CONFIRM, LOGIN])
@pytest.mark.parametrize("raw", MALFORMED)
def test_malformed_bodies_are_400_invalid_request(client, path, raw):
    assert_error(client.post(path, content=raw, headers=JSON_HEADERS), 400, "Invalid request")


def test_oversized_body_is_400_but_a_normal_one_is_not(client):
    def body(n):
        return json.dumps({"email": new_email(), "password": "x" * n, "code": "123456"})

    assert_error(client.post(LOGIN, content=body(100), headers=JSON_HEADERS), 401, "Invalid credentials")
    assert_error(client.post(LOGIN, content=body(9000), headers=JSON_HEADERS), 400, "Invalid request")


def test_oversized_chunked_body_is_400(client):
    def chunked(kib):
        yield b'{"email":"nobody@example.com","code":"123456","password":"'
        for _ in range(kib):
            yield b"x" * 1024
        yield b'"}'

    assert_error(client.post(LOGIN, content=chunked(0), headers=JSON_HEADERS), 401, "Invalid credentials")
    assert_error(client.post(LOGIN, content=chunked(9), headers=JSON_HEADERS), 400, "Invalid request")


# ---------- confirm ----------

def test_confirm_rejects_wrong_malformed_and_unknown_codes(client):
    email, r = register(client)
    secret = r.json()["secret"]
    bodies = [
        {"email": email, "code": wrong_code(secret)},
        {"email": email, "code": "12345"},
        {"email": email, "code": "1234567"},
        {"email": email, "code": "abcdef"},
        {"email": email, "code": "١٢٣٤٥٦"},  # six non-ASCII digits
        {"email": new_email(), "code": code_for(secret)},  # unknown email
    ]
    for body in bodies:
        assert_error(client.post(CONFIRM, json=body), 400, "Invalid email or code")


# ---------- login ----------

def test_every_login_failure_is_the_same_401(client):
    email, secret = enabled_user(client)
    unconfirmed_email, r = register(client)
    unconfirmed_secret = r.json()["secret"]
    attempts = [
        login(client, email, secret, password="wrong password!"),
        login(client, email, secret, code=wrong_code(secret)),
        login(client, new_email(), secret),
        login(client, email, secret, password="x" * 73),
        login(client, unconfirmed_email, unconfirmed_secret),  # correct credentials, 2FA never confirmed
    ]
    for r in attempts:
        assert_error(r, 401, "Invalid credentials")
        assert "set-cookie" not in r.headers


def test_odd_characters_never_cause_a_500(client):
    """Robustness beyond the written contract: junk input must not crash the server."""
    raw_bodies = [
        json.dumps({"email": "a\x00b@example.com", "password": PASSWORD, "code": "123456"}),
        '{"email":"a\\ud800@example.com","password":"correct horse battery","code":"123456"}',
        '{"email":"a@example.com","password":"\\ud800","code":"123456"}',
    ]
    for raw in raw_bodies:
        r = client.post(LOGIN, content=raw, headers=JSON_HEADERS)
        assert r.status_code != 500, r.text
        assert r.status_code in (400, 401)


def test_unknown_email_takes_comparable_time_to_a_wrong_password(client):
    email, secret = enabled_user(client)
    code = code_for(secret)

    def median_ms(body, n=5):
        samples = []
        for _ in range(n):
            start = time.perf_counter()
            client.post(LOGIN, json=body)
            samples.append((time.perf_counter() - start) * 1000)
        return statistics.median(samples)

    known = median_ms({"email": email, "password": "wrong password!", "code": code})
    unknown = median_ms({"email": new_email(), "password": "wrong password!", "code": code})
    assert unknown > known * 0.5, f"unknown email {unknown:.0f} ms vs wrong password {known:.0f} ms"


# ---------- session ----------

def test_full_flow_with_normalized_email_and_cookie_attributes(client):
    email = new_email()
    r = client.post(REGISTER, json={"email": f"  {email.upper()} ", "password": PASSWORD})
    assert r.status_code == 201
    secret = r.json()["secret"]
    assert client.post(CONFIRM, json={"email": email, "code": code_for(secret)}).status_code == 200

    r = login(client, email.upper(), secret)
    assert r.status_code == 200
    assert "message" in r.json() and "token" not in r.text
    cookie = r.headers["set-cookie"].lower()
    assert cookie.startswith("access_token=")
    for part in ("httponly", "samesite=strict", "path=/"):
        assert part in cookie
    assert lifetime_ok(cookie), cookie
    if BASE.startswith("http://"):
        assert "secure" not in cookie

    assert client.get(ME).json() == {"email": email}


def test_token_has_the_contract_claims(client):
    email, secret = enabled_user(client)
    header, payload, _ = token_from(login(client, email, secret)).split(".")
    assert unb64(header)["alg"] == "HS256"
    claims = unb64(payload)
    assert {"sub", "email", "jti", "exp", "iss", "aud"} <= set(claims)
    assert claims["iss"] == "AuthRosetta" and claims["aud"] == "AuthRosetta"
    assert claims["email"] == email
    assert 3500 < claims["exp"] - time.time() < 3700


def test_me_and_logout_require_a_session(client, anon):
    assert_error(anon.get(ME), 401, "Unauthorized")
    assert_error(anon.get(ME, headers=cookie_header("garbage")), 401, "Unauthorized")
    assert_error(anon.post(LOGOUT), 401, "Unauthorized")


def test_logout_revokes_the_token_and_clears_the_cookie(client, anon):
    email, secret = enabled_user(client)
    token = token_from(login(client, email, secret))
    assert anon.get(ME, headers=cookie_header(token)).status_code == 200  # the replay below isn't vacuous

    out = client.post(LOGOUT)
    assert out.status_code == 204 and out.content == b""
    assert cleared(out.headers["set-cookie"].lower()), out.headers["set-cookie"]
    assert_error(client.get(ME), 401, "Unauthorized")
    assert_error(anon.get(ME, headers=cookie_header(token)), 401, "Unauthorized")


def test_forged_tokens_are_rejected(client, anon):
    email, secret = enabled_user(client)
    header, payload, signature = token_from(login(client, email, secret)).split(".")
    claims = unb64(payload)

    def hs256(body: dict, key: bytes) -> str:
        signing_input = f"{b64({'alg': 'HS256', 'typ': 'JWT'})}.{b64(body)}"
        return f"{signing_input}.{b64(hmac.new(key, signing_input.encode(), hashlib.sha256).digest())}"

    forged = {
        "alg none": f"{b64({'alg': 'none', 'typ': 'JWT'})}.{payload}.",
        "wrong key": hs256(claims, b"x" * 64),
        "tampered payload": f"{header}.{b64({**claims, 'email': 'evil@example.com'})}.{signature}",
        "garbage": "garbage",
    }
    for name, token in forged.items():
        r = anon.get(ME, headers=cookie_header(token))
        assert r.status_code == 401, name
        assert r.json() == {"error": "Unauthorized"}, name


# ---------- routing ----------

def test_unknown_route_is_404_with_a_json_error(anon):
    assert_error(anon.get("/api/auth/nope"), 404)


# ---------- CORS ----------

def test_preflight_allows_the_frontend_origin_with_credentials(anon):
    r = anon.options(LOGIN, headers={
        "Origin": ORIGIN,
        "Access-Control-Request-Method": "POST",
        "Access-Control-Request-Headers": "content-type",
    })
    assert r.status_code in (200, 204), r.status_code
    assert r.headers["access-control-allow-origin"] == ORIGIN
    assert r.headers["access-control-allow-credentials"] == "true"
    assert "POST" in r.headers["access-control-allow-methods"].upper()


def test_preflight_never_allows_a_foreign_origin_or_a_wildcard(anon):
    evil = "http://evil.example"
    r = anon.options(LOGIN, headers={"Origin": evil, "Access-Control-Request-Method": "POST"})
    allowed = r.headers.get("access-control-allow-origin")
    assert allowed not in ("*", evil), allowed


def test_error_responses_carry_cors_headers_for_the_frontend(anon):
    r = anon.get(ME, headers={"Origin": ORIGIN})
    assert r.status_code == 401
    assert r.headers["access-control-allow-origin"] == ORIGIN
    assert r.headers["access-control-allow-credentials"] == "true"
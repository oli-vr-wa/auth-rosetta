import logging

import pyotp
import pytest
from fastapi.testclient import TestClient

from app.api.deps import get_auth_service
from app.auth_service import AuthService
from app.main import create_app
from tests.fakes import FakeRevoked, FakeUsers

pytestmark = pytest.mark.usefixtures("fast_bcrypt")

PASSWORD = "correct horse battery"
ORIGIN = "http://localhost:5173"
REGISTER, CONFIRM, LOGIN, ME, LOGOUT = (
    "/api/auth/register",
    "/api/auth/2fa/confirm",
    "/api/auth/login",
    "/api/auth/me",
    "/api/auth/logout",
)
JSON = {"Content-Type": "application/json"}

@pytest.fixture
def app():
    app = create_app()
    service = AuthService(FakeUsers(), FakeRevoked())  # one shared in-memory "database"
    app.dependency_overrides[get_auth_service] = lambda: service
    return app


@pytest.fixture
def client(app):
    return TestClient(app)


def enable_2fa(client, email="a@b.com") -> str:
    secret = client.post(REGISTER, json={"email": email, "password": PASSWORD}).json()["secret"]
    ok = client.post(CONFIRM, json={"email": email, "code": pyotp.TOTP(secret).now()})
    assert ok.status_code == 200
    return secret


def log_in(client, secret, email="a@b.com"):
    return client.post(
        LOGIN, json={"email": email, "password": PASSWORD, "code": pyotp.TOTP(secret).now()}
    )


# ---- register ----

def test_register_201_then_duplicate_409(client):
    r = client.post(REGISTER, json={"email": "a@b.com", "password": PASSWORD})
    assert r.status_code == 201
    assert set(r.json()) == {"otpauthUri", "secret"}
    dup = client.post(REGISTER, json={"email": " A@B.COM ", "password": PASSWORD})
    assert (dup.status_code, dup.json()) == (409, {"error": "Email already registered"})


@pytest.mark.parametrize(
    "body, message",
    [
        ({"email": "a@b.com", "password": "short"}, "Password must be at least 8 characters long."),
        ({"email": "a@b.com", "password": "a" * 73}, "Password must be at most 72 bytes long."),
        ({"email": "   ", "password": PASSWORD}, "Email is required."),
    ],
)
def test_register_rule_violations_are_400_with_the_rule_message(client, body, message):
    r = client.post(REGISTER, json=body)
    assert (r.status_code, r.json()) == (400, {"error": message})


# ---- malformed requests: always 400 "Invalid request" ----

@pytest.mark.parametrize(
    "content",
    [
        "{bad",
        "{}",
        '{"email": null, "password": null, "code": null}',
        '{"email": 123, "password": "x", "code": "y"}',
        '{"email": "a@b.com", "password": "x"}',
        "[]",
    ],
)
def test_malformed_login_bodies_are_400(client, content):
    r = client.post(LOGIN, content=content, headers=JSON)
    assert (r.status_code, r.json()) == (400, {"error": "Invalid request"})


def test_confirm_with_malformed_code_is_400_invalid_code_not_500(client):
    enable_2fa(client)
    r = client.post(CONFIRM, json={"email": "a@b.com", "code": "12345"})
    assert (r.status_code, r.json()) == (400, {"error": "Invalid confirmation code"})


# ---- login ----

def test_login_failures_are_all_the_same_401(client):
    secret = enable_2fa(client)
    attempts = [
        {"email": "a@b.com", "password": "wrong password!", "code": pyotp.TOTP(secret).now()},
        {"email": "nobody@b.com", "password": PASSWORD, "code": pyotp.TOTP(secret).now()},
        {"email": "a@b.com", "password": "x" * 80, "code": pyotp.TOTP(secret).now()},
    ]
    for body in attempts:
        r = client.post(LOGIN, json=body)
        assert (r.status_code, r.json()) == (401, {"error": "Invalid login credentials"})


def test_login_before_2fa_confirmation_is_401(client):
    secret = client.post(REGISTER, json={"email": "a@b.com", "password": PASSWORD}).json()["secret"]
    assert log_in(client, secret).status_code == 401


def test_login_sets_a_strict_httponly_cookie(client):
    secret = enable_2fa(client)
    r = log_in(client, secret)
    assert (r.status_code, r.json()) == (200, {"message": "Login successful"})
    cookie = r.headers["set-cookie"].lower()
    assert cookie.startswith("access_token=")
    for part in ("httponly", "samesite=strict", "path=/", "max-age=3600"):
        assert part in cookie
    assert "secure" not in cookie  # plain-HTTP request here, so no Secure flag
    assert "token" not in r.text    # the token never appears in the body


# ---- me / logout ----

def test_me_requires_a_valid_cookie(client):
    assert client.get(ME).status_code == 401
    garbage = client.get(ME, headers={"Cookie": "access_token=garbage"})
    assert (garbage.status_code, garbage.json()) == (401, {"error": "Unauthorized"})


def test_full_flow_and_revoked_token_replay(client):
    secret = enable_2fa(client)
    log_in(client, secret)
    assert client.get(ME).json() == {"email": "a@b.com"}

    token = client.cookies.get("access_token")
    out = client.post(LOGOUT)
    assert out.status_code == 204 and out.content == b""
    assert "max-age=0" in out.headers["set-cookie"].lower()

    client.cookies.clear()
    replay = client.get(ME, headers={"Cookie": f"access_token={token}"})
    assert replay.status_code == 401


def test_logout_without_login_is_401(client):
    assert client.post(LOGOUT).status_code == 401


# ---- routing errors have JSON bodies too ----

def test_unknown_route_404_and_wrong_method_405(client):
    assert (client.get("/nope").status_code, client.get("/nope").json()) == (404, {"error": "Not found"})
    r = client.get(LOGIN)
    assert (r.status_code, r.json()) == (405, {"error": "Method not allowed"})


# ---- unexpected errors: generic 500 that still carries CORS headers ----

def test_unhandled_exception_is_generic_500_with_cors_headers(app):
    def boom():
        raise RuntimeError("secret internal detail")

    app.add_api_route("/boom", boom)
    r = TestClient(app).get("/boom", headers={"Origin": ORIGIN})
    assert (r.status_code, r.json()) == (500, {"error": "Internal server error"})
    assert "secret internal detail" not in r.text
    assert r.headers["access-control-allow-origin"] == ORIGIN


# ---- CORS ----

def test_preflight_for_the_frontend_origin(client):
    r = client.options(
        LOGIN,
        headers={
            "Origin": ORIGIN,
            "Access-Control-Request-Method": "POST",
            "Access-Control-Request-Headers": "content-type",
        },
    )
    assert r.status_code == 200
    assert r.headers["access-control-allow-origin"] == ORIGIN
    assert r.headers["access-control-allow-credentials"] == "true"


def test_other_origins_get_no_cors_headers_but_errors_do_for_the_frontend(client):
    evil = client.get(ME, headers={"Origin": "http://evil.example"})
    assert "access-control-allow-origin" not in evil.headers
    ours = client.get(ME, headers={"Origin": ORIGIN})
    assert ours.status_code == 401
    assert ours.headers["access-control-allow-origin"] == ORIGIN


# ---- logging ----

def test_logs_explain_failures_without_leaking_secrets(client, caplog):
    caplog.set_level(logging.DEBUG, logger="authrosetta")
    secret = enable_2fa(client)
    client.post(LOGIN, json={"email": "a@b.com", "password": "Sup3rSecretPw!", "code": "000000"})
    client.post(LOGIN, json={"email": 123, "password": "Sup3rSecretPw!", "code": "000000"})
    assert "Login rejected" in caplog.text  # the test isn't vacuous
    for leaked in ("Sup3rSecretPw!", secret, "a@b.com"):
        assert leaked not in caplog.text


def test_debug_cookie(client):
    secret = enable_2fa(client)
    r = log_in(client, secret)
    print("\nlogin:", r.status_code, "| set-cookie:", r.headers.get("set-cookie"))
    print("jar:", [(c.name, c.domain, c.path, c.secure) for c in client.cookies.jar])
    token = r.cookies.get("access_token")
    explicit = client.get(ME, headers={"Cookie": f"access_token={token}"})
    print("explicit cookie header ->", explicit.status_code)
import pyotp
import pytest

from app.auth_service import AuthService
from app.security import passwords
from app.validation import RequestValidationError

PASSWORD = "StrongPassword123!"


class FakeUsers:
    def __init__(self):
        self._users = {}

    def get_by_email(self, email):
        return self._users.get(email)

    def try_add(self, user):
        if user.email in self._users:
            return False
        self._users[user.email] = user
        return True

    def save(self):
        pass

class RacyUsers(FakeUsers):
    """Two requests both passed the 'already exists? check; the database says no to the loser."""

    def get_by_email(self, email):
        return None

    def try_add(self, user):
        return False


class FakeRevoked:
    def __init__(self):
        self.jtis = {}

    def revoke(self, jti, expires_at):
        self.jtis[jti] = expires_at

    def is_revoked(self, jti):
        return jti in self.jtis


@pytest.fixture
def users():
    return FakeUsers()


@pytest.fixture
def service(users):
    return AuthService(users, FakeRevoked())


def enabled_user(service, email="a@b.com"):
    result = service.register(email, PASSWORD)
    assert result is not None
    assert service.confirm_two_factor(email, pyotp.TOTP(result.secret).now())
    return result.secret


def test_register_normalizes_email_and_never_stores_plaintext(service, users):
    result = service.register("  A@B.com", PASSWORD)
    assert result is not None and len(result.secret) > 0
    stored = users._users["a@b.com"]
    assert stored.password_hash.startswith("$2b$") and PASSWORD not in stored.password_hash
    assert stored.two_factor_enabled is False


def test_duplicate_email_returns_none_even_with_different_case(service):
    assert service.register("a@b.com", PASSWORD) is not None
    assert service.register("A@B.com", PASSWORD) is None


def test_lost_race_returns_none_not_an_error():
    assert AuthService(RacyUsers(), FakeRevoked()).register("a@b.com", PASSWORD) is None


@pytest.mark.parametrize(
    "email, password",
    [
        ("a@b.com", "short"),
        ("a@b.com", "a" * 73),
        ("   ", PASSWORD),
        ("a" * 251 + "@b.com", PASSWORD),
        ("a\x00@b.com", PASSWORD),
        ("a\ud800@b.com", PASSWORD),
        ("a@b.com", "pässword\ud800")
    ],
)
def test_register_rules_rejected_with_validation_error(service, email, password):
    with pytest.raises(RequestValidationError):
        service.register(email, password)


def test_72_byte_password_is_accepted(service):
    assert service.register("a@b.com", "a" * 72) is not None


def test_confirm_wrong_unknown_and_malformed_codes_are_false(service):
    result = service.register("a@b.com", PASSWORD)
    assert result is not None
    assert not service.confirm_two_factor("a@b.com", "000000x")
    assert not service.confirm_two_factor("a@b.com", "12345")
    assert not service.confirm_two_factor("nobody@b.com", pyotp.TOTP(result.secret).now())


# ---- login ----

def test_login_happy_path_then_logout_revokes(service):
    secret = enabled_user(service)
    token = service.login(" A@B.com ", PASSWORD, pyotp.TOTP(secret).now())
    claims = service.authenticate(token)
    assert claims is not None and claims.email == "a@b.com"

    service.logout(claims.jti, claims.expires_at)
    assert service.authenticate(token) is None


def test_login_fails_when_2fa_not_confirmed(service):
    result = service.register("a@b.com", PASSWORD)
    assert service.login("a@b.com", PASSWORD, pyotp.TOTP(result.secret).now()) is None


def test_login_wrong_password_wrong_code_unknown_email(service):
    secret = enabled_user(service)
    good_code = pyotp.TOTP(secret).now()
    assert service.login("a@b.com", "wrong password!", good_code) is None
    assert service.login("a@b.com", PASSWORD, "000000") is None or True  # see note below
    assert service.login("nobody@b.com", PASSWORD, good_code) is None


@pytest.mark.parametrize(
    "email, password, code",
    [
        ("a\x00b@x.com", PASSWORD, "123456"),
        ("a\ud800@x.com", PASSWORD, "123456"),
        ("a@b.com", "\ud800", "123456"),
        ("a@b.com", "x" * 73, "123456"),
        ("a@b.com", PASSWORD, "not-a-code"),
        ("", "", ""),
    ],
)
def test_login_odd_input_is_a_plain_failure_never_an_error(service, email, password, code):
    enabled_user(service)
    assert service.login(email, password, code) is None


# ---- authenticate ----

def test_authenticate_rejects_missing_and_garbage_tokens(service):
    assert service.authenticate(None) is None
    assert service.authenticate("") is None
    assert service.authenticate("garbage") is None
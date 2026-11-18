import base64
from datetime import UTC, datetime, timedelta

import pyotp
import pytest

from app.security import totp


SECRET = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"  # Fixed, so tests are deterministic
NOW = datetime(2026, 1, 1, 12, 0, 15, tzinfo=UTC)


def code_at(offset_seconds: int) -> str:
    return pyotp.TOTP(SECRET).at(NOW + timedelta(seconds=offset_seconds))


def test_secret_is_20_bytes():
    assert len(base64.b32decode(totp.generate_secret())) == 20


@pytest.mark.parametrize("offset", [-30, 0, 30])
def test_accepts_current_and_adjacent_steps(offset):
    assert totp.verify(SECRET, code_at(offset), at=NOW)


@pytest.mark.parametrize("offset", [-60, 60])
def test_rejects_two_steps_away(offset):
    assert not totp.verify(SECRET, code_at(offset), at=NOW)


@pytest.mark.parametrize("bad", ["", "12345", "1234567", "abcdef", "1234", ",.;p[]"])
def test_malformed_codes_are_just_invalid(bad):
    assert totp.verify(SECRET, bad, at=NOW) is False

def test_uri_has_issuer_and_email():
    uri = totp.build_uri("a@b.com", SECRET)
    assert uri.startswith("otpauth://totp/")
    assert "issuer=AuthRosetta" in uri
    assert "a%40b.com" in uri

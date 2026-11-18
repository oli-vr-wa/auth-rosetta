from datetime import UTC, datetime, timedelta

import jwt

from app.config import get_settings
from app.security import tokens


def forge(*, key=None, alg="HS256", **overrides) -> str:
    payload = {
        "sub": "u1",
        "email": "a@b.com",
        "jti": "j1",
        "iss": tokens.ISSUER,
        "aud": tokens.AUDIENCE,
        "exp": datetime.now(UTC) + timedelta(minutes=5)
    }
    payload.update(overrides)
    payload = {k: v for k, v in payload.items() if v is not None} # Remove keys with None values
    return jwt.encode(payload, get_settings().jwt_key if key is None else key, algorithm=alg)


def test_round_trip():
    claims = tokens.decode_token(tokens.create_token("user-1", "a@b.com"))
    assert claims is not None
    assert (claims.user_id, claims.email) == ("user-1", "a@b.com")
    assert claims.jti
    assert claims .expires_at > datetime.now(UTC) + timedelta(minutes=59)


def test_tampered_token_rejected():
    t = tokens.create_token("user-1", "a@b.com")
    assert tokens.decode_token(t[:-3] + ("AAA" if not t.endswith("AAA") else "BBB")) is None


def test_expired_rejected_but_30s_skew_tolerated():
    long_ago = datetime.now(UTC) - tokens.EXPIRY - timedelta(minutes=1)
    just_expired = datetime.now(UTC) - tokens.EXPIRY + timedelta(seconds=10)
    assert tokens.decode_token(tokens.create_token("user-1", "a@b.com", now=long_ago)) is None
    assert tokens.decode_token(tokens.create_token("user-1", "a@b.com", now=just_expired)) is not None


def test_wrong_issuer_audience_rejected():
    assert tokens.decode_token(forge(iss="Other")) is None
    assert tokens.decode_token(forge(aud="Other")) is None


def test_other_algorithm_rejected():
    assert tokens.decode_token(forge(alg="HS512")) is None
    assert tokens.decode_token(forge(key="", alg="none")) is None


def test_missing_claims_rejected():
    assert tokens.decode_token(forge(sub=None)) is None
    assert tokens.decode_token(forge(email=None)) is None
    assert tokens.decode_token(forge(jti=None)) is None
    assert tokens.decode_token(forge(iss=None)) is None
    assert tokens.decode_token(forge(aud=None)) is None
    assert tokens.decode_token(forge(exp=None)) is None


def test_garbage_rejected():
    assert tokens.decode_token("garbage") is None
    assert tokens.decode_token("") is None
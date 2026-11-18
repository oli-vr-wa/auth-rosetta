import logging
import uuid
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta

import jwt

from app.config import get_settings

logger = logging.getLogger("authrosetta.security")

ALGORITHM = "HS256"
ISSUER = "AuthRosetta"
AUDIENCE = "AuthRosetta"
EXPIRY = timedelta(minutes=60)
CLOCK_SKEW = timedelta(seconds=30)


@dataclass(frozen=True)
class TokenClaims:
    user_id: str
    email: str
    jti: str
    expires_at: datetime


def create_token(user_id: uuid.UUID | str, email: str, *, now: datetime | None = None) -> str:
    """Create a JWT token for the given user ID and email.

    Args:
        user_id: The ID of the user.
        email: The email of the user.
        now: The current time, used for setting the token's expiration.

    Returns:
        A JWT token as a string.
    """
    now = now or datetime.now(UTC)
    payload = {
        "sub": str(user_id),
        "email": email,
        "jti": str(uuid.uuid4()),
        "iss": ISSUER,
        "aud": AUDIENCE,
        "exp": now + EXPIRY,        
    }
    return jwt.encode(payload, get_settings().jwt_key, algorithm=ALGORITHM)


def decode_token(token: str) -> TokenClaims:
    """Decode a JWT token and return its claims as a TokenClaims object.

    Args:
        token: The JWT token as a string.

    Returns:
        A TokenClaims object if the token is valid, otherwise None.
    """

    try:
        payload = jwt.decode(
            token,
            get_settings().jwt_key,
            algorithms=[ALGORITHM],
            issuer=ISSUER,
            audience=AUDIENCE,
            leeway=CLOCK_SKEW.total_seconds(),
            options={"require": ["exp", "sub", "jti"]},
        )
    except jwt.InvalidTokenError as ex:
        logger.debug("Token rejected: %s", ex)
        return None

    sub, email, jti = payload.get("sub"), payload.get("email"), payload.get("jti")
    if not all(isinstance(v,str) for v in (sub, email, jti)):
        logger.debug("Token rejected: invalid claim types")
        return None
    return TokenClaims(sub, email, jti, datetime.fromtimestamp(payload["exp"], UTC))
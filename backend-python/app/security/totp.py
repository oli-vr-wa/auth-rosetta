import hashlib
import re
from datetime import datetime

import pyotp

ISSUER = "AuthRosetta-Python"
_CODE = re.compile(r"[0-9]{6}")


def generate_secret() -> str:
    """Generate a new TOTP secret.

    Returns:
        A base32-encoded TOTP secret.
    """
    return pyotp.random_base32()


def _totp(secret: str) -> pyotp.TOTP:
    """Return a TOTP object for the given secret.

    Args:
        secret: The base32-encoded TOTP secret.

    Returns:
        A pyotp.TOTP object configured with the given secret.
    """
    return pyotp.TOTP(secret, digits=6, digest=hashlib.sha1, interval=30)


def build_uri(email: str, secret: str) -> str:
    """Build a provisioning URI for the given email and secret.

    Args:
        email: The email address of the user.
        secret: The base32-encoded TOTP secret.

    Returns:
        A provisioning URI that can be used to configure a TOTP authenticator app.
    """
    return _totp(secret).provisioning_uri(name=email, issuer_name=ISSUER)


def verify(secret: str, code: str, *, at: datetime | None = None) -> bool:
    """Verify a TOTP code for the given secret at the specified time.

    Args:
        secret: The base32-encoded TOTP secret.
        code: The 6-digit TOTP code to verify.
        at: The time at which to verify the code.

    Returns:
        True if the code is valid, False otherwise.
    """
    if not _CODE.fullmatch(code):
        return False
    return _totp(secret).verify(code, for_time=at, valid_window=1)
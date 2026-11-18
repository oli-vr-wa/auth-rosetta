import secrets
from functools import cache

import bcrypt

WORK_FACTOR = 12
MAX_PASSWORD_BYTES = 72


def hash_password(password: str) -> str:
    """Hash a password using bcrypt.
    
    Args:
        password: The plaintext password to hash.

    Returns:
        The bcrypt hash of the password as a string.
    """
    # Callers must enforce the 72 byte limit on passwords
    raw = password.encode("utf-8")
    return bcrypt.hashpw(raw, bcrypt.gensalt(rounds=WORK_FACTOR)).decode("ascii")


@cache
def _dummy_hash() -> str:
    """Return a dummy hash for timing attacks.

    Returns:
        A bcrypt hash of a random password.
    """
    # Built lazily and cached to avoid repeated computation
    return hash_password(secrets.token_urlsafe(16))


def verify_against_dummy() -> bool:
    """Spend the same amount of time as verifying a real password against a hash and always return False.
    Used for unknown emails so response time does not reveal whether the email exists.

    Returns:
        False
    """
    bcrypt.checkpw(b"x", _dummy_hash().encode("ascii"))
    return False


def verify_password(password: str, password_hash: str) -> bool:
    """Verify a password against a hash, enforcing the maximum password length.

    Args:
        password: The plaintext password to verify.
        password_hash: The bcrypt hash to verify against.

    Returns:
        True if the password matches the hash and is within the maximum length, False otherwise.
    """
    raw = password.encode("utf-8")
    if len(raw) > MAX_PASSWORD_BYTES:
        return verify_against_dummy()
    try:
        return bcrypt.checkpw(raw, password_hash.encode("ascii"))
    except ValueError:
        return False # malformed input is simple "wrong"
MAX_EMAIL_LENGTH = 256
MIN_PASSWORD_LENGTH = 8
MAX_PASSWORD_BYTES = 72


class RequestValidationError(Exception):
    """A business-rule violation. The API layer will map it to 400 with this message."""


def normalize_email(email: str) -> str:
    """Normalize an email address by stripping whitespace and converting to lowercase."""
    return email.strip().lower()


def is_utf8_encodable(value: str) -> bool:
    """Check if a string can be encoded in UTF-8."""
    try:
        value.encode("utf-8")        
    except UnicodeEncodeError:
        return False
    return True


def is_storeable_text(value: str) -> bool:
    """Check if a string can be stored in the database (no null bytes and UTF-8 encodable)."""
    return "\x00" not in value and is_utf8_encodable(value)


def validate_register(email: str, passowrd: str) -> None:
    errors: list[str] = []

    if not email:
        errors.append("Email is required.")
    elif len(email) > MAX_EMAIL_LENGTH:
        errors.append(f"Email must be at most {MAX_EMAIL_LENGTH} characters")
    elif not is_storeable_text(email):
        errors.append("Email contains invalid characters.")

    if len(passowrd) < MIN_PASSWORD_LENGTH:
        errors.append(f"Password must be at least {MIN_PASSWORD_LENGTH} characters")
    elif not is_utf8_encodable(passowrd):
        errors.append("Password contains invalid characters.")
    elif len(passowrd.encode("utf-8")) > MAX_PASSWORD_BYTES:
        errors.append(f"Password must be at most {MAX_PASSWORD_BYTES} bytes")

    if errors:
        raise RequestValidationError("\n".join(errors))
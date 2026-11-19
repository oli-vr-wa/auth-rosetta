from datetime import datetime
from typing import Protocol

from app.models import User


class UserRepository(Protocol):
    def get_by_email(self, email: str) -> User | None: 
        """Retrieve a user by email; None if not found."""
        ...

    def try_add(self, user: User) -> bool:
        """Insert; False if the email is already taken."""
        ...

    def save(self) -> None:
        """Persist changes to the user."""
        ...


class RevokedTokenRepository(Protocol):
    def revoke(self, jti: str, expires_at: datetime) -> None:
        """Revoke a token by its JTI until the specified expiration time."""
        ...

    def is_revoked(self, jti: str) -> bool:
        """Check if a token by its JTI is revoked."""
        ...

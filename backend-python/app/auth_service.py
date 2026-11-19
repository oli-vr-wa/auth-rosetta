import logging
import uuid
from dataclasses import dataclass
from datetime import datetime

from app.models import User
from app.ports import UserRepository, RevokedTokenRepository
from app.security import passwords, tokens, totp
from app.validation import is_storeable_text, normalize_email, validate_register

logger = logging.getLogger("authrosetta.auth")


@dataclass(frozen=True)
class RegisterResult:
    otpauth_uri: str
    secret: str


class AuthService:
    def __init__(self, users: UserRepository, revoked_tokens: RevokedTokenRepository) -> None:
        self._users = users
        self._revoked_tokens = revoked_tokens

    def _find_user(self, email: str) -> User | None:
        """Find a user by email after normalizing and validating it.
        
        Args:
            email (str): The email of the user to find.

        Returns:
            User | None: The user if found and valid, otherwise None.
        """
        email = normalize_email(email)
        if not is_storeable_text(email):
            return None # Email is not valid for storage so no need to send it to the database which would raise an error
        return self._users.get_by_email(email)
    

    def register(self, email: str, password: str) -> RegisterResult:
        """Register a new user with email and password.

        Args:
            email (str): The email of the user to register.
            password (str): The password of the user to register.

        Returns:
            RegisterResult | None: The registration result if successful, otherwise None.
        """
        email = normalize_email(email)
        validate_register(email, password)
        if (self._users.get_by_email(email)) is not None:
            return None

        secret = totp.generate_secret()
        user = User(
            id=str(uuid.uuid4()),
            email=email,
            password_hash=passwords.hash_password(password),
            totp_secret=secret,
            two_factor_enabled=False
        )

        if not self._users.try_add(user):
            return None
        
        return RegisterResult(totp.build_uri(email, secret), secret=secret)

    def confirm_two_factor(self, email: str, code: str) -> bool:
        """Confirm the two-factor authentication for a user.

        Args:
            email (str): The email of the user to confirm.
            code (str): The two-factor authentication code.

        Returns:
            bool: True if the confirmation is successful, otherwise False.
        """
        user = self._find_user(email)
        if user is None or not totp.verify(user.totp_secret, code):
            return False
        
        user.two_factor_enabled = True
        self._users.save()

        return True

    def login(self, email: str, password: str, code: str) -> str | None:
        """Authenticate a user with email, password, and two-factor code.

        Args:
            email (str): The email of the user to authenticate.
            password (str): The password of the user to authenticate.
            code (str): The two-factor authentication code.

        Returns:
            str | None: The authentication token if successful, otherwise None.
        """
        user = self._find_user(email)

        password_ok = (
            passwords.verify_password(password, user.password_hash)
            if user is not None
            else passwords.verify_against_dummy()
        )
        code_ok = user is not None and totp.verify(user.totp_secret, code)

        if user is None or not password_ok or not user.two_factor_enabled or not code_ok:
            logger.debug(
                "Login rejected (user found: %s, password_ok: %s, two_factor_enabled: %s, code_ok: %s)",
                user is not None,
                password_ok,
                user.two_factor_enabled if user is not None else False,
                code_ok
            )
            return None

        return tokens.create_token(user.id, user.email)

    def authenticate(self, token: str | None) -> tokens.TokenClaims | None:
        """Authenticate a user based on the provided token.

        Args:
            token (str | None): The authentication token.

        Returns:
            tokens.TokenClaims | None: The token claims if authentication is successful, otherwise None.
        """
        if not token:
            return None
        claims = tokens.decode_token(token)
        if claims is None:
            return None
        if self._revoked_tokens.is_revoked(claims.jti):
            logger.debug("Token rejected: revoked")
            return None
        return claims

    def logout(self, jti: str, expires_at: datetime) -> None:
        """Log out a user by revoking the provided token.

        Args:
            jti (str): The unique identifier of the token to revoke.
            expires_at (datetime): The expiration time of the token.

        Returns:
            None
        """
        self._revoked_tokens.revoke(jti, expires_at)
from typing import Annotated

from fastapi import Cookie, Depends, HTTPException
from sqlalchemy.orm import Session

from app.api.cookies import COOKIE_NAME
from app.auth_service import AuthService
from app.db import get_session
from app.repositories import SqlRevokedTokenRepository, SqlUserRepository
from app.security.tokens import TokenClaims


def get_auth_service(session: Annotated[Session, Depends(get_session)]) -> AuthService:
    """Dependency to get an instance of AuthService.

    Args:
        session (Session): The database session.

    Returns:
        AuthService: An instance of AuthService.
    """
    return AuthService(SqlUserRepository(session), SqlRevokedTokenRepository(session))


AuthServiceDep = Annotated[AuthService, Depends(get_auth_service)]


def get_current_claims(auth: AuthServiceDep, access_token: Annotated[str | None, Cookie()] = None) -> TokenClaims:
    """Dependency to get the current token claims.

    Args:
        auth (AuthService): The authentication service.
        access_token (str | None): The access token from the cookie.

    Returns:
        TokenClaims: The claims extracted from the access token.

    Raises:
        HTTPException: If the access token is invalid or missing.
    """
    claims = auth.authenticate(access_token)
    if claims is None:
        raise HTTPException(status_code=401)
    return claims


CurrentClaims = Annotated[TokenClaims, Depends(get_current_claims)]
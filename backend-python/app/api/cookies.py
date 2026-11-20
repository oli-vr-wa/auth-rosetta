from fastapi import Response, Request

from app.security.tokens import EXPIRY

COOKIE_NAME = "access_token"


def _secure(request: Request) -> bool:
    """Determine if the request is secure (HTTPS).

    Args:
        request: The incoming HTTP request.

    Returns:
        True if the request is over HTTPS, False otherwise.
    """
    return request.url.scheme == "https"


def set_auth_cookie(response: Response, request: Request, token: str) -> None:
    """Set the authentication cookie in the response.

    Args:
        response: The HTTP response object.
        request: The incoming HTTP request.
        token: The authentication token to set in the cookie.

    Returns:
        None
    """
    response.set_cookie(
        COOKIE_NAME,
        token,
        max_age=(int(EXPIRY.total_seconds())),
        path="/",
        httponly=True,
        secure=_secure(request),
        samesite="strict"
    )


def clear_auth_cookie(response: Response, request: Request) -> None:
    """Clear the authentication cookie in the response.

    Args:
        response: The HTTP response object.
        request: The incoming HTTP request.

    Returns:
        None
    """
    response.delete_cookie(
        COOKIE_NAME,
        path="/",
        httponly=True,
        secure=_secure(request),
        samesite="strict"
    )
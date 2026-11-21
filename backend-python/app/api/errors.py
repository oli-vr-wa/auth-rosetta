import logging

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError as BodyValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException
from starlette.middleware.base import RequestResponseEndpoint
from starlette.responses import Response

from app.validation import RequestValidationError as RuleViolation

logger = logging.getLogger("authrosetta.api")

_MESSAGES = {
    400: "Invalid request",
    401: "Unauthorized",
    403: "Forbidden",
    404: "Not found",
    405: "Method not allowed",
    415: "Unsupported media type"
}

class BodyTooLarge(Exception):
    """Raised when the request body is too large and exceeds the allowed limit."""


def message_for(status: int) -> str:
    if status >= 500:
        return "Internal server error"
    return _MESSAGES.get(status, "Request failed")


def error_response(status: int, message: str, headers: dict[str, str] | None = None) -> JSONResponse:
    return JSONResponse(
        {"error": message},
        status_code=status,
        headers=headers
    )


def register_error_handling(app: FastAPI) -> None:
    # Added first, so it sits *inside* the CORS middleware that main.py adds afterwards.
    # Why a middleware and not exception_handler(Exception)? Starlette runs the catch-all
    # handler in its outermost layer, outside CORS, so 500 responses would lose their CORS
    # headers and the browser would show a CORS error instead of the real one.
    @app.middleware("http")
    async def catch_unhandled(request: Request, call_next: RequestResponseEndpoint) -> Response:
        """Catch all unhandled exceptions and return a 500 response.
        This middleware ensures that any unhandled exceptions are caught and logged,
        and a generic 500 error response is returned to the client.
        
        Args:
            request: The incoming HTTP request.
            call_next: The next request handler in the middleware chain.

        Returns:
            A Response object, either from the next handler or a 500 error response.
        """
        try:
            return await call_next(request)
        except Exception:
            logger.exception("%s %s -> 500", request.method, request.url.path)
            return error_response(500, message_for(500))

    @app.exception_handler(RuleViolation)
    async def on_rule_violation(request: Request, exc: RuleViolation) -> JSONResponse:
        """Handle rule violation exceptions and return a 400 response.

        Args:
            request: The incoming HTTP request.
            exc: The RuleViolation exception instance.

        Returns:
            A JSONResponse with a 400 status code and the exception message.
        """
        logger.warning("%s %s -> 400: %s", request.method, request.url.path, exc)
        return error_response(400, str(exc))

    @app.exception_handler(BodyValidationError)
    async def on_bad_body(request: Request, exc: BodyValidationError) -> JSONResponse:
        """Handle body validation errors and return a 400 response.

        Args:
            request: The incoming HTTP request.
            exc: The BodyValidationError exception instance.

        Returns:
            A JSONResponse with a 400 status code and a generic error message.
        """
        # Log where and what kind of problem, never the input values (they may contain sensitive information)
        problems = [(".".join(str(p) for p in e["loc"]), e["type"]) for e in exc.errors()]
        logger.warning("%s %s -> 400: %s", request.method, request.url.path, problems)
        return error_response(400, message_for(400))

    @app.exception_handler(StarletteHTTPException)
    async def on_http_exception(request: Request, exc: StarletteHTTPException) -> JSONResponse:
        """Handle HTTP exceptions and return the appropriate response.

        Args:
            request: The incoming HTTP request.
            exc: The StarletteHTTPException instance.

        Returns:
            A JSONResponse with the status code and message corresponding to the exception.
        """
        logger.warning("%s %s -> %s", request.method, request.url.path, exc.status_code)
        return error_response(exc.status_code, message_for(exc.status_code), exc.headers)

    @app.exception_handler(BodyTooLarge)
    async def on_body_too_large(request: Request, exc: BodyTooLarge) -> JSONResponse:
        """Handle requests with bodies that are too large and return a 400 response.

        Args:
            request: The incoming HTTP request.
            exc: The BodyTooLarge exception instance.

        Returns:
            A JSONResponse with a 400 status code and a generic error message.
        """
        logger.warning("%s %s -> 400: request body too large", request.method, request.url.path)
        return error_response(400, message_for(400))
    
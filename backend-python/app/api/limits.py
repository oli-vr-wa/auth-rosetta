import logging

from starlette.types import ASGIApp, Message, Receive, Scope, Send

from app.api.errors import BodyTooLarge, error_response, message_for

MAX_BODY_BYTES = 8 * 1024  # contract: bodies over 8 KiB get 400 "Invalid request"

logger = logging.getLogger("authrosetta.api")


def _declared_length(scope: Scope) -> int | None:
    for name, value in scope["headers"]:
        if name == b"content-length":
            try:
                return int(value)
            except ValueError:
                return None # malformed header: the server deals with it as if no content-length was provided
    return None


class BodySizeLimitMiddleware:
    def __init__(self, app: ASGIApp, max_bytes: int = MAX_BODY_BYTES) -> None:
        self.app = app
        self.max_bytes = max_bytes

    async def __call__(self, scope: Scope, receive: Receive, send: Send) -> None:
        if scope["type"] != "http":
            await self.app(scope, receive, send)
            return

        declared = _declared_length(scope)
        if declared is not None and declared > self.max_bytes:
            logger.warning("%s %s -> 400: declared body of %d bytes", scope["method"], scope["path"], declared)
            await error_response(400, message_for(400))(scope, receive, send)
            return

        received = 0

        async def limited_receive() -> Message:
            nonlocal received
            message = await receive()
            if message["type"] == "http.request":
                received += len(message.get("body", b""))
                if received > self.max_bytes:
                    raise BodyTooLarge()
            return message

        await self.app(scope, limited_receive, send)
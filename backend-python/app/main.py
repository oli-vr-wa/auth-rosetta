import logging

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from app.api.errors import register_error_handling
from app.api.routes import router
from app.api.limits import BodySizeLimitMiddleware
from app.config import get_settings


def configure_logging(level: str) -> None:
    logging.basicConfig(format="%(asctime)s %(levelname)-5s %(name)s: %(message)s")
    logging.getLogger("authrosetta").setLevel(level)


def create_app() -> FastAPI:
    settings = get_settings() # If config is invalid, this will raise an error and server will not start
    configure_logging(settings.log_level)

    # No /docs, /redoc, or /openapi.json (not in the contract, and unknown routes must give the JSON 404),
    # and no trailing-slash redirects, to keep behaviour identical across stacks. 
    app = FastAPI(title="Auth Rosetta (Python)", docs_url=None, redoc_url=None, openapi_url=None, redirect_slashes=False)

    register_error_handling(app)                    # First inner layer of error handling
    app.add_middleware(BodySizeLimitMiddleware)     # Enforce maximum body size for incoming requests
    app.add_middleware(                             # Last outer layer of middleware, so error responses get CORS headers
        CORSMiddleware,
        allow_origins=[settings.frontend_origin],
        allow_credentials=True,
        allow_methods=["GET", "POST", "OPTIONS"],
        allow_headers=["Content-Type"],
    )    
    app.include_router(router)

    @app.get("/health")
    def health() -> dict[str, str]:
        return {"status": "ok"}

    return app

app = create_app()

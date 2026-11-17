from fastapi import FastAPI

from app.config import get_settings

def create_app() -> FastAPI:
    get_settings() # If config is invalid, this will raise an error and server will not start
    app = FastAPI(title="Auth Rosetta (Python)")

    @app.get("/health")
    def health() -> dict[str, str]:
        return {"status": "ok"}

    return app

app = create_app()

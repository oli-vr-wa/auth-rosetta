"""Works out which backend the contract suite should test.

Order of precedence (mirrors Docker Compose: shell variable beats .env):
  1. CONTRACT_BASE_URL / CONTRACT_ORIGIN   explicit override
  2. VITE_API_URL / FRONTEND_ORIGIN        shell environment, then the root .env
"""
import os
from dataclasses import dataclass
from functools import lru_cache
from pathlib import Path
from urllib.parse import urlsplit

ENV_FILE = Path(__file__).resolve().parent.parent / ".env"
DEFAULT_ORIGIN = "http://localhost:5173"
BACKEND_PORTS = {"net": 5001, "nest": 5002, "python": 5003, "go": 5004, "java": 5005}


class TargetError(Exception):
    pass


@dataclass(frozen=True)
class Target:
    base_url: str
    origin: str
    notes: tuple[str, ...]


def read_env_file(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    if not path.is_file():
        return values
    for raw in path.read_text(encoding="utf-8-sig").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[len("export "):].lstrip()
        key, sep, value = line.partition("=")
        if not sep:
            continue
        key, value = key.strip(), value.strip()
        if len(value) >= 2 and value[0] in "'\"" and value[-1] == value[0]:
            value = value[1:-1]
        else:
            value = value.split(" #", 1)[0].strip()  # inline comment on an unquoted value
        values[key] = value
    return values


def _plain(name: str, value: str) -> str:
    if "${" in value:
        raise TargetError(
            f"{name} in .env contains Docker Compose '${{...}}' syntax. "
            f"Put a plain value in .env; that syntax belongs in docker-compose.yml."
        )
    return value


def _effective(name: str, file_values: dict[str, str], notes: list[str]) -> tuple[str | None, str | None]:
    """Value as Docker Compose would see it, plus where it came from."""
    from_file = _plain(name, file_values[name]) if file_values.get(name) else None
    from_shell = os.environ.get(name) or None
    if from_shell is not None:
        if from_file is not None and from_shell != from_file:
            notes.append(
                f"WARNING: {name} is set in your shell and differs from .env; "
                f"Docker Compose uses the shell value. Remove it with: Remove-Item Env:{name}"
            )
        return from_shell, "shell environment"
    if from_file is not None:
        return from_file, ".env"
    return None, None


def _check_origin(name: str, value: str) -> str:
    parts = urlsplit(value)
    if parts.scheme not in ("http", "https") or not parts.netloc or parts.path not in ("", "/"):
        raise TargetError(f"{name} must look like http://localhost:5003 (no path); got {value!r}")
    return f"{parts.scheme}://{parts.netloc}"


@lru_cache
def get_target() -> Target:
    notes: list[str] = []
    file_values = read_env_file(ENV_FILE)
    if not file_values:
        notes.append(f"note: no usable .env found at {ENV_FILE}")

    # --- base URL ---
    explicit = os.environ.get("CONTRACT_BASE_URL")
    if explicit:
        base, base_source = explicit, "CONTRACT_BASE_URL"
    else:
        base, base_source = _effective("VITE_API_URL", file_values, notes)
    if not base:
        raise TargetError(
            "No target backend: set VITE_API_URL in the root .env "
            "(for example http://localhost:5003) or set CONTRACT_BASE_URL."
        )
    base = _check_origin("base URL", base)
    notes.insert(0, f"base URL from {base_source}")

    # --- origin used by the CORS tests ---
    explicit_origin = os.environ.get("CONTRACT_ORIGIN")
    if explicit_origin:
        origin, origin_source = explicit_origin, "CONTRACT_ORIGIN"
    else:
        origin, origin_source = _effective("FRONTEND_ORIGIN", file_values, notes)
    origin = _check_origin("origin", origin or DEFAULT_ORIGIN)
    notes.insert(1, f"origin from {origin_source or 'default'}")

    # --- does the active profile agree with the URL? ---
    profiles_raw, _ = _effective("COMPOSE_PROFILES", file_values, notes)
    profiles = [p.strip() for p in (profiles_raw or "").split(",") if p.strip()]
    known = [p for p in profiles if p in BACKEND_PORTS]
    if len(known) == 1:
        notes.insert(2, f"active backend profile: {known[0]}")
        port = urlsplit(base).port
        if base_source != "CONTRACT_BASE_URL" and port != BACKEND_PORTS[known[0]]:
            raise TargetError(
                f"COMPOSE_PROFILES={known[0]} expects port {BACKEND_PORTS[known[0]]}, "
                f"but VITE_API_URL points at {base}. Fix one of them in .env."
            )
    return Target(base, origin, tuple(notes))
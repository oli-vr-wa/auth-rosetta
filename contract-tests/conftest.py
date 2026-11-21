import httpx2 as httpx
import pytest

from target import TargetError, get_target

def pytest_configure(config):
    try:
        target = get_target()
    except TargetError as ex:
        raise pytest.UsageError(str(ex)) from None


    try:
        httpx.get(f"{target.base_url}/api/auth/me", timeout=5)
    except Exception as ex:
        raise pytest.UsageError(
            f"Nothing is answering at {target.base_url} ({type(ex).__name__})"
            f"Is the stack up? Try: docker compose ps"
        ) from None


def pytest_report_header(config):
    target = get_target()
    return [f"contract target: {target.base_url} (CORS origin {target.origin})"] + [f"  {note}" for note in target.notes]
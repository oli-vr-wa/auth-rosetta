import os

import pytest

os.environ.update(
    {
        "DB_USER": "test",
        "DB_PASSWORD": "test",
        "JWT_KEY": "k" * 64,
        "DB_HOST": "localhost",
        "DB_PORT": "5432",
        "DB_NAME": "auth_test",
        "FRONTEND_ORIGIN": "http://localhost:5173",
        "LOG_LEVEL": "INFO",
    }
)

from app.security import passwords

@pytest.fixture
def fast_bcrypt(monkeypatch):
    """Fixture to temporarily set a fast bcrypt work factor for testing.
    Opt-in: cost 4 instead of 12 for tests that hash many passwords."""
    monkeypatch.setattr(passwords, "WORK_FACTOR", 4)
    passwords._dummy_hash.cache_clear()
    yield
    passwords._dummy_hash.cache_clear()
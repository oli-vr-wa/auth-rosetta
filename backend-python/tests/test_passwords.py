import pytest

from app.security import passwords


def test_hash_uses_cost_12_and_verifies():
    password = "test password"
    h = passwords.hash_password(password)
    assert h.startswith("$2b$12$")
    assert passwords.verify_password(password, h)
    assert not passwords.verify_password("wrong password", h)


@pytest.mark.parametrize("weird", ["", "a" * 73, "pass\x00word", "é" * 40])
def test_verify_nerver_raises_on_odd_input(weird):
    h = passwords.hash_password("correct horse")
    assert passwords.verify_password(weird, h) is False


def test_verify_against_dummy_is_always_false():
    assert passwords.verify_against_dummy() is False


def test_nul_byte_does_not_truncate_the_password():
    h = passwords.hash_password("pass\x00word-one")
    assert passwords.verify_password("pass\x00word-one", h)
    assert not passwords.verify_password("pass\x00word-two", h)
    assert not passwords.verify_password("pass", h)
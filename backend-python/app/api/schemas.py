from pydantic import BaseModel, ConfigDict


class _Body(BaseModel):
    # Base class for request bodies with strict validation and ignoring extra fields
    # Strict: disallow unknown fields and enforce type checks
    # Extra: ignore any additional fields not defined in the model
    model_config = ConfigDict(strict=True, extra="ignore")


class RegisterRequest(_Body):
    email: str
    password: str


class ConfirmRequest(_Body):
    email: str
    code: str


class LoginRequest(_Body):
    email: str
    password: str
    code: str
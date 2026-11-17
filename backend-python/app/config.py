from functools import lru_cache
from pathlib import Path
from urllib.parse import urlsplit
from sqlalchemy.engine import URL

from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict

ROOT_ENV = Path(__file__).resolve().parents[2] / ".env"
DEFAULT_FRONTEND_ORIGIN = "http://localhost:5173"


class Settings(BaseSettings):
	model_config = SettingsConfigDict(
		env_file=ROOT_ENV,
		env_file_encoding="utf-8",
		extra="ignore",				# root .env also golds COMPOSE_PROFILES, VITE_API_URL, ...etc
		env_ignore_empty=True,		# "DB_USER=" (blank) should count as missing
		hide_input_in_errors=True	# never echo values, e.g. HWT_KEY in error outpout
	)

	db_user: str
	db_password: str
	jwt_key: str = Field(min_length=32)
	db_host: str = "localhost"
	db_port: int = 5432
	db_name: str = "auth_python"
	frontend_origin: str = DEFAULT_FRONTEND_ORIGIN

	@field_validator("db_user", "db_password", "jwt_key")
	@classmethod
	def not_blank(cls, v: str) -> str:
		if not v.strip():
			raise ValueError("must not be blank")
		return v

	@field_validator("frontend_origin", mode="before")
	@classmethod
	def plain_origin(cls, v: object) -> str:
		v = (v or "").strip() if isinstance(v, str) or v is None else v
		v = v or DEFAULT_FRONTEND_ORIGIN
		parts = urlsplit(v)
		if (
			parts.scheme not in ("http", "https")
			or not parts.netloc
			or "*" in v
			or f"{parts.scheme}://{parts.netloc}" != v
		):
			raise ValueError("must be a plain origin such as http://localhost:5173 (no wildcard, path or trailing slash)")
		return v

	@property
	def database_url(self) -> URL:
		return URL.create(
			"postgresql+psycopg",
			username=self.db_user,
			password=self.db_password,
			host=self.db_host,
			port=self.db_port,
			database=self.db_name,
		)


@lru_cache
def get_settings() -> Settings:
	return Settings()
from sqlalchemy.dialects import postgresql
from sqlalchemy.schema import CreateTable

from app.models import User
from app.repositories import EMAIL_UNIQUE_CONSTRAINT


def test_email_constraint_name_matches_what_the_model_generates():
    ddl = str(CreateTable(User.__table__).compile(dialect=postgresql.dialect()))
    assert f"CONSTRAINT {EMAIL_UNIQUE_CONSTRAINT} UNIQUE (email)" in ddl
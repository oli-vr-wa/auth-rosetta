from datetime import datetime

from psycopg.errors import UniqueViolation
from sqlalchemy import select
from sqlalchemy.dialects.postgresql import insert
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.models import User, RevokedToken

EMAIL_UNIQUE_CONSTRAINT = "uq_user_email"


class SqlUserRepository:
    def __init__(self, session: Session) -> None:
        self._session = session

    def get_by_email(self, email: str) -> User | None:
        return self._session.scalar(select(User).where(User.email == email))

    def try_add(self, user: User) -> bool:
        self._session.add(user)
        try:
            self._session.commit()
            return True
        except IntegrityError as ex:
            self._session.rollback()
            orig = ex.orig
            # Only the email constraint means "already registered"
            # any other error is a real bug and should be raised as 500
            if (
                isinstance(orig, UniqueViolation)
                and orig.diag.constraint_name == EMAIL_UNIQUE_CONSTRAINT
            ):
                return False
            raise

    def save(self) -> None:
        self._session.commit()


class SqlRevokedTokenRepository:
    def __init__(self, session: Session) -> None:
        self._session = session

    def revoke(self, jti: str, expires_at: datetime) -> None:
        stmt = (
            insert(RevokedToken)
            .values(jti=jti, expires_at=expires_at)
            .on_conflict_do_nothing(index_elements=["jti"])
        )
        self._session.execute(stmt)
        self._session.commit()

    def is_revoked(self, jti: str) -> bool:
        found = self._session.scalar(select(RevokedToken.jti).where(RevokedToken.jti == jti))
        return found is not None
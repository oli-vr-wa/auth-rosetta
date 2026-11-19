from collections.abc import Iterator
from functools import lru_cache

from sqlalchemy import MetaData, Engine, create_engine
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

from app.config import get_settings

# Explicit constraints naming convention
# Without them Postgres will generate default constraint names which can lead to conflicts and make migrations harder to manage
NAMING_CONVENTION = {
    "ix": "ix_%(column_0_label)s",
    "uq": "uq_%(table_name)s_%(column_0_name)s",
    "ck": "ck_%(table_name)s_%(column_0_name)s",
    "fk": "fk_%(table_name)s_%(column_0_name)s_%(referred_table_name)s",
    "pk": "pk_%(table_name)s"
}

class Base(DeclarativeBase):
    metadata = MetaData(naming_convention=NAMING_CONVENTION)


@lru_cache
def get_engine() -> Engine:
    """Get a SQLAlchemy engine instance, cached for reuse.

    Returns:
        Engine: The SQLAlchemy engine connected to the database.
    """
    return create_engine(get_settings().database_url, pool_pre_ping=True)


@lru_cache
def get_session_factory() -> sessionmaker[Session]:
    """Get a SQLAlchemy session factory, cached for reuse.

    Returns:
        sessionmaker[Session]: A SQLAlchemy session factory.
    """
    return sessionmaker(get_engine(), expire_on_commit=False)


def get_session() -> Iterator[Session]:
    """Get a SQLAlchemy session, yielding it for use within a context manager.
    One session per request; closed even if the request fails.

    Yields:
        Session: A SQLAlchemy session instance.
    """
    with get_session_factory()() as session:
        yield session
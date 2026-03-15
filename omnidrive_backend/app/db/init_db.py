from app.db.base import *  # noqa: F401,F403
from app.db.session import engine
from app.models.base import Base


def init_db():
    Base.metadata.create_all(bind=engine)


"""Database engine and session management."""

from sqlalchemy.ext.asyncio import async_sessionmaker, create_async_engine

from career_guidance_service.config import settings

engine = create_async_engine(settings.database_url, echo=settings.debug, future=True)
AsyncSessionLocal = async_sessionmaker(bind=engine, expire_on_commit=False)

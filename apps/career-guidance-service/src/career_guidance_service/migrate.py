"""Apply Career Guidance migrations without starting the HTTP or event runtimes."""

import asyncio

from career_guidance_service.db import engine, initialize_database
from career_guidance_service.models import Base


async def main() -> None:
    try:
        await initialize_database(Base.metadata)
    finally:
        await engine.dispose()


if __name__ == "__main__":
    asyncio.run(main())

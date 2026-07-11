"""FastAPI dependencies for tenant isolation, auth, and DB sessions."""

from collections.abc import AsyncGenerator
from typing import Annotated

from fastapi import Depends, Header, HTTPException, status
from sqlalchemy.ext.asyncio import AsyncSession

from ai_recommendation_service.db import AsyncSessionLocal


class Actor:
    """Resolved actor from gateway-forwarded headers."""

    def __init__(
        self,
        user_id: str | None = None,
        role: str | None = None,
        tenant_id: str | None = None,
        permissions: str | None = None,
    ) -> None:
        self.user_id = user_id
        self.role = role
        self.tenant_id = tenant_id
        self.permissions = set((permissions or "").split(",")) if permissions else set()

    def has_permission(self, permission: str) -> bool:
        return permission in self.permissions


async def get_db() -> AsyncGenerator[AsyncSession]:
    async with AsyncSessionLocal() as session:
        yield session
        await session.commit()


DbSession = Annotated[AsyncSession, Depends(get_db)]


def require_actor(
    x_actor_user: Annotated[str | None, Header()] = None,
    x_actor_role: Annotated[str | None, Header()] = None,
    x_actor_tenant: Annotated[str | None, Header()] = None,
    x_actor_permissions: Annotated[str | None, Header()] = None,
) -> Actor:
    if not x_actor_user:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail={"code": "unauthorized", "message": "Missing actor header"},
        )
    return Actor(
        user_id=x_actor_user,
        role=x_actor_role,
        tenant_id=x_actor_tenant,
        permissions=x_actor_permissions,
    )


CurrentActor = Annotated[Actor, Depends(require_actor)]


def require_permission(permission: str):
    def checker(actor: CurrentActor) -> Actor:
        if not actor.has_permission(permission):
            raise HTTPException(
                status_code=status.HTTP_403_FORBIDDEN,
                detail={"code": "forbidden", "message": f"Missing permission {permission}"},
            )
        return actor

    return Depends(checker)


def get_tenant_id(
    x_tenant_id: Annotated[str | None, Header()] = None,
    x_tenant_code: Annotated[str | None, Header()] = None,
) -> str:
    tenant = x_tenant_id or x_tenant_code
    if not tenant:
        raise HTTPException(
            status_code=status.HTTP_422_UNPROCESSABLE_ENTITY,
            detail={"code": "validation_error", "message": "Tenant header required"},
        )
    return tenant


TenantId = Annotated[str, Depends(get_tenant_id)]


async def ensure_tenant_match(tenant_id: str, resource_tenant_id: str) -> None:
    if tenant_id != resource_tenant_id:
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail={"code": "tenant_mismatch", "message": "Resource belongs to another tenant"},
        )

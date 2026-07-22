"""Private Student Service client for learner-owned recommendation access."""

import asyncio
import json
import os
from http import HTTPStatus
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urljoin
from urllib.request import Request, urlopen

from ai_recommendation_service.config import settings

MAX_INTERNAL_JSON_RESPONSE_BYTES = 1 << 20


class LearnerScopeUnavailableError(RuntimeError):
    """Student ownership could not be resolved safely."""


async def resolve_learner_ids(tenant_id: str, user_id: str, role: str) -> set[str]:
    service_token = settings.internal_service_token or os.getenv("INTERNAL_SERVICE_TOKEN", "")
    if not service_token:
        raise LearnerScopeUnavailableError(  # noqa: TRY003
            "learner scope credentials are not configured"
        )
    base_url = settings.student_service_url
    if "://" not in base_url:
        base_url = "http://" + base_url
    endpoint = urljoin(base_url.rstrip("/") + "/", "internal/v1/learner-scope")
    endpoint += "?" + urlencode({"user_id": user_id, "role": role})
    request = Request(  # noqa: S310 - URL is restricted to the configured service
        endpoint,
        headers={
            "Authorization": f"Bearer {service_token}",
            "X-Tenant-ID": tenant_id,
        },
    )
    try:
        status_code, raw_body = await asyncio.to_thread(_read_scope_response, request)
    except (HTTPError, URLError, TimeoutError, OSError) as exc:
        raise LearnerScopeUnavailableError(  # noqa: TRY003
            "learner scope service is unavailable"
        ) from exc
    if status_code != HTTPStatus.OK:
        raise LearnerScopeUnavailableError(  # noqa: TRY003
            "learner scope could not be resolved"
        )
    try:
        body = json.loads(raw_body)
        return {str(value) for value in body.get("student_ids", []) if value}
    except (AttributeError, TypeError, ValueError, json.JSONDecodeError) as exc:
        raise LearnerScopeUnavailableError(  # noqa: TRY003
            "learner scope returned an invalid response"
        ) from exc


def _read_scope_response(request: Request) -> tuple[int, bytes]:
    with urlopen(request, timeout=5.0) as response:  # noqa: S310 - configured private service URL
        body = response.read(MAX_INTERNAL_JSON_RESPONSE_BYTES + 1)
        if len(body) > MAX_INTERNAL_JSON_RESPONSE_BYTES:
            raise LearnerScopeUnavailableError("learner scope response is too large")  # noqa: TRY003
        return response.status, body

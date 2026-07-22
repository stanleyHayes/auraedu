"""Privacy regression tests for the local event transport."""

import logging

from career_guidance_service.events.transport import ConsoleTransport


async def test_console_transport_logs_metadata_not_payload(caplog):
    payload = b'{"phone":"+233200000000","refresh_token":"never-log-me"}'
    with caplog.at_level(logging.INFO):
        await ConsoleTransport().publish("ai.guidance_generated.v1", payload)

    assert "ai.guidance_generated.v1" in caplog.text
    assert f"payload_bytes={len(payload)}" in caplog.text
    assert "+233200000000" not in caplog.text
    assert "never-log-me" not in caplog.text

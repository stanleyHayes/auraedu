"""Privacy regression tests for the local event transport."""

import logging

from ai_prediction_service.events.transport import ConsoleTransport


async def test_console_transport_logs_metadata_not_payload(caplog):
    payload = b'{"email":"student@example.com","access_token":"never-log-me"}'
    with caplog.at_level(logging.INFO):
        await ConsoleTransport().publish("ai.prediction_generated.v1", payload)

    assert "ai.prediction_generated.v1" in caplog.text
    assert f"payload_bytes={len(payload)}" in caplog.text
    assert "student@example.com" not in caplog.text
    assert "never-log-me" not in caplog.text

"""Authoritative JSON Schema assertions for serialized CloudEvents."""

from __future__ import annotations

import json
from collections.abc import Mapping
from pathlib import Path
from typing import Any, cast

from jsonschema import FormatChecker
from jsonschema.validators import validator_for


def assert_event_contract(schema_path: Path, event: Mapping[str, Any]) -> None:
    """Raise one readable assertion containing every contract violation."""
    schema = cast(dict[str, Any], json.loads(schema_path.read_text(encoding="utf-8")))
    validator_class = validator_for(schema)
    validator_class.check_schema(schema)
    validator = validator_class(schema, format_checker=FormatChecker())
    errors = sorted(
        validator.iter_errors(event),
        key=lambda error: tuple(str(segment) for segment in error.absolute_path),
    )
    if not errors:
        return

    details: list[str] = []
    for error in errors:
        location = ".".join(str(segment) for segment in error.absolute_path) or "<event>"
        details.append(f"{location}: {error.message}")
    raise AssertionError(f"event does not satisfy {schema_path.name}:\n- " + "\n- ".join(details))

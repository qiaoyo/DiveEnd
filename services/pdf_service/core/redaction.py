"""Helpers for keeping credentials out of user-visible errors and logs."""

import re
from typing import Any

REDACTED = "[redacted]"
MAX_SAFE_ERROR_LENGTH = 1200

_REDACTION_PATTERNS = [
    (
        re.compile(r"\b(Authorization\s*[:=]\s*)Bearer\s+([A-Za-z0-9._~+/=-]{8,})", re.IGNORECASE),
        lambda match: f"{match.group(1)}Bearer {REDACTED}",
    ),
    (
        re.compile(r"\bBearer\s+([A-Za-z0-9._~+/=-]{8,})", re.IGNORECASE),
        f"Bearer {REDACTED}",
    ),
    (
        re.compile(r"\b(sk-[A-Za-z0-9_-]{8,})\b"),
        REDACTED,
    ),
    (
        re.compile(
            r"([?&](?:access_token|refresh_token|api_key|apikey|key|token|client_secret|clientsecret)=)([^&#\s]+)",
            re.IGNORECASE,
        ),
        lambda match: f"{match.group(1)}{REDACTED}",
    ),
    (
        re.compile(
            r"\b((?:api[_-]?key|apikey|access[_-]?token|refresh[_-]?token|client[_-]?secret|clientsecret|token)\s*[:=]\s*)(\"[^\"]*\"|'[^']*'|[^\s,;)}\]]+)",
            re.IGNORECASE,
        ),
        lambda match: f"{match.group(1)}{REDACTED}",
    ),
    (
        re.compile(
            r"([\"']?(?:api[_-]?key|apiKey|access[_-]?token|accessToken|refresh[_-]?token|refreshToken|client[_-]?secret|clientSecret|token)[\"']?\s*:\s*)(\"[^\"]*\"|'[^']*'|[^\s,}]+)",
            re.IGNORECASE,
        ),
        lambda match: f"{match.group(1)}{REDACTED}",
    ),
]


def redact_sensitive_text(value: str) -> str:
    """Redact common credential shapes from a string."""
    text = str(value or "").strip()
    if not text:
        return ""

    for pattern, replacement in _REDACTION_PATTERNS:
        text = pattern.sub(replacement, text)

    if len(text) <= MAX_SAFE_ERROR_LENGTH:
        return text
    return f"{text[:MAX_SAFE_ERROR_LENGTH]}... [truncated]"


def redact_sensitive_value(value: Any) -> Any:
    """Redact strings inside JSON-like structures."""
    if isinstance(value, str):
        return redact_sensitive_text(value)
    if isinstance(value, dict):
        return {key: redact_sensitive_value(item) for key, item in value.items()}
    if isinstance(value, list):
        return [redact_sensitive_value(item) for item in value]
    return value

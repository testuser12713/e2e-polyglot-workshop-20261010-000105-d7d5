"""Lazy configuration for the invoice worker.

Every value is read from the environment inside :meth:`Config.from_env`, never at
import time, so a missing variable is reported by name instead of killing the
interpreter before anything can be logged. The worker talks to PostgreSQL and
Valkey directly; there is no SQLite or in-memory fallback.
"""

from __future__ import annotations

import os
from collections.abc import Mapping
from dataclasses import dataclass

COMPLETED_ORDERS_QUEUE = "workshop:completed_orders"
DEFAULT_POLL_INTERVAL_MS = 2000


class ConfigError(RuntimeError):
    """A required configuration value is missing or malformed."""


def _require(env: Mapping[str, str], name: str) -> str:
    value = env.get(name)
    if not value:
        raise ConfigError(f"{name} is not set; declare it in RUN.json and start the worker with it")
    return value


def _int_env(env: Mapping[str, str], name: str, default: int | None = None) -> int:
    raw = env.get(name)
    if raw is None or raw == "":
        if default is None:
            raise ConfigError(
                f"{name} is not set; declare it in RUN.json and start the worker with it"
            )
        return default
    try:
        return int(raw)
    except ValueError as exc:
        raise ConfigError(f"{name} must be an integer number of cents, got {raw!r}") from exc


@dataclass(frozen=True)
class Config:
    """Runtime configuration of the invoice worker."""

    database_url: str
    valkey_url: str
    hourly_rate_cents: int
    poll_interval_ms: int
    completed_orders_queue: str = COMPLETED_ORDERS_QUEUE

    @classmethod
    def from_env(cls, env: Mapping[str, str] | None = None) -> Config:
        """Build the configuration from the environment (or a supplied mapping)."""
        source = os.environ if env is None else env
        poll_interval_ms = _int_env(source, "WORKER_POLL_INTERVAL_MS", DEFAULT_POLL_INTERVAL_MS)
        if poll_interval_ms <= 0:
            poll_interval_ms = DEFAULT_POLL_INTERVAL_MS
        return cls(
            database_url=_require(source, "DATABASE_URL"),
            valkey_url=_require(source, "VALKEY_URL"),
            hourly_rate_cents=_int_env(source, "WORKSHOP_HOURLY_RATE_CENTS"),
            poll_interval_ms=poll_interval_ms,
        )

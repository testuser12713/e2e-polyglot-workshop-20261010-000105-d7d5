"""Tests for the invoice worker skeleton: configuration and the poll loop."""

from __future__ import annotations

import asyncio
import logging
import os

import pytest
from config import COMPLETED_ORDERS_QUEUE, Config, ConfigError
from main import run_worker


def _env(**overrides: str | None) -> dict[str, str]:
    env: dict[str, str | None] = {
        "DATABASE_URL": "postgresql://app@localhost:5432/app",
        "VALKEY_URL": "redis://localhost:6379/0",
        "WORKSHOP_HOURLY_RATE_CENTS": "8900",
        "WORKER_POLL_INTERVAL_MS": "10",
    }
    env.update(overrides)
    return {key: value for key, value in env.items() if value is not None}


def test_config_reads_all_declared_variables() -> None:
    config = Config.from_env(_env())
    assert config.database_url == "postgresql://app@localhost:5432/app"
    assert config.valkey_url == "redis://localhost:6379/0"
    assert config.hourly_rate_cents == 8900
    assert config.poll_interval_ms == 10
    assert config.completed_orders_queue == COMPLETED_ORDERS_QUEUE


def test_config_defaults_the_poll_interval() -> None:
    config = Config.from_env(_env(WORKER_POLL_INTERVAL_MS=None))
    assert config.poll_interval_ms > 0


def test_config_missing_required_variable_names_it() -> None:
    with pytest.raises(ConfigError) as excinfo:
        Config.from_env(_env(DATABASE_URL=None))
    assert "DATABASE_URL" in str(excinfo.value)


def test_config_rejects_non_integer_rate() -> None:
    with pytest.raises(ConfigError) as excinfo:
        Config.from_env(_env(WORKSHOP_HOURLY_RATE_CENTS="not-a-number"))
    assert "WORKSHOP_HOURLY_RATE_CENTS" in str(excinfo.value)


class _EmptyQueue:
    """A stand-in queue that is never empty of ``None`` -- i.e. always empty."""

    def __init__(self) -> None:
        self.pops = 0

    async def pop(self) -> None:
        self.pops += 1
        return None

    async def close(self) -> None:
        return None


def test_empty_queue_keeps_the_worker_running(caplog: pytest.LogCaptureFixture) -> None:
    async def scenario() -> _EmptyQueue:
        queue = _EmptyQueue()
        stop = asyncio.Event()
        task = asyncio.create_task(run_worker(Config.from_env(_env()), queue, None, stop=stop))
        await asyncio.sleep(0.05)
        assert not task.done(), "the worker exited while the queue was empty"
        assert queue.pops >= 1, "the worker never polled the queue"
        stop.set()
        await asyncio.wait_for(task, timeout=1)
        return queue

    with caplog.at_level(logging.INFO, logger="worker.main"):
        queue = asyncio.run(scenario())

    assert queue.pops >= 1
    assert any("queue empty" in record.getMessage().lower() for record in caplog.records)


_LIVE_DATABASES = bool(os.environ.get("DATABASE_URL") and os.environ.get("VALKEY_URL"))


@pytest.mark.skipif(not _LIVE_DATABASES, reason="PostgreSQL and Valkey are not available")
def test_worker_connects_to_postgres_and_valkey() -> None:
    async def scenario() -> None:
        from queue import CompletedOrdersQueue

        import asyncpg

        config = Config(
            database_url=os.environ["DATABASE_URL"],
            valkey_url=os.environ["VALKEY_URL"],
            hourly_rate_cents=8900,
            poll_interval_ms=10,
        )
        queue = CompletedOrdersQueue(config.valkey_url)
        await queue.connect()
        try:
            assert await queue.ping()
        finally:
            await queue.close()
        connection = await asyncpg.connect(config.database_url)
        await connection.close()

    asyncio.run(scenario())

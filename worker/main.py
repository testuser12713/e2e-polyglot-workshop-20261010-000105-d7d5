"""Entry point of the invoice worker.

The worker connects to PostgreSQL and Valkey, then polls the Valkey list of
completed orders forever. An empty queue is normal and only produces a log line,
so the process keeps running and waits for the next order. Log lines carry only
technical ids (order id, queue name), never personal data.
"""

from __future__ import annotations

import asyncio
import contextlib
import logging
import signal
import sys
from collections.abc import Mapping
from queue import CompletedOrdersQueue, ensure_stdlib_queue
from typing import Any

import asyncpg
from config import Config, ConfigError
from invoice import process_message

logger = logging.getLogger("worker.main")

POLL_LOG_MESSAGE = "completed-orders queue empty; no order to process"


async def _wait_for_stop(stop: asyncio.Event, seconds: float) -> None:
    try:
        await asyncio.wait_for(stop.wait(), timeout=seconds)
    except TimeoutError:
        return


async def run_worker(
    config: Config,
    queue: Any,
    db: Any,
    *,
    stop: asyncio.Event | None = None,
) -> None:
    """Poll the completed-orders queue until ``stop`` is set.

    An empty queue is not an error: the worker logs it and keeps polling.
    """
    stop = stop if stop is not None else asyncio.Event()
    while not stop.is_set():
        try:
            message: Mapping[str, Any] | None = await queue.pop()
        except Exception:
            logger.exception("could not read the completed-orders queue")
            message = None
        if message is None:
            logger.info(POLL_LOG_MESSAGE)
            await _wait_for_stop(stop, config.poll_interval_ms / 1000)
            continue
        order_id = message.get("order_id")
        try:
            await process_message(message, db=db, queue=queue, config=config)
        except NotImplementedError:
            logger.warning("invoice processing not implemented yet; order_id=%s", order_id)
        except Exception:
            logger.exception("failed to process completed order; order_id=%s", order_id)


async def _connect_queue(config: Config) -> CompletedOrdersQueue:
    queue = CompletedOrdersQueue(config.valkey_url)
    try:
        await queue.connect()
    except Exception as exc:
        logger.error("could not connect to Valkey (VALKEY_URL): %s", exc)
        raise
    return queue


async def _connect_database(config: Config) -> Any:
    try:
        return await asyncpg.connect(config.database_url)
    except Exception as exc:
        logger.error("could not connect to PostgreSQL (DATABASE_URL): %s", exc)
        raise


async def _serve(config: Config) -> None:
    ensure_stdlib_queue()
    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        with contextlib.suppress(NotImplementedError, RuntimeError):
            loop.add_signal_handler(sig, stop.set)
    queue = await _connect_queue(config)
    db = await _connect_database(config)
    try:
        await run_worker(config, queue, db, stop=stop)
    finally:
        await queue.close()
        await db.close()


def _configure_logging() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
        stream=sys.stdout,
    )


def main() -> int:
    _configure_logging()
    try:
        config = Config.from_env()
    except ConfigError as exc:
        logger.error("invalid configuration: %s", exc)
        return 2
    try:
        asyncio.run(_serve(config))
    except KeyboardInterrupt:
        logger.info("worker stopped")
        return 0
    except Exception:
        logger.exception("worker stopped because of an error")
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())

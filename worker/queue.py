"""Access to the Valkey list of completed orders.

The producer (the Go API) pushes ``{"order_id": int, "order_number": str}`` JSON
messages onto the list ``workshop:completed_orders``; this worker pops them from
the left. The order id is the idempotency key of every message.

Because this file is named ``queue.py`` and the service directory is on
``sys.path``, it shadows the standard library ``queue`` module, which the Valkey
client imports internally. :func:`ensure_stdlib_queue` puts the genuine standard
library module back under its own name before the client is imported.
"""

from __future__ import annotations

import importlib.util
import json
import os
import sys
from typing import Any

from config import COMPLETED_ORDERS_QUEUE


def ensure_stdlib_queue() -> None:
    """Restore the standard library ``queue`` module under its own name.

    This module shadows the standard library module of the same name. Several
    third-party packages (including the Valkey client) import it, so load the
    genuine module from the standard library directory and register it as
    ``queue``. Calling this repeatedly is harmless.
    """
    current = sys.modules.get("queue")
    if current is not None and hasattr(current, "Empty"):
        return
    stdlib_dir = os.path.dirname(os.__file__)
    spec = importlib.util.spec_from_file_location("queue", os.path.join(stdlib_dir, "queue.py"))
    if spec is None or spec.loader is None:
        return
    module = importlib.util.module_from_spec(spec)
    sys.modules["queue"] = module
    spec.loader.exec_module(module)


class CompletedOrdersQueue:
    """A thin asynchronous client for the completed-orders Valkey list."""

    def __init__(self, url: str, name: str = COMPLETED_ORDERS_QUEUE) -> None:
        self._url = url
        self._name = name
        self._client: Any = None

    async def connect(self) -> None:
        """Connect to Valkey and fail loudly if it is unreachable."""
        ensure_stdlib_queue()
        import redis.asyncio as redis_asyncio

        client = redis_asyncio.from_url(self._url, decode_responses=True)
        await client.ping()
        self._client = client

    async def ping(self) -> bool:
        """Return whether the Valkey server answers a ping."""
        if self._client is None:
            raise RuntimeError("CompletedOrdersQueue.connect() must be awaited before ping()")
        return bool(await self._client.ping())

    async def pop(self) -> dict[str, Any] | None:
        """Pop the next completed order, or ``None`` when the list is empty."""
        if self._client is None:
            raise RuntimeError("CompletedOrdersQueue.connect() must be awaited before pop()")
        raw = await self._client.lpop(self._name)
        if raw is None:
            return None
        return json.loads(raw)

    async def close(self) -> None:
        if self._client is not None:
            await self._client.aclose()
            self._client = None

"""Integration tests for invoice persistence and idempotent processing.

These run against the PostgreSQL instance named by ``DATABASE_URL`` and are
skipped when it is not available. Every test creates the schema it uses, works on
its own freshly generated order id and removes only its own rows afterwards.
"""

from __future__ import annotations

import asyncio
import os
import time
from typing import Any

import asyncpg
import pytest
from config import Config
from invoice import process_message
from persistence import ensure_schema

pytestmark = pytest.mark.skipif(
    not os.environ.get("DATABASE_URL"), reason="DATABASE_URL is not available"
)

ORDER_ITEMS_SCHEMA = """
    CREATE TABLE IF NOT EXISTS order_items (
        id BIGSERIAL PRIMARY KEY,
        order_id BIGINT NOT NULL,
        kind TEXT NOT NULL,
        description TEXT NOT NULL,
        quantity NUMERIC,
        hours NUMERIC,
        unit_price_cents BIGINT,
        amount_cents BIGINT
    )
"""


def _config() -> Config:
    return Config(
        database_url=os.environ["DATABASE_URL"],
        valkey_url=os.environ.get("VALKEY_URL", "redis://localhost:6379/0"),
        hourly_rate_cents=8900,
        poll_interval_ms=10,
    )


def _new_order_id() -> int:
    return int(time.time_ns())


async def _prepare(conn: Any, order_id: int) -> None:
    await ensure_schema(conn)
    await conn.execute(ORDER_ITEMS_SCHEMA)
    await conn.execute(
        "INSERT INTO order_items (order_id, kind, description, quantity, hours, unit_price_cents)"
        " VALUES ($1, 'labor', 'Arbeitszeit', 1, 3.5, NULL), ($1, 'part', 'Bremsbelag', 2, NULL, 2500)",
        order_id,
    )


async def _cleanup(conn: Any, order_id: int) -> None:
    await conn.execute(
        "DELETE FROM invoice_items WHERE invoice_id IN (SELECT id FROM invoices WHERE order_id = $1)",
        order_id,
    )
    await conn.execute("DELETE FROM outbox WHERE order_id = $1", order_id)
    await conn.execute("DELETE FROM invoices WHERE order_id = $1", order_id)
    await conn.execute("DELETE FROM order_items WHERE order_id = $1", order_id)


def test_stores_invoice_items_and_one_notification() -> None:
    async def scenario() -> None:
        order_id = _new_order_id()
        order_number = f"A-{order_id}"
        conn = await asyncpg.connect(_config().database_url)
        try:
            await _prepare(conn, order_id)
            invoice_id = await process_message(
                {"order_id": order_id, "order_number": order_number},
                db=conn,
                queue=None,
                config=_config(),
            )
            assert invoice_id is not None

            invoice = await conn.fetchrow("SELECT * FROM invoices WHERE order_id = $1", order_id)
            assert invoice is not None
            assert invoice["invoice_number"] == f"RE-{order_number}"
            assert invoice["net_cents"] == 36150
            assert invoice["tax_cents"] == 6869
            assert invoice["gross_cents"] == 43019

            items = await conn.fetch(
                "SELECT description, amount_cents FROM invoice_items WHERE invoice_id = $1 ORDER BY position",
                invoice_id,
            )
            assert [(row["description"], row["amount_cents"]) for row in items] == [
                ("Arbeitszeit", 31150),
                ("Bremsbelag", 5000),
            ]

            notifications = await conn.fetch("SELECT * FROM outbox WHERE order_id = $1", order_id)
            assert len(notifications) == 1
            assert notifications[0]["sent_at"] is None
        finally:
            await _cleanup(conn, order_id)
            await conn.close()

    asyncio.run(scenario())


def test_repeated_message_creates_no_second_invoice_or_notification() -> None:
    async def scenario() -> None:
        order_id = _new_order_id()
        message = {"order_id": order_id, "order_number": f"A-{order_id}"}
        conn = await asyncpg.connect(_config().database_url)
        try:
            await _prepare(conn, order_id)
            first = await process_message(message, db=conn, queue=None, config=_config())
            assert first is not None
            second = await process_message(message, db=conn, queue=None, config=_config())

            assert second is None
            invoice_count = await conn.fetchval(
                "SELECT count(*) FROM invoices WHERE order_id = $1", order_id
            )
            item_count = await conn.fetchval(
                "SELECT count(*) FROM invoice_items WHERE invoice_id = $1", first
            )
            notification_count = await conn.fetchval(
                "SELECT count(*) FROM outbox WHERE order_id = $1", order_id
            )
            assert invoice_count == 1
            assert item_count == 2
            assert notification_count == 1
        finally:
            await _cleanup(conn, order_id)
            await conn.close()

    asyncio.run(scenario())

"""Integration tests for invoice persistence and idempotent processing.

These run against the PostgreSQL instance named by ``DATABASE_URL`` and are
skipped when it is not available. Every test applies the product's authoritative
schema (``backend/internal/store/schema.sql``, the same file the API applies at
startup), then works on its own freshly created order and removes only its own
rows afterwards.
"""

from __future__ import annotations

import asyncio
import os
from datetime import date
from pathlib import Path
from typing import Any

import asyncpg
import pytest
from config import Config
from invoice import process_message

SCHEMA_PATH = Path(__file__).resolve().parents[2] / "backend" / "internal" / "store" / "schema.sql"

pytestmark = pytest.mark.skipif(
    not os.environ.get("DATABASE_URL") or not SCHEMA_PATH.exists(),
    reason="DATABASE_URL or the shared schema is not available",
)


def _config() -> Config:
    return Config(
        database_url=os.environ["DATABASE_URL"],
        valkey_url=os.environ.get("VALKEY_URL", "redis://localhost:6379/0"),
        hourly_rate_cents=8900,
        poll_interval_ms=10,
    )


async def _prepare(conn: Any, order_number: str, email: str) -> int:
    await conn.execute(SCHEMA_PATH.read_text(encoding="utf-8"))
    customer_id = await conn.fetchval(
        "INSERT INTO customers (name, email, phone) VALUES ($1, $2, $3) RETURNING id",
        "Testkunde",
        email,
        "0000-000000",
    )
    vehicle_id = await conn.fetchval(
        "INSERT INTO vehicles (plate, make, model, mileage) VALUES ($1, $2, $3, $4) RETURNING id",
        f"T-{order_number}",
        "VW",
        "Golf",
        1000,
    )
    order_id = await conn.fetchval(
        "INSERT INTO orders (order_number, customer_id, vehicle_id, status, desired_date, problem)"
        " VALUES ($1, $2, $3, 'done', $4, $5) RETURNING id",
        order_number,
        customer_id,
        vehicle_id,
        date(2026, 10, 9),
        "",
    )
    await conn.execute(
        "INSERT INTO order_items"
        " (order_id, kind, description, quantity, hours, unit_price_cents, amount_cents)"
        " VALUES ($1, 'labor', 'Arbeitszeit', 1, 3.5, 8900, 31150),"
        "        ($1, 'part', 'Bremsbelag', 2, 0, 2500, 5000)",
        order_id,
    )
    return order_id


async def _cleanup(conn: Any, order_id: int) -> None:
    await conn.execute(
        "DELETE FROM invoice_items WHERE invoice_id IN"
        " (SELECT id FROM invoices WHERE order_id = $1)",
        order_id,
    )
    await conn.execute("DELETE FROM outbox WHERE order_id = $1", order_id)
    await conn.execute("DELETE FROM invoices WHERE order_id = $1", order_id)
    await conn.execute("DELETE FROM order_items WHERE order_id = $1", order_id)
    order = await conn.fetchrow(
        "SELECT customer_id, vehicle_id FROM orders WHERE id = $1", order_id
    )
    await conn.execute("DELETE FROM orders WHERE id = $1", order_id)
    if order is not None:
        await conn.execute("DELETE FROM vehicles WHERE id = $1", order["vehicle_id"])
        await conn.execute("DELETE FROM customers WHERE id = $1", order["customer_id"])


def test_stores_invoice_items_and_one_notification() -> None:
    async def scenario() -> None:
        order_number = f"A-{os.urandom(6).hex()}"
        email = f"{order_number}@example.test"
        order_id: int | None = None
        conn = await asyncpg.connect(_config().database_url)
        try:
            order_id = await _prepare(conn, order_number, email)
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
                "SELECT description, amount_cents FROM invoice_items"
                " WHERE invoice_id = $1 ORDER BY id",
                invoice_id,
            )
            assert [(row["description"], row["amount_cents"]) for row in items] == [
                ("Arbeitszeit", 31150),
                ("Bremsbelag", 5000),
            ]

            notifications = await conn.fetch("SELECT * FROM outbox WHERE order_id = $1", order_id)
            assert len(notifications) == 1
            assert notifications[0]["invoice_id"] == invoice_id
            assert notifications[0]["recipient"] == email
        finally:
            if order_id is not None:
                await _cleanup(conn, order_id)
            await conn.close()

    asyncio.run(scenario())


def test_repeated_message_creates_no_second_invoice_or_notification() -> None:
    async def scenario() -> None:
        order_number = f"A-{os.urandom(6).hex()}"
        email = f"{order_number}@example.test"
        order_id: int | None = None
        conn = await asyncpg.connect(_config().database_url)
        try:
            order_id = await _prepare(conn, order_number, email)
            message = {"order_id": order_id, "order_number": order_number}
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
            if order_id is not None:
                await _cleanup(conn, order_id)
            await conn.close()

    asyncio.run(scenario())

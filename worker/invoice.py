"""Invoice computation and queue-message processing for the workshop worker.

The two public entry points are :func:`compute_invoice` (pure and synchronous)
and :func:`process_message` (orchestrates one completed-order message):
compute the invoice from the stored positions, persist it together with its
positions and the customer notification in one PostgreSQL transaction, keyed on
the order id so a repeated message changes nothing.
"""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from decimal import ROUND_HALF_UP, Decimal, InvalidOperation
from typing import TYPE_CHECKING, Any

import persistence

if TYPE_CHECKING:
    from config import Config

TAX_RATE_PERCENT = 19


@dataclass(frozen=True)
class InvoiceLine:
    """One invoice position with its amount in whole cents."""

    description: str
    amount_cents: int


@dataclass(frozen=True)
class Invoice:
    """A computed invoice in whole cents, before persistence assigns a number."""

    order_id: int
    order_number: str
    lines: tuple[InvoiceLine, ...]
    net_cents: int
    tax_cents: int
    gross_cents: int

    @property
    def invoice_number(self) -> str:
        """Deterministic invoice number derived from the unique order number."""
        return f"RE-{self.order_number}"


def _to_decimal(value: Any) -> Decimal:
    if value is None:
        return Decimal(0)
    if isinstance(value, Decimal):
        return value
    try:
        return Decimal(str(value))
    except (InvalidOperation, ValueError):
        return Decimal(0)


def _line_amount_cents(item: Mapping[str, Any], hourly_rate_cents: int) -> int:
    """Amount of one position in whole cents.

    A ``labor`` position is hours times the configured hourly rate; any other
    position (a part) is quantity times its unit price. Rounding is half up so
    every amount stays a whole number of cents.
    """
    kind = str(item.get("kind", "")).strip().lower()
    if kind == "labor":
        hours = _to_decimal(item.get("hours"))
        if hours == 0 and item.get("quantity") is not None:
            hours = _to_decimal(item.get("quantity"))
        raw = hours * Decimal(hourly_rate_cents)
    else:
        unit_price = int(item.get("unit_price_cents") or 0)
        if unit_price:
            raw = _to_decimal(item.get("quantity") or 1) * Decimal(unit_price)
        else:
            raw = _to_decimal(item.get("amount_cents"))
    return int(raw.to_integral_value(rounding=ROUND_HALF_UP))


def compute_invoice(
    order_id: int,
    order_number: str,
    items: Sequence[Mapping[str, Any]],
    hourly_rate_cents: int,
) -> Invoice:
    """Compute the invoice of one finished order.

    ``items`` are the captured order positions (``kind`` ``"labor"`` or
    ``"part"``). Net is the labour hours times ``hourly_rate_cents`` plus the
    part amounts, tax is 19 % of net, and every amount is a whole number of
    cents.
    """
    lines = tuple(
        InvoiceLine(
            description=str(item.get("description") or ""),
            amount_cents=_line_amount_cents(item, hourly_rate_cents),
        )
        for item in items
    )
    net_cents = sum(line.amount_cents for line in lines)
    tax_cents = int(
        (Decimal(net_cents) * Decimal(TAX_RATE_PERCENT) / Decimal(100)).to_integral_value(
            rounding=ROUND_HALF_UP
        )
    )
    return Invoice(
        order_id=order_id,
        order_number=order_number,
        lines=lines,
        net_cents=net_cents,
        tax_cents=tax_cents,
        gross_cents=net_cents + tax_cents,
    )


async def process_message(
    message: Mapping[str, Any],
    *,
    db: Any,
    queue: Any,
    config: Config,
) -> int | None:
    """Process one ``{"order_id": int, "order_number": str}`` queue message.

    ``db`` is the live PostgreSQL connection, ``queue`` the completed-orders
    queue and ``config`` the worker configuration. The order id is the
    idempotency key: processing the same message twice leaves exactly one
    invoice and one notification. Returns the stored invoice id, or ``None``
    when the order had already been invoiced.
    """
    order_id = int(message["order_id"])
    order_number = str(message["order_number"])
    items = await persistence.fetch_order_items(db, order_id)
    invoice = compute_invoice(order_id, order_number, items, config.hourly_rate_cents)
    return await persistence.store_invoice(db, invoice)

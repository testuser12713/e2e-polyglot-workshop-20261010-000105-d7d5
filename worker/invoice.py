"""Invoice computation and queue-message processing for the workshop worker.

The two public entry points are :func:`compute_invoice` (pure and synchronous)
and :func:`process_message` (orchestrates one completed-order message). This
module declares their complete signatures; ticket #14 fills the bodies with
invoice generation, persistence, outbox and idempotency.
"""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from config import Config


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
    cents. Ticket #14 implements this.
    """
    raise NotImplementedError("ticket #14 (invoice generation) implements compute_invoice")


async def process_message(
    message: Mapping[str, Any],
    *,
    db: Any,
    queue: Any,
    config: Config,
) -> None:
    """Process one ``{"order_id": int, "order_number": str}`` queue message.

    ``db`` is the live PostgreSQL connection, ``queue`` the completed-orders
    queue and ``config`` the worker configuration. The order id is the
    idempotency key: processing the same message twice must leave exactly one
    invoice. Ticket #14 implements this.
    """
    raise NotImplementedError("ticket #14 (invoice generation) implements process_message")

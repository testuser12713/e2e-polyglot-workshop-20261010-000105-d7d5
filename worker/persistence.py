"""PostgreSQL persistence for generated invoices.

The worker writes three things per completed order inside a single transaction:

* one row in ``invoices`` (net, tax and gross in whole cents),
* one ``invoice_items`` row per computed position,
* one ``outbox`` row holding the customer notification (no real e-mail is sent).

Idempotency is keyed on the order id: ``invoices.order_id`` is unique and the
insert uses ``ON CONFLICT (order_id) DO NOTHING``. A repeated message therefore
inserts nothing and returns ``None``, so neither a second invoice nor a second
notification can appear. ``outbox.invoice_id`` is unique as well, so the
database itself refuses a second notification for the same invoice.

The table shapes are the ones the workshop API applies from
``backend/internal/store/schema.sql`` at startup; this module never creates or
alters the schema. The order positions are read from the shared ``order_items``
table, and the notification recipient from the order's customer.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from invoice import Invoice

ORDER_ITEMS_QUERY = """
    SELECT kind, description, quantity, hours, unit_price_cents, amount_cents
    FROM order_items
    WHERE order_id = $1
    ORDER BY id
"""

CUSTOMER_EMAIL_QUERY = """
    SELECT c.email
    FROM orders o
    JOIN customers c ON c.id = o.customer_id
    WHERE o.id = $1
"""

INSERT_INVOICE = """
    INSERT INTO invoices (invoice_number, order_id, net_cents, tax_cents, gross_cents)
    VALUES ($1, $2, $3, $4, $5)
    ON CONFLICT (order_id) DO NOTHING
    RETURNING id
"""

INSERT_INVOICE_ITEM = """
    INSERT INTO invoice_items (invoice_id, description, amount_cents)
    VALUES ($1, $2, $3)
"""

INSERT_OUTBOX = """
    INSERT INTO outbox (order_id, invoice_id, recipient, subject, body)
    VALUES ($1, $2, $3, $4, $5)
"""


async def fetch_order_items(db: Any, order_id: int) -> list[dict[str, Any]]:
    """Return the captured positions of one order as plain dictionaries."""
    rows = await db.fetch(ORDER_ITEMS_QUERY, order_id)
    return [dict(row) for row in rows]


async def fetch_customer_email(db: Any, order_id: int) -> str:
    """Return the customer e-mail of an order, or an empty string if unknown."""
    email = await db.fetchval(CUSTOMER_EMAIL_QUERY, order_id)
    return str(email) if email else ""


async def store_invoice(db: Any, invoice: Invoice) -> int | None:
    """Persist ``invoice`` with its positions and the outbox notification.

    Everything happens in one transaction. Returns the new invoice id, or
    ``None`` when an invoice for the same order already existed (the idempotent
    repeat case), in which case nothing at all is written.
    """
    async with db.transaction():
        row = await db.fetchrow(
            INSERT_INVOICE,
            invoice.invoice_number,
            invoice.order_id,
            invoice.net_cents,
            invoice.tax_cents,
            invoice.gross_cents,
        )
        if row is None:
            return None
        invoice_id = int(row["id"])
        await db.executemany(
            INSERT_INVOICE_ITEM,
            [(invoice_id, line.description, line.amount_cents) for line in invoice.lines],
        )
        recipient = await fetch_customer_email(db, invoice.order_id)
        subject = f"Rechnung {invoice.invoice_number} zum Auftrag {invoice.order_number}"
        body = (
            f"Rechnung {invoice.invoice_number} zum Auftrag {invoice.order_number}: "
            f"Netto {invoice.net_cents} Cent, MwSt. 19 % {invoice.tax_cents} Cent, "
            f"Brutto {invoice.gross_cents} Cent."
        )
        await db.execute(
            INSERT_OUTBOX,
            invoice.order_id,
            invoice_id,
            recipient,
            subject,
            body,
        )
        return invoice_id

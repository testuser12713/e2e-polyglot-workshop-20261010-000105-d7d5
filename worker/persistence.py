"""PostgreSQL persistence for generated invoices.

The worker writes three things per completed order inside a single transaction:

* one row in ``invoices`` (net, tax and gross in whole cents),
* one ``invoice_items`` row per computed position,
* one ``outbox`` row holding the customer notification (no real e-mail is sent).

Idempotency is keyed on the order id: ``invoices.order_id`` is unique and the
insert uses ``ON CONFLICT (order_id) DO NOTHING``. A repeated message therefore
inserts nothing and returns ``None``, so neither a second invoice nor a second
notification can appear.

The order positions are read from the ``order_items`` table owned by the
workshop API; only the columns named by the shared ``OrderItem`` type are read.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from invoice import Invoice

INVOICE_ITEMS_TABLE = "invoice_items"

SCHEMA_STATEMENTS: tuple[str, ...] = (
    """
    CREATE TABLE IF NOT EXISTS invoices (
        id BIGSERIAL PRIMARY KEY,
        invoice_number TEXT NOT NULL UNIQUE,
        order_id BIGINT NOT NULL UNIQUE,
        order_number TEXT NOT NULL,
        net_cents BIGINT NOT NULL,
        tax_cents BIGINT NOT NULL,
        gross_cents BIGINT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now()
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS invoice_items (
        id BIGSERIAL PRIMARY KEY,
        invoice_id BIGINT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
        position INTEGER NOT NULL,
        description TEXT NOT NULL,
        amount_cents BIGINT NOT NULL
    )
    """,
    """
    CREATE TABLE IF NOT EXISTS outbox (
        id BIGSERIAL PRIMARY KEY,
        invoice_id BIGINT NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
        order_id BIGINT NOT NULL,
        order_number TEXT NOT NULL,
        subject TEXT NOT NULL,
        body TEXT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
        sent_at TIMESTAMPTZ
    )
    """,
)

ORDER_ITEMS_QUERY = """
    SELECT kind, description, quantity, hours, unit_price_cents, amount_cents
    FROM order_items
    WHERE order_id = $1
    ORDER BY id
"""

INSERT_INVOICE = """
    INSERT INTO invoices (
        invoice_number, order_id, order_number, net_cents, tax_cents, gross_cents
    )
    VALUES ($1, $2, $3, $4, $5, $6)
    ON CONFLICT (order_id) DO NOTHING
    RETURNING id
"""

INSERT_INVOICE_ITEM = """
    INSERT INTO invoice_items (invoice_id, position, description, amount_cents)
    VALUES ($1, $2, $3, $4)
"""

INSERT_OUTBOX = """
    INSERT INTO outbox (invoice_id, order_id, order_number, subject, body)
    VALUES ($1, $2, $3, $4, $5)
"""


async def ensure_schema(db: Any) -> None:
    """Create the worker-owned tables if they do not exist yet.

    Called at startup so the worker works against a fresh PostgreSQL instance
    without any manual migration step. The API's own tables (``order_items``)
    are not touched here; they are created by the API at its startup.
    """
    for statement in SCHEMA_STATEMENTS:
        await db.execute(statement)


async def fetch_order_items(db: Any, order_id: int) -> list[dict[str, Any]]:
    """Return the captured positions of one order as plain dictionaries."""
    rows = await db.fetch(ORDER_ITEMS_QUERY, order_id)
    return [dict(row) for row in rows]


async def invoice_exists(db: Any, order_id: int) -> bool:
    """Return whether an invoice for ``order_id`` is already stored."""
    value = await db.fetchval("SELECT 1 FROM invoices WHERE order_id = $1", order_id)
    return value is not None


def _notification_body(invoice: Invoice) -> str:
    return (
        f"Rechnung {invoice.invoice_number} zum Auftrag {invoice.order_number}: "
        f"Netto {invoice.net_cents} Cent, MwSt. 19 % {invoice.tax_cents} Cent, "
        f"Brutto {invoice.gross_cents} Cent."
    )


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
            invoice.order_number,
            invoice.net_cents,
            invoice.tax_cents,
            invoice.gross_cents,
        )
        if row is None:
            return None
        invoice_id = int(row["id"])
        await db.executemany(
            INSERT_INVOICE_ITEM,
            [
                (invoice_id, position, line.description, line.amount_cents)
                for position, line in enumerate(invoice.lines)
            ],
        )
        await db.execute(
            INSERT_OUTBOX,
            invoice_id,
            invoice.order_id,
            invoice.order_number,
            f"Rechnung {invoice.invoice_number} zum Auftrag {invoice.order_number}",
            _notification_body(invoice),
        )
        return invoice_id

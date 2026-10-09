"""Tests for the pure invoice arithmetic in :mod:`invoice`."""

from __future__ import annotations

from invoice import TAX_RATE_PERCENT, Invoice, InvoiceLine, compute_invoice

HOURLY_RATE = 8900


def test_labor_and_part_positions_are_summed_in_cents() -> None:
    items = [
        {"kind": "labor", "description": "Arbeitszeit", "hours": "2.5", "quantity": 1},
        {"kind": "part", "description": "Bremsbelag", "quantity": 2, "unit_price_cents": 1500},
    ]

    invoice = compute_invoice(7, "A-1007", items, HOURLY_RATE)

    assert isinstance(invoice, Invoice)
    assert invoice.order_id == 7
    assert invoice.order_number == "A-1007"
    assert invoice.invoice_number == "RE-A-1007"
    assert invoice.lines == (
        InvoiceLine("Arbeitszeit", 22250),
        InvoiceLine("Bremsbelag", 3000),
    )
    assert invoice.net_cents == 25250
    assert invoice.tax_cents == 4798
    assert invoice.gross_cents == 30048


def test_tax_is_nineteen_percent_of_net() -> None:
    invoice = compute_invoice(
        1, "A-1", [{"kind": "part", "quantity": 1, "unit_price_cents": 100}], 0
    )
    assert invoice.net_cents == 100
    assert invoice.tax_cents == TAX_RATE_PERCENT
    assert invoice.gross_cents == 119


def test_every_amount_is_a_whole_number_of_cents() -> None:
    items = [
        {"kind": "labor", "hours": 1.333, "quantity": 1},
        {"kind": "part", "quantity": 3, "unit_price_cents": 333},
    ]
    invoice = compute_invoice(2, "A-2", items, 9999)
    for amount in (
        *(line.amount_cents for line in invoice.lines),
        invoice.net_cents,
        invoice.tax_cents,
        invoice.gross_cents,
    ):
        assert isinstance(amount, int)
    assert invoice.gross_cents == invoice.net_cents + invoice.tax_cents


def test_empty_order_computes_a_zero_invoice() -> None:
    invoice = compute_invoice(3, "A-3", [], HOURLY_RATE)
    assert invoice.lines == ()
    assert invoice.net_cents == 0
    assert invoice.tax_cents == 0
    assert invoice.gross_cents == 0

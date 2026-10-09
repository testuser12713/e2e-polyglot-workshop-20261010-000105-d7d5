-- Schema for the Kfz-Werkstatt customer portal.
-- Applied idempotently at API startup. PostgreSQL only; there is no SQLite or
-- in-memory fallback (SPEC AC-24).
--
-- All money is stored as integer cents. Status values are
-- requested | confirmed | in_progress | done | picked_up.

CREATE TABLE IF NOT EXISTS customers (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        NOT NULL,
    email      TEXT        NOT NULL UNIQUE,
    phone      TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vehicles (
    id         BIGSERIAL PRIMARY KEY,
    plate      TEXT        NOT NULL UNIQUE,
    make       TEXT        NOT NULL,
    model      TEXT        NOT NULL,
    mileage    INTEGER     NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id           BIGSERIAL PRIMARY KEY,
    order_number TEXT        NOT NULL UNIQUE,
    customer_id  BIGINT      NOT NULL REFERENCES customers (id),
    vehicle_id   BIGINT      NOT NULL REFERENCES vehicles (id),
    status       TEXT        NOT NULL DEFAULT 'requested'
        CHECK (status IN ('requested', 'confirmed', 'in_progress', 'done', 'picked_up')),
    desired_date DATE        NOT NULL,
    problem      TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS order_items (
    id               BIGSERIAL PRIMARY KEY,
    order_id         BIGINT       NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    kind             TEXT         NOT NULL CHECK (kind IN ('labor', 'part')),
    description      TEXT         NOT NULL,
    quantity         NUMERIC(12, 2) NOT NULL DEFAULT 0,
    hours            NUMERIC(12, 2) NOT NULL DEFAULT 0,
    unit_price_cents BIGINT       NOT NULL DEFAULT 0,
    amount_cents     BIGINT       NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS status_log (
    id          BIGSERIAL PRIMARY KEY,
    order_id    BIGINT      NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    from_status TEXT,
    to_status   TEXT        NOT NULL
);

CREATE TABLE IF NOT EXISTS invoices (
    id             BIGSERIAL PRIMARY KEY,
    invoice_number TEXT        NOT NULL UNIQUE,
    order_id       BIGINT      NOT NULL UNIQUE REFERENCES orders (id),
    net_cents      BIGINT      NOT NULL,
    tax_cents      BIGINT      NOT NULL,
    gross_cents    BIGINT      NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS invoice_items (
    id           BIGSERIAL PRIMARY KEY,
    invoice_id   BIGINT      NOT NULL REFERENCES invoices (id) ON DELETE CASCADE,
    description  TEXT        NOT NULL,
    amount_cents BIGINT      NOT NULL
);

CREATE TABLE IF NOT EXISTS employees (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    name          TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    token       TEXT        PRIMARY KEY,
    employee_id BIGINT      NOT NULL REFERENCES employees (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL
);

-- Outbox for customer notifications. The worker writes one row per invoice;
-- no real e-mail is ever sent (SPEC AC-09). invoice_id is unique so that
-- reprocessing the same completion message can never create a second entry
-- (SPEC AC-10).
CREATE TABLE IF NOT EXISTS outbox (
    id          BIGSERIAL PRIMARY KEY,
    order_id    BIGINT      NOT NULL REFERENCES orders (id),
    invoice_id  BIGINT      UNIQUE REFERENCES invoices (id),
    recipient   TEXT        NOT NULL,
    subject     TEXT        NOT NULL,
    body        TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);
CREATE INDEX IF NOT EXISTS idx_orders_order_number ON orders (order_number);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items (order_id);
CREATE INDEX IF NOT EXISTS idx_status_log_order_id ON status_log (order_id);
CREATE INDEX IF NOT EXISTS idx_sessions_employee_id ON sessions (employee_id);

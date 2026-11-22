-- +goose Up
-- Cobro manual: el superadmin registra cada pago; paid_until avanza y la racha cuenta meses pagados a tiempo.
CREATE TABLE subscriptions (
    store_id      bigint PRIMARY KEY REFERENCES stores(id) ON DELETE CASCADE,
    paid_until    date, -- NULL = aún no paga (tienda recién dada de alta)
    streak_months int NOT NULL DEFAULT 0,
    is_founder    boolean NOT NULL DEFAULT false,
    price_cents   int NOT NULL DEFAULT 0 CHECK (price_cents >= 0)
);
INSERT INTO subscriptions (store_id) SELECT id FROM stores;

CREATE TABLE payments (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id        bigint NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    amount_cents    int NOT NULL CHECK (amount_cents > 0),
    months          int NOT NULL CHECK (months BETWEEN 1 AND 12),
    paid_on         date NOT NULL,
    note            text NOT NULL DEFAULT '' CHECK (length(note) <= 200),
    on_time         boolean NOT NULL,
    prev_paid_until date,
    new_paid_until  date NOT NULL,
    admin_id        bigint REFERENCES admins(id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payments_store_idx ON payments(store_id, created_at DESC);

-- Bitácora: sobrevive al borrado de la tienda o del admin. admin_id NULL = el sistema (job diario).
CREATE TABLE admin_audit (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_id   bigint REFERENCES admins(id) ON DELETE SET NULL,
    store_id   bigint REFERENCES stores(id) ON DELETE SET NULL,
    action     text NOT NULL,
    detail     text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_audit_created_idx ON admin_audit(created_at DESC);

-- +goose Down
DROP TABLE admin_audit;
DROP TABLE payments;
DROP TABLE subscriptions;

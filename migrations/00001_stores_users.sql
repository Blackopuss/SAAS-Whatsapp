-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE stores (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    slug        text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9-]{3,30}$'),
    name        text NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    whatsapp    text NOT NULL DEFAULT '' CHECK (whatsapp = '' OR whatsapp ~ '^\+[1-9][0-9]{7,14}$'),
    template    text NOT NULL DEFAULT 'vitrina' CHECK (template IN ('vitrina', 'lista', 'boutique')),
    brand_color text NOT NULL DEFAULT 'oklch(0.62 0.19 350)',
    status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'suspended')),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id      bigint NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    email         citext NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX users_store_id_idx ON users(store_id);

-- +goose Down
DROP TABLE users;
DROP TABLE stores;

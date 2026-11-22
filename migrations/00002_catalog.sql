-- +goose Up
-- Cada tabla lleva store_id y las FK son compuestas (store_id, id):
-- la base de datos impide ligar un producto a la categoría de otra tienda.

CREATE TABLE categories (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id   bigint NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    name       text NOT NULL CHECK (length(name) BETWEEN 1 AND 60),
    position   int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (store_id, id)
);
CREATE INDEX categories_store_position_idx ON categories(store_id, position);

CREATE TABLE products (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id         bigint NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    category_id      bigint,
    name             text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    description      text NOT NULL DEFAULT '' CHECK (length(description) <= 4000),
    price_cents      int NOT NULL CHECK (price_cents >= 0),
    sale_price_cents int CHECK (sale_price_cents >= 0),
    active           boolean NOT NULL DEFAULT true,
    sold_out         boolean NOT NULL DEFAULT false,
    is_new           boolean NOT NULL DEFAULT false,
    on_sale          boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (store_id, id),
    FOREIGN KEY (store_id, category_id) REFERENCES categories(store_id, id) ON DELETE SET NULL (category_id)
);
CREATE INDEX products_store_category_idx ON products(store_id, category_id);

CREATE TABLE product_variants (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id   bigint NOT NULL,
    product_id bigint NOT NULL,
    size       text NOT NULL DEFAULT '' CHECK (length(size) <= 20),
    color      text NOT NULL DEFAULT '' CHECK (length(color) <= 30),
    sold_out   boolean NOT NULL DEFAULT false,
    UNIQUE (product_id, size, color),
    FOREIGN KEY (store_id, product_id) REFERENCES products(store_id, id) ON DELETE CASCADE
);
CREATE INDEX product_variants_store_product_idx ON product_variants(store_id, product_id);

CREATE TABLE product_images (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id       bigint NOT NULL,
    product_id     bigint NOT NULL,
    r2_key         text NOT NULL UNIQUE,
    position       int NOT NULL DEFAULT 0,
    dominant_color text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (store_id, product_id) REFERENCES products(store_id, id) ON DELETE CASCADE
);
CREATE INDEX product_images_store_product_idx ON product_images(store_id, product_id, position);

-- +goose Down
DROP TABLE product_images;
DROP TABLE product_variants;
DROP TABLE products;
DROP TABLE categories;

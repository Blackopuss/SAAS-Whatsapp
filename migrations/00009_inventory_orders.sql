-- +goose Up
-- Inventario y pedidos. Cantidades en milésimas de la unidad (int): 1 pieza = 1000, 0.25 kg = 250.
-- stock NULL = la vendedora no lleva inventario de eso (se usa el sold_out manual de siempre).

ALTER TABLE products
    ADD COLUMN unit  text NOT NULL DEFAULT 'pieza' CHECK (unit IN ('pieza', 'kg', 'g', 'litro', 'metro')),
    ADD COLUMN step  int  NOT NULL DEFAULT 1000 CHECK (step BETWEEN 1 AND 1000000),
    ADD COLUMN stock int CHECK (stock >= 0);

ALTER TABLE product_variants ADD COLUMN stock int CHECK (stock >= 0);

-- status: 0 pendiente, 1 vendido, 2 no vendido.
-- reason (solo no vendidos): 1 sin existencia, 2 la clienta no respondió, 3 precio, 4 otro, 5 sin respuesta en 7 días (automático).
CREATE TABLE orders (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id    bigint NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    code        text NOT NULL CHECK (code ~ '^[A-HJ-NP-Z2-9]{6}$'),
    status      smallint NOT NULL DEFAULT 0 CHECK (status BETWEEN 0 AND 2),
    reason      smallint NOT NULL DEFAULT 0 CHECK (reason BETWEEN 0 AND 5),
    total_cents int NOT NULL CHECK (total_cents >= 0),
    created_at  timestamptz NOT NULL DEFAULT now(),
    decided_at  timestamptz,
    UNIQUE (store_id, code),
    UNIQUE (store_id, id)
);
-- Lista por estado y gráficas por periodo.
CREATE INDEX orders_store_status_created_idx ON orders(store_id, status, created_at);
-- Solo los pendientes, para vencerlos cada hora sin recorrer toda la tabla.
CREATE INDEX orders_pending_idx ON orders(created_at) WHERE status = 0;

-- Nombre, categoría, talla y color se copian para que el historial no cambie si editan o borran el producto.
CREATE TABLE order_items (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    store_id    bigint NOT NULL,
    order_id    bigint NOT NULL,
    product_id  bigint,
    name        text NOT NULL,
    category    text NOT NULL DEFAULT '',
    size        text NOT NULL DEFAULT '',
    color       text NOT NULL DEFAULT '',
    unit        text NOT NULL DEFAULT 'pieza',
    qty         int NOT NULL CHECK (qty > 0),
    price_cents int NOT NULL CHECK (price_cents >= 0),
    FOREIGN KEY (store_id, order_id) REFERENCES orders(store_id, id) ON DELETE CASCADE,
    FOREIGN KEY (store_id, product_id) REFERENCES products(store_id, id) ON DELETE SET NULL (product_id)
);
CREATE INDEX order_items_order_idx ON order_items(store_id, order_id);
-- Para que borrar un producto no recorra todos los renglones de todas las tiendas.
CREATE INDEX order_items_product_idx ON order_items(product_id) WHERE product_id IS NOT NULL;

-- +goose Down
DROP TABLE order_items;
DROP TABLE orders;
ALTER TABLE product_variants DROP COLUMN stock;
ALTER TABLE products DROP COLUMN stock, DROP COLUMN step, DROP COLUMN unit;

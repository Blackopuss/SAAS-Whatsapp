-- Toda query lleva store_id, que sale del tenant (Host) y nunca del formulario.

-- name: ListCategories :many
SELECT * FROM categories WHERE store_id = $1 ORDER BY position, id;

-- name: CreateCategory :one
INSERT INTO categories (store_id, name, position)
VALUES ($1, $2, (SELECT coalesce(max(position), -1) + 1 FROM categories WHERE store_id = $1))
RETURNING *;

-- name: RenameCategory :execrows
UPDATE categories SET name = $3 WHERE store_id = $1 AND id = $2;

-- name: DeleteCategory :execrows
DELETE FROM categories WHERE store_id = $1 AND id = $2;

-- name: ReorderCategories :exec
-- @ids es la lista completa en el nuevo orden.
UPDATE categories c SET position = o.ord
FROM unnest(@ids::bigint[]) WITH ORDINALITY AS o(id, ord)
WHERE c.store_id = @store_id AND c.id = o.id;

-- name: ListProducts :many
SELECT * FROM products WHERE store_id = $1 ORDER BY created_at DESC, id DESC;

-- name: GetProduct :one
SELECT * FROM products WHERE store_id = $1 AND id = $2;

-- name: ListPanelProducts :many
-- Lista del panel: foto, categoría y existencia total (-1 si no lleva inventario).
SELECT p.id, p.name, p.price_cents, p.sale_price_cents, p.active, p.sold_out, p.is_new, p.on_sale, p.unit, p.step,
    coalesce(c.name, '')::text AS category,
    coalesce(i.r2_key, '')::text AS image_key,
    coalesce(i.widths, '{}')::int[] AS image_widths,
    v.n::int AS variants,
    coalesce(CASE WHEN v.n > 0 THEN v.stock ELSE p.stock END, -1)::int AS stock
FROM products p
LEFT JOIN categories c ON c.store_id = p.store_id AND c.id = p.category_id
LEFT JOIN LATERAL (
    SELECT r2_key, widths FROM product_images
    WHERE store_id = p.store_id AND product_id = p.id ORDER BY position, id LIMIT 1
) i ON true
CROSS JOIN LATERAL (
    SELECT count(*) AS n, sum(stock) AS stock FROM product_variants
    WHERE store_id = p.store_id AND product_id = p.id
) v
WHERE p.store_id = $1
ORDER BY p.created_at DESC, p.id DESC;

-- name: CreateProduct :one
INSERT INTO products (store_id, category_id, name, description, price_cents, sale_price_cents, is_new, on_sale, sold_out, unit, step, stock)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id;

-- name: UpdateProduct :execrows
-- Sin price_cents: después de crear, el precio normal solo lo cambia el superadmin (SetProductPrice).
-- La oferta debe quedar abajo del precio guardado aunque el superadmin lo haya bajado mientras tanto.
UPDATE products
SET category_id = $3, name = $4, description = $5, sale_price_cents = $6,
    is_new = $7, on_sale = $8, sold_out = $9, unit = $10, step = $11, stock = $12, updated_at = now()
WHERE store_id = $1 AND id = $2 AND ($6::int IS NULL OR $6::int < price_cents);

-- name: SetProductPrice :one
-- Solo superadmin. Quita la oferta si ya no queda abajo del precio nuevo.
UPDATE products p
SET price_cents = $3,
    sale_price_cents = CASE WHEN p.sale_price_cents < $3 THEN p.sale_price_cents END,
    on_sale = p.on_sale AND coalesce(p.sale_price_cents < $3, false),
    updated_at = now()
FROM products old
WHERE p.store_id = $1 AND p.id = $2 AND old.store_id = p.store_id AND old.id = p.id
RETURNING p.name, old.price_cents AS old_price_cents;

-- name: SetProductStock :execrows
UPDATE products SET stock = $3, sold_out = $4, updated_at = now() WHERE store_id = $1 AND id = $2;

-- name: SetVariantStock :exec
UPDATE product_variants SET stock = @stock, sold_out = @sold_out
WHERE store_id = @store_id AND product_id = @product_id AND size = @size AND color = @color;

-- name: DeleteProduct :execrows
DELETE FROM products WHERE store_id = $1 AND id = $2;

-- name: ToggleProductFlag :execrows
-- @flag ya viene validado por el handler (active, sold_out, is_new, on_sale).
UPDATE products SET
    active   = CASE WHEN @flag::text = 'active'   THEN NOT active   ELSE active   END,
    sold_out = CASE WHEN @flag::text = 'sold_out' THEN NOT sold_out ELSE sold_out END,
    is_new   = CASE WHEN @flag::text = 'is_new'   THEN NOT is_new   ELSE is_new   END,
    on_sale  = CASE WHEN @flag::text = 'on_sale'  THEN NOT on_sale  ELSE on_sale  END,
    updated_at = now()
WHERE store_id = @store_id AND id = @id;

-- name: ListVariants :many
SELECT * FROM product_variants WHERE store_id = $1 AND product_id = $2 ORDER BY id;

-- name: DeleteVariants :exec
DELETE FROM product_variants WHERE store_id = $1 AND product_id = $2;

-- name: CreateVariant :exec
INSERT INTO product_variants (store_id, product_id, size, color, sold_out, stock) VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListAllVariants :many
-- Para ajustar existencias desde la lista del panel (incluye productos ocultos).
SELECT product_id, size, color, sold_out, stock FROM product_variants WHERE store_id = $1 ORDER BY id;

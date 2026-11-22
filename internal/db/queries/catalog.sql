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

-- name: CreateProduct :one
INSERT INTO products (store_id, category_id, name, description, price_cents, sale_price_cents)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: UpdateProduct :execrows
UPDATE products
SET category_id = $3, name = $4, description = $5, price_cents = $6, sale_price_cents = $7, updated_at = now()
WHERE store_id = $1 AND id = $2;

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
INSERT INTO product_variants (store_id, product_id, size, color, sold_out) VALUES ($1, $2, $3, $4, $5);

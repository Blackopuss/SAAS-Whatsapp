-- name: CountProductImages :one
SELECT count(*) FROM product_images WHERE store_id = $1 AND product_id = $2;

-- name: LockProductForPhotos :one
-- Dentro de la transacción de alta de foto: dos subidas simultáneas no pueden pasar ambas el tope de fotos.
SELECT id FROM products WHERE store_id = $1 AND id = $2 FOR UPDATE;

-- name: CreateProductImage :one
INSERT INTO product_images (store_id, product_id, r2_key, position, dominant_color, widths)
VALUES ($1, $2, $3, (SELECT coalesce(max(position), -1) + 1 FROM product_images WHERE store_id = $1 AND product_id = $2), $4, $5)
RETURNING id;

-- name: ListProductImages :many
SELECT * FROM product_images WHERE store_id = $1 AND product_id = $2 ORDER BY position, id;

-- name: DeleteProductImage :one
DELETE FROM product_images WHERE store_id = $1 AND id = $2 RETURNING product_id, r2_key, widths;

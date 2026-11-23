-- name: CountProductImages :one
SELECT count(*) FROM product_images WHERE store_id = $1 AND product_id = $2;

-- name: CreateProductImage :one
INSERT INTO product_images (store_id, product_id, r2_key, position, dominant_color, widths)
VALUES ($1, $2, $3, (SELECT coalesce(max(position), -1) + 1 FROM product_images WHERE store_id = $1 AND product_id = $2), $4, $5)
RETURNING id;

-- name: ListProductImages :many
SELECT * FROM product_images WHERE store_id = $1 AND product_id = $2 ORDER BY position, id;

-- name: DeleteProductImage :one
DELETE FROM product_images WHERE store_id = $1 AND id = $2 RETURNING product_id, r2_key, widths;

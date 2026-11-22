-- name: GetStoreBySlug :one
-- Base del middleware de tenant (sprint 1): Host -> slug -> tienda.
SELECT * FROM stores WHERE slug = $1;

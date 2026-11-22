-- Tienda pública: solo productos activos, siempre filtrados por store_id del Host.

-- name: ListStoreProducts :many
SELECT p.id, p.category_id, p.name, p.price_cents, p.sale_price_cents, p.sold_out, p.is_new, p.on_sale,
    coalesce(i.r2_key, '')::text AS image_key,
    coalesce(i.widths, '{}')::int[] AS image_widths,
    coalesce(i.dominant_color, '')::text AS image_color,
    coalesce((SELECT array_agg(s.size ORDER BY s.first_id) FROM (
        SELECT v.size, min(v.id) AS first_id FROM product_variants v
        WHERE v.store_id = p.store_id AND v.product_id = p.id AND v.size <> '' AND NOT v.sold_out
        GROUP BY v.size) s), '{}')::text[] AS sizes
FROM products p
LEFT JOIN LATERAL (
    SELECT r2_key, widths, dominant_color FROM product_images
    WHERE store_id = p.store_id AND product_id = p.id ORDER BY position, id LIMIT 1
) i ON true
WHERE p.store_id = $1 AND p.active
ORDER BY p.created_at DESC, p.id DESC;

-- name: ListStoreVariants :many
SELECT v.product_id, v.size, v.color, v.sold_out FROM product_variants v
JOIN products p ON p.store_id = v.store_id AND p.id = v.product_id
WHERE v.store_id = $1 AND p.active ORDER BY v.id;

-- name: UpdateStoreWhatsapp :exec
UPDATE stores SET whatsapp = $2 WHERE id = $1;

-- name: UpdateStoreSettings :exec
UPDATE stores SET name = $2, brand_color = $3, brand_color_raw = $4, faq = $5, payment_info = $6, style = $7 WHERE id = $1;

-- name: SetStoreLogo :one
-- Devuelve el logo anterior para borrar sus archivos.
UPDATE stores s SET logo_key = @logo_key, logo_widths = @logo_widths
FROM (SELECT o.id, o.logo_key, o.logo_widths FROM stores o WHERE o.id = @id FOR UPDATE) old
WHERE s.id = old.id
RETURNING old.logo_key, old.logo_widths;

-- Pedidos. Toda query lleva store_id del Host. Cantidades en milésimas (1 pieza = 1000).

-- name: CreateOrder :one
-- Si el celular reintenta con el mismo código no se duplica: no devuelve filas.
INSERT INTO orders (store_id, code, total_cents) VALUES ($1, $2, $3)
ON CONFLICT (store_id, code) DO NOTHING
RETURNING id;

-- name: AddOrderItem :exec
INSERT INTO order_items (store_id, order_id, product_id, name, category, size, color, unit, qty, price_cents)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListOrders :many
-- shortcut: los últimos 200 de cada estado; paginar si alguna tienda los rebasa seguido.
SELECT * FROM orders WHERE store_id = $1 AND status = $2 ORDER BY created_at DESC, id DESC LIMIT 200;

-- name: ListOrderItems :many
-- stock actual del renglón (de la variante si tiene talla/color); -1 = sin inventario o producto borrado.
SELECT i.*,
    coalesce(CASE WHEN i.size = '' AND i.color = '' THEN p.stock ELSE v.stock END, -1)::int AS stock,
    coalesce(p.step, 1000)::int AS step
FROM order_items i
LEFT JOIN products p ON p.store_id = i.store_id AND p.id = i.product_id
LEFT JOIN product_variants v ON v.store_id = i.store_id AND v.product_id = i.product_id AND v.size = i.size AND v.color = i.color
WHERE i.store_id = @store_id AND i.order_id = ANY(@order_ids::bigint[])
ORDER BY i.id;

-- name: GetOrderForUpdate :one
SELECT * FROM orders WHERE store_id = $1 AND id = $2 FOR UPDATE;

-- name: SetOrderStatus :exec
UPDATE orders SET status = @status, reason = @reason,
    decided_at = CASE WHEN @status::smallint = 0 THEN NULL ELSE now() END
WHERE store_id = @store_id AND id = @id;

-- name: SetOrderTotal :exec
UPDATE orders SET total_cents = (SELECT coalesce(sum((i.qty::bigint * i.price_cents + 500) / 1000), 0) FROM order_items i WHERE i.store_id = $1 AND i.order_id = $2)::int
WHERE store_id = $1 AND id = $2;

-- name: SetOrderItemQty :execrows
UPDATE order_items SET qty = @qty::int WHERE store_id = @store_id AND order_id = @order_id AND id = @id;

-- name: DeleteOrderItem :execrows
DELETE FROM order_items WHERE store_id = @store_id AND order_id = @order_id AND id = @id;

-- name: TakeProductStock :execrows
-- Descuenta si alcanza; sin inventario (NULL) pasa sin cambios. 0 filas = no alcanza.
UPDATE products SET stock = stock - @qty::int
WHERE store_id = @store_id AND id = @id AND (stock IS NULL OR stock >= @qty::int);

-- name: TakeVariantStock :execrows
UPDATE product_variants SET stock = stock - @qty::int
WHERE store_id = @store_id AND product_id = @product_id AND size = @size AND color = @color AND (stock IS NULL OR stock >= @qty::int);

-- name: ReturnProductStock :exec
UPDATE products SET stock = stock + @qty::int WHERE store_id = @store_id AND id = @id AND stock IS NOT NULL;

-- name: ReturnVariantStock :exec
UPDATE product_variants SET stock = stock + @qty::int
WHERE store_id = @store_id AND product_id = @product_id AND size = @size AND color = @color AND stock IS NOT NULL;

-- name: CountPendingOrders :one
SELECT count(*) FROM orders WHERE store_id = $1 AND status = 0;

-- name: ExpireOrders :execrows
-- Pendientes de más de 7 días: no venta "sin respuesta". No apartan inventario, así que no hay nada que regresar.
UPDATE orders SET status = 2, reason = 5, decided_at = now() WHERE status = 0 AND created_at < now() - interval '7 days';

-- ---- Resumen de ventas (periodo [@since, @until)) ----

-- name: SalesTotals :many
SELECT status, reason, count(*)::int AS orders, coalesce(sum(total_cents), 0)::bigint AS cents
FROM orders WHERE store_id = @store_id AND created_at >= @since AND created_at < @until
GROUP BY status, reason;

-- name: SalesSeries :many
-- @bucket: 'day' o 'month', en hora de la Ciudad de México.
SELECT date_trunc(@bucket::text, created_at AT TIME ZONE 'America/Mexico_City')::date AS day,
    coalesce(sum(total_cents) FILTER (WHERE status = 1), 0)::bigint AS sold,
    coalesce(sum(total_cents) FILTER (WHERE status = 2), 0)::bigint AS lost
FROM orders WHERE store_id = @store_id AND created_at >= @since AND created_at < @until AND status > 0
GROUP BY 1 ORDER BY 1;

-- name: SalesItems :many
-- Por producto + talla + color y estado; el handler arma lo más vendido, lo perdido y las categorías.
SELECT i.name, i.category, i.size, i.color, i.unit, o.status,
    sum(i.qty)::bigint AS qty, sum((i.qty::bigint * i.price_cents + 500) / 1000)::bigint AS cents
FROM order_items i JOIN orders o ON o.store_id = i.store_id AND o.id = i.order_id
WHERE o.store_id = @store_id AND o.created_at >= @since AND o.created_at < @until AND o.status > 0
GROUP BY 1, 2, 3, 4, 5, 6;

-- name: SalesCSV :many
SELECT o.code, o.status, o.reason, o.created_at, i.name, i.category, i.size, i.color, i.unit, i.qty, i.price_cents
FROM order_items i JOIN orders o ON o.store_id = i.store_id AND o.id = i.order_id
WHERE o.store_id = @store_id AND o.created_at >= @since AND o.created_at < @until
ORDER BY o.created_at, o.id, i.id;

-- name: LowStock :many
-- Lo que lleva inventario y tiene 3 pasos o menos (0.75 kg si se vende de 0.25 en 0.25).
SELECT p.id, p.name, '' AS size, '' AS color, p.unit, p.stock::int AS stock
FROM products p
WHERE p.store_id = $1 AND p.stock IS NOT NULL AND p.stock <= p.step * 3
    AND NOT EXISTS (SELECT 1 FROM product_variants v WHERE v.store_id = p.store_id AND v.product_id = p.id)
UNION ALL
SELECT p.id, p.name, v.size, v.color, p.unit, v.stock::int
FROM product_variants v JOIN products p ON p.store_id = v.store_id AND p.id = v.product_id
WHERE v.store_id = $1 AND v.stock IS NOT NULL AND v.stock <= p.step * 3
ORDER BY 6, 2 LIMIT 50;

-- ---- Para validar un pedido nuevo contra la base ----

-- name: OrderProducts :many
SELECT p.id, p.name, coalesce(c.name, '')::text AS category, p.price_cents, p.sale_price_cents, p.on_sale, p.sold_out, p.unit, p.step, p.stock
FROM products p LEFT JOIN categories c ON c.store_id = p.store_id AND c.id = p.category_id
WHERE p.store_id = @store_id AND p.id = ANY(@ids::bigint[]) AND p.active;

-- name: OrderVariants :many
SELECT product_id, size, color, sold_out, stock FROM product_variants
WHERE store_id = @store_id AND product_id = ANY(@ids::bigint[]);

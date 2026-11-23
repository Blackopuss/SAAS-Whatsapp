-- Fechas en hora de México: "hoy" de la vendedora, no de UTC.

-- name: Today :one
SELECT (now() AT TIME ZONE 'America/Mexico_City')::date AS today;

-- name: GetSubscription :one
SELECT * FROM subscriptions WHERE store_id = $1;

-- name: UpdateSubscriptionPlan :exec
UPDATE subscriptions SET is_founder = $2, price_cents = $3 WHERE store_id = $1;

-- name: LockSubscription :one
SELECT * FROM subscriptions WHERE store_id = $1 FOR UPDATE;

-- name: ApplyPayment :exec
UPDATE subscriptions SET paid_until = $2, streak_months = $3 WHERE store_id = $1;

-- name: CreatePayment :exec
INSERT INTO payments (store_id, amount_cents, months, paid_on, note, on_time, prev_paid_until, new_paid_until, admin_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListPayments :many
SELECT * FROM payments WHERE store_id = $1 ORDER BY created_at DESC LIMIT 50;

-- name: SetStoreStatus :execrows
UPDATE stores SET status = @status WHERE id = @id AND status <> @status;

-- name: SuspendOverdue :many
-- Día 5 de atraso: se pausa. Solo tiendas activas con fecha de pago; las recién dadas de alta (NULL) no.
UPDATE stores s SET status = 'suspended'
FROM subscriptions sub
WHERE sub.store_id = s.id AND s.status = 'active' AND sub.paid_until IS NOT NULL
  AND sub.paid_until < (now() AT TIME ZONE 'America/Mexico_City')::date - 5
RETURNING s.id, s.slug;

-- name: AddAudit :exec
INSERT INTO admin_audit (admin_id, store_id, action, detail) VALUES ($1, $2, $3, $4);

-- name: ListAudit :many
SELECT a.id, a.created_at, a.action, a.detail, coalesce(ad.email, 'sistema')::text AS who, coalesce(s.slug, '')::text AS store
FROM admin_audit a
LEFT JOIN admins ad ON ad.id = a.admin_id
LEFT JOIN stores s ON s.id = a.store_id
ORDER BY a.created_at DESC LIMIT 200;

-- name: GetStoreAdmin :one
SELECT s.id, s.slug, s.name, s.status, s.whatsapp, sub.paid_until, sub.streak_months, sub.is_founder, sub.price_cents
FROM stores s JOIN subscriptions sub ON sub.store_id = s.id WHERE s.id = $1;

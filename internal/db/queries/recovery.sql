-- name: GetUserByEmail :one
SELECT id FROM users WHERE store_id = $1 AND email = $2;

-- name: CreatePasswordReset :exec
INSERT INTO password_resets (token_hash, store_id, user_id, expires_at) VALUES ($1, $2, $3, $4);

-- name: CheckPasswordReset :one
SELECT user_id FROM password_resets
WHERE token_hash = $1 AND store_id = $2 AND used_at IS NULL AND expires_at > now();

-- name: UsePasswordReset :one
-- Un solo uso: dos peticiones simultáneas con el mismo token no pueden ganar ambas.
UPDATE password_resets SET used_at = now()
WHERE token_hash = $1 AND store_id = $2 AND used_at IS NULL AND expires_at > now()
RETURNING user_id;

-- name: SetUserPassword :exec
UPDATE users SET password_hash = $3, password_changed_at = now() WHERE store_id = $1 AND id = $2;

-- name: GetUserPasswordChangedAt :one
SELECT password_changed_at FROM users WHERE store_id = $1 AND id = $2;

-- name: GetAdminByEmail :one
SELECT * FROM admins WHERE email = $1;

-- name: GetAdmin :one
SELECT * FROM admins WHERE id = $1;

-- name: UseTOTPStep :execrows
-- Solo acepta un paso más nuevo que el último usado: un código interceptado no se puede repetir.
UPDATE admins SET last_totp_step = $2 WHERE id = $1 AND last_totp_step < $2;

-- name: CreateAdmin :one
INSERT INTO admins (email, password_hash, totp_secret) VALUES ($1, $2, $3) RETURNING id;

-- name: ListStoresAdmin :many
SELECT s.id, s.slug, s.name, s.status, s.created_at, sub.paid_until, sub.streak_months, sub.is_founder,
    (SELECT count(*) FROM products p WHERE p.store_id = s.id) AS products
FROM stores s JOIN subscriptions sub ON sub.store_id = s.id
ORDER BY sub.paid_until NULLS FIRST, s.created_at DESC;

-- name: GetStoreOwner :one
SELECT u.id AS user_id, s.slug, s.name, s.whatsapp FROM stores s JOIN users u ON u.store_id = s.id
WHERE s.id = $1 ORDER BY u.id LIMIT 1;

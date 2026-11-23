-- name: GetUserForLogin :one
-- store_id sale del tenant (Host), nunca del formulario: las credenciales de luz no sirven en ana.
SELECT id, store_id, password_hash FROM users WHERE store_id = $1 AND email = $2;

-- name: CreateStore :one
-- Toda tienda nace con su suscripción (sin pagar aún), así ningún JOIN la pierde.
WITH s AS (INSERT INTO stores (slug, name) VALUES ($1, $2) RETURNING *),
     sub AS (INSERT INTO subscriptions (store_id) SELECT id FROM s)
SELECT * FROM s;

-- name: CreateUser :one
INSERT INTO users (store_id, email, password_hash) VALUES ($1, $2, $3) RETURNING id;

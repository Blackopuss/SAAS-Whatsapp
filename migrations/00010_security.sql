-- +goose Up
-- Auditoría de seguridad (2026-11-22).
-- credentials_changed_at: al cambiar contraseña/TOTP de un admin (createadmin -reset) sus sesiones abiertas dejan de servir.
ALTER TABLE admins ADD COLUMN credentials_changed_at timestamptz NOT NULL DEFAULT now();

-- La bitácora solo crece: la app puede escribir renglones nuevos, pero no editarlos ni borrarlos.
REVOKE UPDATE, DELETE, TRUNCATE ON admin_audit FROM tiendas_app;

-- +goose Down
GRANT UPDATE, DELETE ON admin_audit TO tiendas_app;
ALTER TABLE admins DROP COLUMN credentials_changed_at;

-- +goose Up
-- Al cambiar la contraseña, las sesiones iniciadas antes dejan de servir.
ALTER TABLE users ADD COLUMN password_changed_at timestamptz NOT NULL DEFAULT now();

-- Solo se guarda el hash del token: si se filtra la tabla, los enlaces no sirven.
CREATE TABLE password_resets (
    token_hash bytea PRIMARY KEY,
    store_id   bigint NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);
CREATE INDEX password_resets_user_idx ON password_resets(user_id);

-- Superadmin: vive en admin.<dominio>, con su propia cookie y TOTP obligatorio.
CREATE TABLE admins (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email          citext NOT NULL UNIQUE,
    password_hash  text NOT NULL,
    totp_secret    bytea NOT NULL, -- cifrado con TOTP_KEY (AES-GCM)
    last_totp_step bigint NOT NULL DEFAULT 0, -- impide reusar un mismo código
    created_at     timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE admins;
DROP TABLE password_resets;
ALTER TABLE users DROP COLUMN password_changed_at;

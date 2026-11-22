-- +goose Up
-- style: estilo visual de la tienda según su giro (todas comparten el mismo marcado; cambia el CSS).
ALTER TABLE stores ADD COLUMN style text NOT NULL DEFAULT 'vitrina'
    CHECK (style IN ('vitrina', 'boutique', 'barberia', 'carniceria', 'abarrotes', 'papeleria'));

-- +goose Down
ALTER TABLE stores DROP COLUMN style;

-- +goose Up
-- r2_key guarda la base (stores/{id}/{uuid}); cada versión vive en {base}-{ancho}.{webp|avif}.
ALTER TABLE product_images ADD COLUMN widths int[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE product_images DROP COLUMN widths;

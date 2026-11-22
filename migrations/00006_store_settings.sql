-- +goose Up
-- brand_color: el color que se usa (ya ajustado por contraste); brand_color_raw: el que eligió la vendedora.
UPDATE stores SET brand_color = '';
ALTER TABLE stores ALTER COLUMN brand_color SET DEFAULT '',
    ADD CONSTRAINT stores_brand_color_hex CHECK (brand_color = '' OR brand_color ~ '^#[0-9a-f]{6}$'),
    ADD COLUMN brand_color_raw text NOT NULL DEFAULT '' CHECK (brand_color_raw = '' OR brand_color_raw ~ '^#[0-9a-f]{6}$'),
    ADD COLUMN logo_key text NOT NULL DEFAULT '',
    ADD COLUMN logo_widths int[] NOT NULL DEFAULT '{}',
    ADD COLUMN faq text NOT NULL DEFAULT '' CHECK (length(faq) <= 4000),
    ADD COLUMN payment_info text NOT NULL DEFAULT '' CHECK (length(payment_info) <= 1000);

-- +goose Down
ALTER TABLE stores DROP COLUMN payment_info, DROP COLUMN faq, DROP COLUMN logo_widths, DROP COLUMN logo_key,
    DROP COLUMN brand_color_raw, DROP CONSTRAINT stores_brand_color_hex;
ALTER TABLE stores ALTER COLUMN brand_color SET DEFAULT 'oklch(0.62 0.19 350)';

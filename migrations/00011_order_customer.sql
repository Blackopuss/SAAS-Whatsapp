-- +goose Up
-- Nombre que la clienta escribe en el carrito para su pedido (2026-11-23). Los pedidos de antes quedan con ''
-- (en Ventas: "Cliente sin nombre registrado"); no se toca ningún otro dato. Solo el nombre: nada de teléfono ni dirección.
ALTER TABLE orders ADD COLUMN customer text NOT NULL DEFAULT '' CHECK (char_length(customer) <= 60);

-- +goose Down
ALTER TABLE orders DROP COLUMN customer;

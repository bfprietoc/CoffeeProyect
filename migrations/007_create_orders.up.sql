CREATE TABLE orders (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID        NOT NULL REFERENCES users(id),
    -- shipping address snapshot (preserved even if user later changes/deletes address)
    shipping_street     TEXT        NOT NULL,
    shipping_city       TEXT        NOT NULL,
    shipping_department TEXT        NOT NULL,
    shipping_country    TEXT        NOT NULL DEFAULT 'CO',
    total_cents         INT         NOT NULL,
    currency            TEXT        NOT NULL DEFAULT 'COP',
    status              TEXT        NOT NULL DEFAULT 'confirmed',
    tracking_number     TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE order_items (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    coffee_id        UUID NOT NULL REFERENCES coffees(id),
    coffee_name      TEXT NOT NULL,
    quantity         INT  NOT NULL CHECK (quantity > 0),
    unit_price_cents INT  NOT NULL
);

CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_status  ON orders(status);

CREATE TRIGGER orders_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE stock_movements (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    coffee_id    UUID        NOT NULL REFERENCES coffees(id),
    delta        INT         NOT NULL,
    result_stock INT         NOT NULL,
    note         TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_stock_movements_coffee_id ON stock_movements(coffee_id);

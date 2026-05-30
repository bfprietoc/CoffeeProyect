CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE coffees (
    id             UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    name           TEXT    NOT NULL,
    process        TEXT    NOT NULL,
    roast_level    TEXT    NOT NULL,
    tasting_notes  JSONB   NOT NULL DEFAULT '[]',
    description    TEXT    NOT NULL DEFAULT '',
    farm_id        UUID    REFERENCES farms(id),
    bag_size_grams INT     NOT NULL,
    price_cents    INT     NOT NULL,
    currency       TEXT    NOT NULL DEFAULT 'COP',
    stock_bags     INT     NOT NULL DEFAULT 0,
    available      BOOLEAN GENERATED ALWAYS AS (stock_bags > 0) STORED,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER coffees_updated_at
BEFORE UPDATE ON coffees
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

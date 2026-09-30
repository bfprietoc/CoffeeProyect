CREATE TABLE farms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    country     TEXT NOT NULL,
    region      TEXT NOT NULL,
    producer_id UUID REFERENCES producers(id)
);

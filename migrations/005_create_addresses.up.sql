CREATE TABLE addresses (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    street     TEXT NOT NULL,
    city       TEXT NOT NULL,
    department TEXT NOT NULL,
    country    TEXT NOT NULL DEFAULT 'CO',
    is_default BOOLEAN NOT NULL DEFAULT false
);

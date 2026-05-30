CREATE TABLE waitlist (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    coffee_id  UUID        NOT NULL REFERENCES coffees(id),
    user_id    UUID        REFERENCES users(id),
    email      TEXT        NOT NULL,
    notified   BOOLEAN     NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(coffee_id, email)
);

CREATE INDEX idx_waitlist_coffee_pending ON waitlist(coffee_id) WHERE notified = false;

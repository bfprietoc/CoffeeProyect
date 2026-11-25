-- Payment lifecycle (Wompi Web Checkout).
-- Reference is unique per attempt: each payment attempt (including retries)
-- gets a fresh reference (order.id + unix timestamp), and the webhook maps
-- the transaction back to the order via payment_reference.
ALTER TABLE orders
    ADD COLUMN payment_status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (payment_status IN ('pending', 'paid', 'failed')),
    ADD COLUMN payment_provider TEXT,
    ADD COLUMN payment_reference TEXT,
    ADD COLUMN payment_transaction_id TEXT,
    ADD COLUMN paid_at TIMESTAMPTZ;

-- Orders created before payment tracking existed: the shop collected payment
-- outside the transactional flow, so treat them as paid — otherwise every
-- existing order could never be moved to "processing".
UPDATE orders SET payment_status = 'paid' WHERE payment_status = 'pending';

CREATE UNIQUE INDEX idx_orders_payment_reference
    ON orders(payment_reference)
    WHERE payment_reference IS NOT NULL;

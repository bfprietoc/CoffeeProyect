DROP INDEX IF EXISTS idx_orders_payment_reference;

ALTER TABLE orders
    DROP COLUMN payment_reference,
    DROP COLUMN payment_transaction_id,
    DROP COLUMN paid_at,
    DROP COLUMN payment_provider,
    DROP COLUMN payment_status;

CREATE TABLE IF NOT EXISTS transactions (
    id              VARCHAR(255) PRIMARY KEY,
    sender_id       VARCHAR(255) NOT NULL,
    receiver_id     VARCHAR(255) NOT NULL,
    amount          DECIMAL(20,2) NOT NULL,
    currency        VARCHAR(10) NOT NULL,
    status          VARCHAR(30) NOT NULL DEFAULT 'created',
    device_id       VARCHAR(255),
    ip              VARCHAR(50),
    lat             DOUBLE PRECISION,
    lng             DOUBLE PRECISION,
    payment_method  VARCHAR(20),
    fraud_decision  VARCHAR(20),
    fraud_score     INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_sender ON transactions(sender_id);
CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status);

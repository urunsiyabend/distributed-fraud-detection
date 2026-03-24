CREATE TABLE outbox (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type   VARCHAR(100) NOT NULL,
    payload      JSONB NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retry_count  INT NOT NULL DEFAULT 0,
    last_error   TEXT
);

CREATE INDEX idx_outbox_status ON outbox(status) WHERE status = 'pending';

CREATE TABLE assessments (
    transaction_id VARCHAR(255) PRIMARY KEY,
    decision       VARCHAR(20) NOT NULL,
    risk_score     INT NOT NULL,
    rule_results   JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS config (
    key   VARCHAR(255) PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS known_devices (
    sender_id VARCHAR(255) NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (sender_id, device_id)
);

CREATE TABLE IF NOT EXISTS outbox (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type   VARCHAR(100) NOT NULL,
    payload      JSONB NOT NULL,
    status       VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retry_count  INT NOT NULL DEFAULT 0,
    last_error   TEXT
);

CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox(status) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS assessments (
    transaction_id VARCHAR(255) PRIMARY KEY,
    decision       VARCHAR(20) NOT NULL,
    risk_score     INT NOT NULL,
    rule_results   JSONB NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Rule config
INSERT INTO config (key, value) VALUES
    ('rules.velocity.max_count', '10'),
    ('rules.velocity.window_minutes', '5'),
    ('rules.velocity.score', '50'),
    ('rules.velocity.fallback_score', '25'),
    ('rules.amount.threshold', '1000'),
    ('rules.amount.score', '40'),
    ('rules.amount.critical_score', '80'),
    ('rules.amount.fallback_score', '20'),
    ('rules.device.missing_score', '30'),
    ('rules.device.unknown_score', '35'),
    ('rules.device.fallback_score', '15'),
    ('rules.location.max_distance_km', '500'),
    ('rules.location.score', '60'),
    ('rules.location.fallback_score', '20'),
    ('rules.pattern.score', '45'),
    ('rules.pattern.fallback_score', '15')
ON CONFLICT (key) DO NOTHING;

-- Seed known devices for 100 test users
INSERT INTO known_devices (sender_id, device_id)
SELECT 'user-' || i, 'known-device-' || i
FROM generate_series(1, 100) AS i
ON CONFLICT DO NOTHING;

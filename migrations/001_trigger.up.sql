CREATE TABLE IF NOT EXISTS signal_sources (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL UNIQUE,
    type VARCHAR(30) NOT NULL,
    endpoint_url TEXT,
    auth JSONB,
    polling_interval_sec INT DEFAULT 300,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS trigger_rules (
    id BIGSERIAL PRIMARY KEY,
    advertiser_id BIGINT,
    name VARCHAR(255) NOT NULL,
    campaign_id BIGINT,
    conditions JSONB NOT NULL,
    actions JSONB NOT NULL,
    cooldown_sec INT DEFAULT 3600,
    last_fired_at TIMESTAMPTZ,
    fire_count BIGINT DEFAULT 0,
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS trigger_firings (
    id BIGSERIAL PRIMARY KEY,
    rule_id BIGINT REFERENCES trigger_rules(id),
    signal_snapshot JSONB,
    actions_executed JSONB,
    fired_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS signal_snapshots (
    id BIGSERIAL PRIMARY KEY,
    source_id BIGINT REFERENCES signal_sources(id),
    data JSONB NOT NULL,
    received_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_signal_time ON signal_snapshots(source_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_trigger_firings_rule ON trigger_firings(rule_id, fired_at DESC);
CREATE INDEX IF NOT EXISTS idx_trigger_rules_status ON trigger_rules(status);
CREATE INDEX IF NOT EXISTS idx_signal_sources_status ON signal_sources(status);

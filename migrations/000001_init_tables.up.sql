CREATE TABLE IF NOT EXISTS targets (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'GET',
    expected_status INT NOT NULL DEFAULT 200,
    interval_seconds INT NOT NULL DEFAULT 60,
    timeout_seconds INT NOT NULL DEFAULT 5,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS check_result (
    id SERIAL PRIMARY KEY,
    target_id INT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    response_time_ms INT,
    status_code INT,
    error_message TEXT,
    checked_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS incidents (
    id SERIAL PRIMARY KEY,
    target_id INT NOT NULL REFERENCES targets(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('open', 'resolved')),
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMP NULL,
    last_error TEXT NULL,
    checks_failed INT NOT NULL DEFAULT 1
);

CREATE UNIQUE INDEX unique_open_incident_per_target
ON incidents(target_id)
WHERE status = 'open';

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
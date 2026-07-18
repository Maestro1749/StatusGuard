ALTER TABLE targets ADD COLUMN next_check_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE targets SET next_check_at = now() WHERE next_check_at IS NULL;

CREATE INDEX idx_targets_due_check ON targets (next_check_at) WHERE enabled = true;
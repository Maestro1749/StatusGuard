DROP INDEX IF EXISTS idx_incidents_status;

DROP INDEX IF EXISTS idx_check_result_target_checked_at;

ALTER TABLE incidents
DROP CONSTRAINT IF EXISTS incidents_checks_failed_check;

ALTER TABLE check_result
DROP CONSTRAINT IF EXISTS check_result_status_code_check;

ALTER TABLE check_result
DROP CONSTRAINT IF EXISTS check_result_response_time_ms_check;

ALTER TABLE check_result
DROP CONSTRAINT IF EXISTS check_result_status_check;

ALTER TABLE targets
DROP CONSTRAINT IF EXISTS targets_timeout_seconds_check;

ALTER TABLE targets
DROP CONSTRAINT IF EXISTS targets_interval_seconds_check;

ALTER TABLE targets
DROP CONSTRAINT IF EXISTS targets_expected_status_check;

ALTER TABLE targets
DROP CONSTRAINT IF EXISTS targets_method_check;
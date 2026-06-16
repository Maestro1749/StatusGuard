UPDATE targets
SET method = 'GET'
WHERE method IS NULL OR method <> 'GET';

UPDATE targets
SET expected_status = 200
WHERE expected_status < 100 OR expected_status > 599;

UPDATE targets
SET interval_seconds = 60
WHERE interval_seconds < 10;

UPDATE targets
SET timeout_seconds = 5
WHERE timeout_seconds < 1 OR timeout_seconds > 30;

UPDATE check_result
SET status = 'DOWN'
WHERE status NOT IN ('UP', 'DOWN');

UPDATE check_result
SET response_time_ms = 0
WHERE response_time_ms IS NOT NULL AND response_time_ms < 0;

UPDATE check_result
SET status_code = NULL
WHERE status_code IS NOT NULL AND status_code NOT BETWEEN 100 AND 599;

UPDATE incidents
SET checks_failed = 1
WHERE checks_failed < 1;

ALTER TABLE targets
ADD CONSTRAINT targets_method_check
CHECK (method IN ('GET'));

ALTER TABLE targets
ADD CONSTRAINT targets_expected_status_check
CHECK (expected_status BETWEEN 100 AND 599);

ALTER TABLE targets
ADD CONSTRAINT targets_interval_seconds_check
CHECK (interval_seconds >= 10);

ALTER TABLE targets
ADD CONSTRAINT targets_timeout_seconds_check
CHECK (timeout_seconds BETWEEN 1 AND 30);

ALTER TABLE check_result
ADD CONSTRAINT check_result_status_check
CHECK (status IN ('UP', 'DOWN'));

ALTER TABLE check_result
ADD CONSTRAINT check_result_response_time_ms_check
CHECK (response_time_ms >= 0);

ALTER TABLE check_result
ADD CONSTRAINT check_result_status_code_check
CHECK (status_code IS NULL OR status_code BETWEEN 100 AND 599);

ALTER TABLE incidents
ADD CONSTRAINT incidents_check_failed_check
CHECK (checks_failed >= 1);

CREATE INDEX idx_check_result_target_checked_at
ON check_result(target_id, checked_at DESC);

CREATE INDEX idx_incidents_status
ON incidents(status);
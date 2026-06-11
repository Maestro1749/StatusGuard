package transport

type CreateTargetRequest struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	Method          string `json:"method"`
	ExpectedStatus  int    `json:"expected_status"`
	IntervalSeconds int    `json:"interval_seconds"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
}

type UpdateTargetRequest struct {
	Name            *string `json:"name"`
	URL             *string `json:"url"`
	Method          *string `json:"method"`
	ExpectedStatus  *int    `json:"expected_status"`
	IntervalSeconds *int    `json:"interval_seconds"`
	TimeoutSeconds  *int    `json:"timeout_seconds"`
	Enabled         *bool   `json:"enabled"`
}

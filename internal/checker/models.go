package checker

import (
	"errors"
	"time"
)

type Result struct {
	ID             int       `json:"id"`
	TargetID       int       `json:"target_id"`
	Status         string    `json:"status"`
	ResponseTimeMs int       `json:"response_time_ms"`
	HTTPStatus     *int      `json:"http_status"`
	ErrorMessage   *string   `json:"error_message"`
	CheckedAt      time.Time `json:"checked_at"`
}

var (
	StatusUp   = "UP"
	StatusDown = "DOWN"

	ErrInvalidID       = errors.New("input target id is not vaild")
	ErrInternalServer  = errors.New("internal server error")
	ErrTimeout         = errors.New("response timeout")
	ErrResultsNotFound = errors.New("results with this target id not found")
)

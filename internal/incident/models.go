package incident

import (
	"errors"
	"time"
)

type Status string

const (
	StatusOpen     Status = "open"
	StatusResolved Status = "resolved"
)

type Incident struct {
	ID           int        `json:"id"`
	TargetID     int        `json:"target_id"`
	Status       Status     `json:"status"`
	StartedAt    time.Time  `json:"started_at"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	LastError    *string    `json:"last_error"`
	ChecksFailed int        `json:"checks_failed"`
}

var (
	ErrTimeout         = errors.New("response timeout")
	ErrNotFound        = errors.New("incidents not found")
	ErrInternalServer  = errors.New("internal server error")
	ErrInvalidTargetID = errors.New("invalid target id")
)

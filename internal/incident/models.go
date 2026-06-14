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
	ID           int
	TargetID     int
	Status       Status
	StartedAt    time.Time
	ResolvedAt   *time.Time
	LastError    *string
	ChecksFailed int
}

var (
	ErrTimeout         = errors.New("response timeout")
	ErrNotFound        = errors.New("incidents not found")
	ErrInternalServer  = errors.New("internal server error")
	ErrInvalidTargetID = errors.New("invalid target id")
)

package incident

import "time"

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

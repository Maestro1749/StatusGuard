package checker

import (
	"errors"
	"time"
)

type Result struct {
	ID             int
	TargetID       int
	Status         string
	ResponseTimeMs int
	HTTPStatus     *int
	ErrorMessage   *string
	CheckedAt      time.Time
}

var (
	StatusUp   = "UP"
	StatusDown = "DOWN"

	ErrInvalidID      = errors.New("input target id is not vaild")
	ErrInternalServer = errors.New("internal server error")
	ErrTimeout        = errors.New("response timeout")
)

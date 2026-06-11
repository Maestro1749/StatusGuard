package monitor

import (
	"errors"
	"time"
)

type Target struct {
	ID              int
	Name            string
	URL             string
	Method          string
	ExpectedStatus  int
	IntervalSeconds int
	TimeoutSeconds  int
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

var (
	ErrEmptyName             = errors.New("target name is empty")
	ErrInvalidURL            = errors.New("target URL is not valid")
	ErrInvalidInterval       = errors.New("interval seconds is not valid. Expected value >= 10")
	ErrInvalidTimeout        = errors.New("timeout seconds is not vaild. Expected value <= 30 and >= 1")
	ErrInvalidMethod         = errors.New("target method is not valid")
	ErrInvalidExpectedStatus = errors.New("expected status is not valid")

	ErrInvalidID      = errors.New("input id is not valid")
	ErrTargetNotFound = errors.New("target with such id not found")

	ErrTimeout = errors.New("response timeout")

	ErrInternalServer = errors.New("internal server error")
)

type UpdateTargetInput struct {
	ID              int
	Name            *string
	URL             *string
	Method          *string
	ExpectedStatus  *int
	IntervalSeconds *int
	TimeoutSeconds  *int
	Enabled         *bool
}

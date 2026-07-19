package monitor

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"
)

type MonitorRepository interface {
	CreateTarget(ctx context.Context, target Target) (*Target, error)
	DeleteTarget(ctx context.Context, id int) error
	GetAllTargets(ctx context.Context) ([]Target, error)
	GetByID(ctx context.Context, id int) (*Target, error)
	UpdateTarget(ctx context.Context, target Target) (*Target, error)
}

type MonitorService struct {
	repo   MonitorRepository
	logger *zap.Logger
}

func NewMonitorService(repo MonitorRepository, logger *zap.Logger) *MonitorService {
	return &MonitorService{repo: repo, logger: logger}
}

const (
	MinCheckInterval = 10
	MaxCheckInterval = 24 * 60 * 60

	MinCheckTimeout = 1
	MaxCheckTimeout = 30
)

func (s *MonitorService) CreateTarget(
	ctx context.Context,
	name string,
	urlTarget string,
	method string,
	expectedStatus int,
	intervalSeconds int,
	timeoutSeconds int,
) (*Target, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ErrEmptyName
	}

	parsed, err := url.ParseRequestURI(urlTarget)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, ErrInvalidURL
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, ErrInvalidURL
	}

	if method == "" {
		method = http.MethodGet
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method != http.MethodGet {
		return nil, ErrInvalidMethod
	}

	if expectedStatus < 100 || expectedStatus > 599 {
		expectedStatus = 200
	}

	if intervalSeconds < MinCheckInterval || intervalSeconds > MaxCheckInterval {
		return nil, ErrInvalidInterval
	}

	if timeoutSeconds < MinCheckTimeout || timeoutSeconds > MaxCheckTimeout || timeoutSeconds*2 > intervalSeconds {
		return nil, ErrInvalidTimeout
	}

	target := Target{
		Name:            name,
		URL:             urlTarget,
		Method:          method,
		ExpectedStatus:  expectedStatus,
		IntervalSeconds: intervalSeconds,
		TimeoutSeconds:  timeoutSeconds,
		Enabled:         true,
	}

	return s.repo.CreateTarget(ctx, target)
}

func (s *MonitorService) DeleteTarget(ctx context.Context, id int) error {
	if id <= 0 {
		return ErrInvalidID
	}

	return s.repo.DeleteTarget(ctx, id)
}

func (s *MonitorService) GetTarget(ctx context.Context, id int) (*Target, error) {
	if id <= 0 {
		return nil, ErrInvalidID
	}

	return s.repo.GetByID(ctx, id)
}

func (s *MonitorService) GetAllTargets(ctx context.Context) ([]Target, error) {
	return s.repo.GetAllTargets(ctx)
}

func (s *MonitorService) UpdateTarget(ctx context.Context, input UpdateTargetInput) (*Target, error) {
	target, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, ErrEmptyName
		}
		target.Name = name
	}

	if input.URL != nil {
		if _, err := url.ParseRequestURI(*input.URL); err != nil {
			return nil, ErrInvalidURL
		}
		target.URL = *input.URL
	}

	if input.Method != nil {
		method := strings.ToUpper(strings.TrimSpace(*input.Method))
		if method != http.MethodGet {
			return nil, ErrInvalidMethod
		}
		target.Method = method
	}

	if input.ExpectedStatus != nil {
		if *input.ExpectedStatus < 100 || *input.ExpectedStatus > 599 {
			return nil, ErrInvalidExpectedStatus
		}
		target.ExpectedStatus = *input.ExpectedStatus
	}

	if input.IntervalSeconds != nil {
		if *input.IntervalSeconds < MinCheckInterval || *input.IntervalSeconds > MaxCheckInterval {
			return nil, ErrInvalidInterval
		}
		target.IntervalSeconds = *input.IntervalSeconds
	}

	if input.TimeoutSeconds != nil {
		if *input.TimeoutSeconds < MinCheckTimeout || *input.TimeoutSeconds > MaxCheckTimeout {
			return nil, ErrInvalidTimeout
		}
		target.TimeoutSeconds = *input.TimeoutSeconds
	}

	if input.Enabled != nil {
		target.Enabled = *input.Enabled
	}

	if target.TimeoutSeconds*2 > target.IntervalSeconds {
		return nil, ErrInvalidTimeout
	}

	return s.repo.UpdateTarget(ctx, *target)
}

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
	GetTarget(ctx context.Context, target Target) (*Target, error)
	GetAllTargets(ctx context.Context) ([]Target, error)
	GetByID(ctx context.Context, id int) (*Target, error)
	UpdateTarget(ctx context.Context, target Target) (*Target, error)
	GetAllActive(ctx context.Context) ([]Target, error)
}

type MonitorService struct {
	repo   MonitorRepository
	logger *zap.Logger
}

func NewMonitorService(repo MonitorRepository, logger *zap.Logger) *MonitorService {
	return &MonitorService{repo: repo, logger: logger}
}

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

	if intervalSeconds == 0 {
		intervalSeconds = 60
	}

	if timeoutSeconds == 0 {
		timeoutSeconds = 5
	}

	if expectedStatus < 100 || expectedStatus > 599 {
		expectedStatus = 200
	}

	if intervalSeconds < 10 {
		return nil, ErrInvalidInterval
	}

	if timeoutSeconds < 1 || timeoutSeconds > 30 {
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

	target := Target{
		ID: id,
	}

	return s.repo.GetTarget(ctx, target)
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
		if *input.IntervalSeconds < 10 {
			return nil, ErrInvalidInterval
		}
		target.IntervalSeconds = *input.IntervalSeconds
	}

	if input.TimeoutSeconds != nil {
		if *input.TimeoutSeconds < 1 || *input.TimeoutSeconds > 30 {
			return nil, ErrInvalidTimeout
		}
		target.TimeoutSeconds = *input.TimeoutSeconds
	}

	if input.Enabled != nil {
		target.Enabled = *input.Enabled
	}

	return s.repo.UpdateTarget(ctx, *target)
}

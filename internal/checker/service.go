package checker

import (
	"StatusGuard/internal/monitor"
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type TargetProvider interface {
	GetByID(ctx context.Context, id int) (*monitor.Target, error)
}

type CheckerRepository interface {
	Save(ctx context.Context, result Result) (*Result, error)
	GetByTargetID(ctx context.Context, targetID int, limit int) ([]Result, error)
}

type rateLimiter interface {
	Allow(ctx context.Context, key string) (bool, time.Duration, error)
}

type CheckerService struct {
	targetsProvider TargetProvider
	checkerRepo     CheckerRepository
	rateLimiter     rateLimiter
	client          *http.Client
	logger          *zap.Logger
}

func NewCheckerService(targetsProvider TargetProvider, checkerRepo CheckerRepository, rateLimiter rateLimiter, httpClient *http.Client, logger *zap.Logger) *CheckerService {
	return &CheckerService{
		targetsProvider: targetsProvider,
		checkerRepo:     checkerRepo,
		rateLimiter:     rateLimiter,
		client:          httpClient,
		logger:          logger,
	}
}

func (s *CheckerService) CheckManually(ctx context.Context, id int) (*Result, *time.Duration, error) {
	target, err := s.targetsProvider.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	key := fmt.Sprintf("statusguard:manual-check:%d", target.ID)

	allowed, retryAfter, err := s.rateLimiter.Allow(ctx, key)
	if err != nil {
		return nil, nil, err
	}

	if !allowed {
		return nil, &retryAfter, ErrTooManyRequests
	}

	result := s.executeCheck(ctx, target)

	savedResult, err := s.checkerRepo.Save(ctx, result)
	if err != nil {
		return nil, nil, err
	}

	return savedResult, nil, nil
}

func (s *CheckerService) CheckScheduled(ctx context.Context, target monitor.Target) Result {
	result := s.executeCheck(ctx, &target)

	if _, err := s.checkerRepo.Save(ctx, result); err != nil {
		s.logger.Error("failed to save check result",
			zap.Int("target_id", target.ID),
			zap.Error(err),
		)
	}

	return result
}

func (s *CheckerService) executeCheck(ctx context.Context, target *monitor.Target) Result {
	start := time.Now()

	checkCtx, cancel := context.WithTimeout(
		ctx,
		time.Duration(target.TimeoutSeconds)*time.Second,
	)
	defer cancel()

	req, err := http.NewRequestWithContext(
		checkCtx,
		target.Method,
		target.URL,
		nil,
	)
	if err != nil {
		msg := err.Error()

		return Result{
			TargetID:       target.ID,
			Status:         StatusDown,
			ResponseTimeMs: int(time.Since(start).Milliseconds()),
			ErrorMessage:   &msg,
			CheckedAt:      time.Now(),
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		msg := err.Error()

		return Result{
			TargetID:       target.ID,
			Status:         StatusDown,
			ResponseTimeMs: int(time.Since(start).Milliseconds()),
			ErrorMessage:   &msg,
			CheckedAt:      time.Now(),
		}
	}
	defer resp.Body.Close()

	httpStatus := resp.StatusCode

	status := StatusUp
	var errMsg *string

	if resp.StatusCode != target.ExpectedStatus {
		status = StatusDown
		msg := fmt.Sprintf("expected status %d, got %d", target.ExpectedStatus, resp.StatusCode)
		errMsg = &msg
	}

	return Result{
		TargetID:       target.ID,
		Status:         status,
		HTTPStatus:     &httpStatus,
		ResponseTimeMs: int(time.Since(start).Milliseconds()),
		ErrorMessage:   errMsg,
		CheckedAt:      time.Now(),
	}
}

func (s *CheckerService) GetCheckHistory(ctx context.Context, targetID int, limit int) ([]Result, error) {
	if targetID <= 0 {
		return nil, ErrInvalidID
	}

	if limit <= 1 {
		return nil, ErrInvalidLimit
	}

	return s.checkerRepo.GetByTargetID(ctx, targetID, limit)
}

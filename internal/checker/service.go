package checker

import (
	"StatusGuard/internal/monitor"
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type CheckerService struct {
	targetsProvider TargetProvider
	checkerRepo     CheckerRepository
	client          *http.Client
	logger          *zap.Logger
}

func NewCheckerService(targetsProvider TargetProvider, checkerRepo CheckerRepository, logger *zap.Logger) *CheckerService {
	return &CheckerService{
		targetsProvider: targetsProvider,
		checkerRepo:     checkerRepo,
		client:          &http.Client{},
		logger:          logger,
	}
}

func (s *CheckerService) CheckTarget(ctx context.Context, id int) (*Result, error) {
	target, err := s.targetsProvider.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result := s.check(ctx, target)

	savedResult, err := s.checkerRepo.Save(ctx, result)
	if err != nil {
		return nil, err
	}

	return savedResult, nil
}

func (s *CheckerService) Check(ctx context.Context, target monitor.Target) Result {
	result := s.check(ctx, &target)

	if _, err := s.checkerRepo.Save(ctx, result); err != nil {
		s.logger.Error("failed to save check result",
			zap.Int("target_id", target.ID),
			zap.Error(err),
		)
	}

	return result
}

func (s *CheckerService) check(ctx context.Context, target *monitor.Target) Result {
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
	if resp.StatusCode != target.ExpectedStatus {
		status = StatusDown
	}

	return Result{
		TargetID:       target.ID,
		Status:         status,
		HTTPStatus:     &httpStatus,
		ResponseTimeMs: int(time.Since(start).Milliseconds()),
		CheckedAt:      time.Now(),
	}
}

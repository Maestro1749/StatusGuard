package scheduler

import (
	"StatusGuard/internal/checker"
	"StatusGuard/internal/monitor"
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type TargetProvider interface {
	GetAllActive(ctx context.Context) ([]monitor.Target, error)
}

type Checker interface {
	Check(ctx context.Context, target monitor.Target) checker.Result
}

type IncidentService interface {
	HandleCheckResult(ctx context.Context, target monitor.Target, result checker.Result) error
}

type Scheduler struct {
	targetProvider TargetProvider
	checker        Checker
	incident       IncidentService

	interval time.Duration
	workers  int

	logger *zap.Logger
}

func NewScheduler(
	targetProvider TargetProvider,
	checker Checker,
	incident IncidentService,
	interval time.Duration,
	workers int,
	logger *zap.Logger,
) *Scheduler {
	if workers <= 0 {
		workers = 1
	}

	return &Scheduler{
		targetProvider: targetProvider,
		checker:        checker,
		incident:       incident,
		interval:       interval,
		workers:        workers,
		logger:         logger,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("scheduler started",
		zap.Duration("interval", s.interval),
		zap.Int("workers", s.workers),
	)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	targets, err := s.targetProvider.GetAllActive(ctx)
	if err != nil {
		s.logger.Error("failed to get active targets", zap.Error(err))
		return
	}

	if len(targets) == 0 {
		s.logger.Debug("no active targets to check")
		return
	}

	s.logger.Info("scheduler check started", zap.Int("targets_count", len(targets)))

	jobs := make(chan monitor.Target)

	var wg sync.WaitGroup

	for i := 0; i < s.workers; i++ {
		wg.Add(1)

		go func(workerID int) {
			defer wg.Done()

			for target := range jobs {
				s.checkTarget(ctx, target, workerID)
			}
		}(i + 1)
	}

	for _, target := range targets {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		case jobs <- target:
		}
	}

	close(jobs)
	wg.Wait()

	s.logger.Info("scheduler check finished")
}

func (s *Scheduler) checkTarget(ctx context.Context, target monitor.Target, workerID int) {
	result := s.checker.Check(ctx, target)

	if err := s.incident.HandleCheckResult(ctx, target, result); err != nil {
		s.logger.Error("failed to handle check result",
			zap.Int("worker_id", workerID),
			zap.Int("target_id", target.ID),
			zap.Error(err),
		)

		return
	}

	s.logger.Debug("target checked",
		zap.Int("worker_id", workerID),
		zap.Int("target_id", target.ID),
		zap.String("status", string(result.Status)),
		zap.Int("response_time_ms", result.ResponseTimeMs),
	)
}

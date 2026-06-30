package incident

import (
	"StatusGuard/internal/checker"
	"StatusGuard/internal/monitor"
	"StatusGuard/internal/notification"
	"context"
	"time"

	"go.uber.org/zap"
)

type Repository interface {
	GetOpenByTargetID(ctx context.Context, targetID int) (*Incident, error)
	Create(ctx context.Context, incident Incident) (*Incident, error)
	IncrementFailure(ctx context.Context, incidentID int, LastError *string) error
	Resolve(ctx context.Context, incidentID int, resolvedAt time.Time) error
	GetOpen(ctx context.Context) ([]Incident, error)
	GetAllOpenByTargetID(ctx context.Context, targetID int) ([]Incident, error)
}

type Service struct {
	repo     Repository
	notifier notification.Notifier
	logger   *zap.Logger
}

func NewService(repo Repository, notifier notification.Notifier, logger *zap.Logger) *Service {
	return &Service{
		repo:     repo,
		notifier: notifier,
		logger:   logger,
	}
}

func (s *Service) HandleCheckResult(ctx context.Context, target monitor.Target, result checker.Result) error {
	switch result.Status {
	case checker.StatusDown:
		return s.handleDown(ctx, target, result)

	case checker.StatusUp:
		return s.handleUp(ctx, target, result)

	default:
		s.logger.Warn("unknown check result status",
			zap.Int("target_id", target.ID),
			zap.String("status", string(result.Status)),
		)
		return nil
	}
}

func (s *Service) GetOpen(ctx context.Context) ([]Incident, error) {
	return s.repo.GetOpen(ctx)
}

func (s *Service) GetAllOpenByTargetID(ctx context.Context, targetID int) ([]Incident, error) {
	if targetID <= 0 {
		return nil, ErrInvalidTargetID
	}

	return s.repo.GetAllOpenByTargetID(ctx, targetID)
}

func (s *Service) handleDown(ctx context.Context, target monitor.Target, result checker.Result) error {
	openIncident, err := s.repo.GetOpenByTargetID(ctx, target.ID)
	if err != nil {
		return err
	}

	if openIncident != nil {
		if err := s.repo.IncrementFailure(ctx, openIncident.ID, result.ErrorMessage); err != nil {
			return err
		}

		s.logger.Info("incident failure count increased",
			zap.Int("target_id", target.ID),
			zap.Int("incident_id", openIncident.ID),
		)

		return nil
	}

	newIncident := Incident{
		TargetID:     target.ID,
		Status:       StatusOpen,
		StartedAt:    time.Now(),
		ResolvedAt:   nil,
		LastError:    result.ErrorMessage,
		ChecksFailed: 1,
	}

	createdIncident, err := s.repo.Create(ctx, newIncident)
	if err != nil {
		return err
	}

	errMsg := ""
	if result.ErrorMessage != nil {
		errMsg = *result.ErrorMessage
	}

	if err := s.notifier.NotifyIncidentOpened(ctx, target.Name, target.URL, errMsg); err != nil {
		s.logger.Error("failed to send incident opened notification",
			zap.Int("target_id", target.ID),
			zap.Int("incident_id", createdIncident.ID),
			zap.Error(err),
		)
	}

	s.logger.Warn("incident opened",
		zap.Int("target_id", target.ID),
		zap.Int("incident_id", createdIncident.ID),
		zap.String("target_url", target.URL),
	)

	return nil
}

func (s *Service) handleUp(
	ctx context.Context,
	target monitor.Target,
	result checker.Result,
) error {
	openIncident, err := s.repo.GetOpenByTargetID(ctx, target.ID)
	if err != nil {
		return err
	}

	if openIncident == nil {
		return nil
	}

	resolvedAt := time.Now()

	if err := s.repo.Resolve(ctx, openIncident.ID, resolvedAt); err != nil {
		return err
	}

	if err := s.notifier.NotifyIncidentResolved(ctx, target.Name, target.URL); err != nil {
		s.logger.Error("failed to send incident resolved notification",
			zap.Int("target_id", target.ID),
			zap.Int("incident_id", openIncident.ID),
			zap.Error(err),
		)
	}

	s.logger.Info("incident resolved",
		zap.Int("target_id", target.ID),
		zap.Int("incident_id", openIncident.ID),
		zap.Duration("duration", resolvedAt.Sub(openIncident.StartedAt)),
	)

	return nil
}

package checker

import (
	"StatusGuard/internal/monitor"
	"context"
	"database/sql"
	"errors"
	"time"

	"go.uber.org/zap"
)

type TargetProvider interface {
	GetByID(ctx context.Context, id int) (*monitor.Target, error)
}

type CheckerRepository interface {
	Save(ctx context.Context, result Result) (*Result, error)
}

type CheckerRepo struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewCheckerRepository(db *sql.DB, logger *zap.Logger) *CheckerRepo {
	return &CheckerRepo{
		db:     db,
		logger: logger,
	}
}

func (r *CheckerRepo) Save(ctx context.Context, result Result) (*Result, error) {
	query := `
		INSERT INTO check_result (
			target_id,
			status,
			response_time_ms,
			status_code,
			error_message,
			checked_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id;
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.db.QueryRowContext(
		ctxTimeout,
		query,
		result.TargetID,
		result.Status,
		result.ResponseTimeMs,
		result.HTTPStatus,
		result.ErrorMessage,
		result.CheckedAt,
	).Scan(&result.ID); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 5*time.Second))
			return nil, ErrTimeout
		}
		r.logger.Error("failed to execute database query")
		return nil, ErrInternalServer
	}

	return &result, nil
}

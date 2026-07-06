package checker

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"go.uber.org/zap"
)

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
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctxTimeout.Err(), context.DeadlineExceeded) {
			r.logger.Warn(
				"database query timed out", zap.Error(err),
				zap.Int("target_id", result.TargetID),
				zap.Duration("timeout_limit", 5*time.Second),
			)
			return nil, ErrTimeout
		}

		r.logger.Error(
			"failed to execute database query",
			zap.Error(err),
			zap.Int("target_id", result.TargetID),
		)

		return nil, ErrInternalServer
	}

	return &result, nil
}

func (r *CheckerRepo) GetByTargetID(ctx context.Context, targetID int, limit int) ([]Result, error) {
	query := `
		SELECT id, target_id, status, response_time_ms, status_code, error_message, checked_at
		FROM check_result
		WHERE target_id = $1
		ORDER BY checked_at DESC
		LIMIT $2;
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var results []Result
	rows, err := r.db.QueryContext(
		ctxTimeout,
		query,
		targetID,
		limit,
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 10*time.Second))
			return nil, ErrTimeout
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrResultsNotFound
		}

		r.logger.Error("failed to execute query", zap.Error(err))
		return nil, ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var result Result
		if err := rows.Scan(
			&result.ID,
			&result.TargetID,
			&result.Status,
			&result.ResponseTimeMs,
			&result.HTTPStatus,
			&result.ErrorMessage,
			&result.CheckedAt,
		); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 10*time.Second))
				return nil, ErrTimeout
			}

			r.logger.Error("error reading data", zap.Error(err))
			return nil, ErrInternalServer
		}

		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		r.logger.Error("iteration error", zap.Error(err))
		return nil, ErrInternalServer
	}

	return results, nil
}

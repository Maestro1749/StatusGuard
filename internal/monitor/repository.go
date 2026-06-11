package monitor

import (
	"context"
	"database/sql"
	"errors"
	"time"

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

type MonitorRepo struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewMonitorRepository(db *sql.DB, logger *zap.Logger) *MonitorRepo {
	return &MonitorRepo{db: db, logger: logger}
}

func (r *MonitorRepo) CreateTarget(ctx context.Context, target Target) (*Target, error) {
	query := `
		INSERT INTO targets (
			name,
			url,
			method,
			expected_status,
			interval_seconds,
			timeout_seconds,
			enabled
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at;
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := r.db.QueryRowContext(
		ctxTimeout,
		query,
		target.Name,
		target.URL,
		target.Method,
		target.ExpectedStatus,
		target.IntervalSeconds,
		target.TimeoutSeconds,
		target.Enabled,
	).Scan(
		&target.ID,
		&target.CreatedAt,
		&target.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 5*time.Second))
			return nil, ErrTimeout
		}

		r.logger.Error("error to execute query", zap.Error(err))
		return nil, ErrInternalServer
	}

	return &target, nil
}

func (r *MonitorRepo) DeleteTarget(ctx context.Context, id int) error {
	query := `
		DELETE FROM targets WHERE id = $1;
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := r.db.ExecContext(
		ctxTimeout,
		query,
		id,
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 5*time.Second))
			return ErrTimeout
		}

		r.logger.Error("failed to delete target", zap.Int("id", id), zap.Error(err))
		return ErrInternalServer
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		r.logger.Error("failed to get rows affected", zap.Error(err))
		return ErrInternalServer
	}
	if rowsAffected == 0 {
		return ErrTargetNotFound
	}

	return nil
}

func (r *MonitorRepo) GetTarget(ctx context.Context, target Target) (*Target, error) {
	query := `
		SELECT 
			name,
			url,
			method,
			expected_status,
			interval_seconds,
			timeout_seconds,
			enabled,
			created_at,
			updated_at
		FROM targets WHERE id = $1;
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.db.QueryRowContext(ctxTimeout, query, target.ID).Scan(
		&target.Name,
		&target.URL,
		&target.Method,
		&target.ExpectedStatus,
		&target.IntervalSeconds,
		&target.TimeoutSeconds,
		&target.Enabled,
		&target.CreatedAt,
		&target.UpdatedAt,
	); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 5*time.Second))
			return nil, ErrTimeout
		}

		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTargetNotFound
		}

		r.logger.Error("failed to execute query", zap.Error(err))
		return nil, ErrInternalServer
	}

	return &target, nil
}

func (r *MonitorRepo) GetAllTargets(ctx context.Context) ([]Target, error) {
	query := `
		SELECT 
			id,
			name,
			url,
			method,
			expected_status,
			interval_seconds,
			timeout_seconds,
			enabled,
			created_at,
			updated_at
		FROM targets;
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rows, err := r.db.QueryContext(ctxTimeout, query)
	if err != nil {
		r.logger.Error("failed to execute query", zap.Error(err))
		return nil, ErrInternalServer
	}
	defer rows.Close()

	var targets []Target

	for rows.Next() {
		var target Target
		if err := rows.Scan(
			&target.ID,
			&target.Name,
			&target.URL,
			&target.Method,
			&target.ExpectedStatus,
			&target.IntervalSeconds,
			&target.TimeoutSeconds,
			&target.Enabled,
			&target.CreatedAt,
			&target.UpdatedAt,
		); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 10*time.Second))
				return nil, ErrTimeout
			}

			r.logger.Error("error reading data", zap.Error(err))
			return nil, ErrInternalServer
		}
		targets = append(targets, target)

		if err := rows.Err(); err != nil {
			r.logger.Error("iteration error", zap.Error(err))
			return nil, ErrInternalServer
		}
	}

	return targets, nil
}

func (r *MonitorRepo) GetByID(ctx context.Context, id int) (*Target, error) {
	query := `
		SELECT 
			id,
			name,
			url,
			method,
			expected_status,
			interval_seconds,
			timeout_seconds,
			enabled,
			created_at,
			updated_at
		FROM targets WHERE id = $1;  
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var target Target

	if err := r.db.QueryRowContext(
		ctxTimeout,
		query,
		id,
	).Scan(
		&target.ID,
		&target.Name,
		&target.URL,
		&target.Method,
		&target.ExpectedStatus,
		&target.IntervalSeconds,
		&target.TimeoutSeconds,
		&target.Enabled,
		&target.CreatedAt,
		&target.UpdatedAt,
	); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 5*time.Second))
			return nil, ErrTimeout
		}

		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTargetNotFound
		}

		r.logger.Error("failed to execute query", zap.Error(err))
		return nil, ErrInternalServer
	}

	return &target, nil
}

func (r *MonitorRepo) UpdateTarget(ctx context.Context, target Target) (*Target, error) {
	query := `
		UPDATE targets
		SET
			name = $1,
			url = $2,
			method = $3,
			expected_status = $4,
			interval_seconds = $5,
			timeout_seconds = $6,
			enabled = $7,
			updated_at = now()
		WHERE id = $8
		RETURNING id, name, url, method, expected_status, interval_seconds, 
			timeout_seconds, enabled, created_at, updated_at;
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := r.db.QueryRowContext(
		ctxTimeout,
		query,
		target.Name,
		target.URL,
		target.Method,
		target.ExpectedStatus,
		target.IntervalSeconds,
		target.TimeoutSeconds,
		target.Enabled,
		target.ID,
	).Scan(
		&target.ID,
		&target.Name,
		&target.URL,
		&target.Method,
		&target.ExpectedStatus,
		&target.IntervalSeconds,
		&target.TimeoutSeconds,
		&target.Enabled,
		&target.CreatedAt,
		&target.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTargetNotFound
		}

		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 5*time.Second))
			return nil, ErrTimeout
		}

		r.logger.Error("failed to execute database query", zap.Error(err))
		return nil, ErrInternalServer
	}

	return &target, nil
}

func (r *MonitorRepo) GetAllActive(ctx context.Context) ([]Target, error) {
	query := `
		SELECT 
			id,
			name,
			url,
			method,
			expected_status,
			interval_seconds,
			timeout_seconds,
			enabled,
			created_at,
			updated_at
		FROM targets
		WHERE enabled = true;
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var targets []Target
	rows, err := r.db.QueryContext(ctxTimeout, query)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 10*time.Second))
			return nil, ErrTimeout
		}

		r.logger.Error("failed to execute database query", zap.Error(err))
		return nil, ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var target Target
		if err := rows.Scan(
			&target.ID,
			&target.Name,
			&target.URL,
			&target.Method,
			&target.ExpectedStatus,
			&target.IntervalSeconds,
			&target.TimeoutSeconds,
			&target.Enabled,
			&target.CreatedAt,
			&target.UpdatedAt,
		); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 10*time.Second))
				return nil, ErrTimeout
			}

			r.logger.Error("error reading data", zap.Error(err))
			return nil, ErrInternalServer
		}

		targets = append(targets, target)

		if err := rows.Err(); err != nil {
			r.logger.Error("iteration error", zap.Error(err))
			return nil, ErrInternalServer
		}
	}

	return targets, nil
}

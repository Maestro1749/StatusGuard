package monitor

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"go.uber.org/zap"
)

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
	}
	if err := rows.Err(); err != nil {
		r.logger.Error("iteration error", zap.Error(err))
		return nil, ErrInternalServer
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
	}
	if err := rows.Err(); err != nil {
		r.logger.Error("iteration error", zap.Error(err))
		return nil, ErrInternalServer
	}

	return targets, nil
}

func (r *MonitorRepo) GetTargetsDueForCheck(ctx context.Context, limit int) ([]Target, error) {
	query := `
		WITH locked_targets AS (
			SELECT id
			FROM targets
			WHERE enabled = true AND next_check_at <= now()
			ORDER BY next_check_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE targets
		SET next_check_at = NOW() + INTERVAL '5 minutes'
		FROM locked_targets
		WHERE targets.id = locked_targets.id
		RETURNING
			targets.id,
			targets.name,
			targets.url,
			targets.method,
			targets.expected_status,
			targets.interval_seconds,
			targets.timeout_seconds,
			targets.enabled,
			targets.created_at,
			targets.updated_at,
			targets.next_check_at;
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var targets []Target
	rows, err := r.db.QueryContext(ctxTimeout, query, limit)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 5*time.Second))
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
			&target.NextCheckAt,
		); err != nil {
			r.logger.Error("error reading data", zap.Error(err))
			return nil, ErrInternalServer
		}

		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database iteration timed out", zap.Error(err))
			return nil, ErrTimeout
		}
		r.logger.Error("iteration error", zap.Error(err))
		return nil, ErrInternalServer
	}

	return targets, nil
}

func (r *MonitorRepo) UpdateNextCheckAt(ctx context.Context, targetID int, nextCheckAt time.Time) error {
	query := `
		UPDATE targets SET next_check_at = $1 WHERE id = $2; 
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := r.db.ExecContext(
		ctxTimeout,
		query,
		nextCheckAt,
		targetID,
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 5*time.Second))
			return ErrTimeout
		}

		r.logger.Error("failed to execute database query", zap.Error(err))
		return ErrInternalServer
	}

	return nil
}

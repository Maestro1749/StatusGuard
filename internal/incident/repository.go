package incident

import (
	"context"
	"database/sql"
	"errors"
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

type Repo struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewRepository(db *sql.DB, logger *zap.Logger) *Repo {
	return &Repo{
		db:     db,
		logger: logger,
	}
}

func (r *Repo) GetOpenByTargetID(ctx context.Context, targetID int) (*Incident, error) {
	query := `
		SELECT id, target_id, status, started_at, resolved_at, last_error, checks_failed
		FROM incidents
		WHERE target_id = $1 AND status = 'open'
		LIMIT 1;
	`

	var incident Incident

	err := r.db.QueryRowContext(ctx, query, targetID).Scan(
		&incident.ID,
		&incident.TargetID,
		&incident.Status,
		&incident.StartedAt,
		&incident.ResolvedAt,
		&incident.LastError,
		&incident.ChecksFailed,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &incident, nil
}

func (r *Repo) Create(ctx context.Context, incident Incident) (*Incident, error) {
	query := `
		INSERT INTO incidents (
			target_id,
			status,
			started_at,
			resolved_at,
			last_error,
			checks_failed
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, target_id, status, started_at, resolved_at, last_error, checks_failed
	`

	var created Incident

	err := r.db.QueryRowContext(
		ctx,
		query,
		incident.TargetID,
		incident.Status,
		incident.StartedAt,
		incident.ResolvedAt,
		incident.LastError,
		incident.ChecksFailed,
	).Scan(
		&created.ID,
		&created.TargetID,
		&created.Status,
		&created.StartedAt,
		&created.ResolvedAt,
		&created.LastError,
		&created.ChecksFailed,
	)

	if err != nil {
		return nil, err
	}

	return &created, nil
}

func (r *Repo) IncrementFailure(
	ctx context.Context,
	incidentID int,
	lastError *string,
) error {
	query := `
		UPDATE incidents
		SET 
			checks_failed = checks_failed + 1,
			last_error = $2
		WHERE id = $1 AND status = 'open'
	`

	_, err := r.db.ExecContext(ctx, query, incidentID, lastError)
	return err
}

func (r *Repo) Resolve(
	ctx context.Context,
	incidentID int,
	resolvedAt time.Time,
) error {
	query := `
		UPDATE incidents
		SET 
			status = 'resolved',
			resolved_at = $2
		WHERE id = $1 AND status = 'open'
	`

	_, err := r.db.ExecContext(ctx, query, incidentID, resolvedAt)
	return err
}

func (r *Repo) GetOpen(ctx context.Context) ([]Incident, error) {
	query := `
		SELECT id, target_id, status, started_at, resolved_at, last_error, checks_failed
		FROM incidents
		WHERE status = 'open';
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var incidents []Incident
	rows, err := r.db.QueryContext(ctxTimeout, query)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 10*time.Second))
			return nil, ErrTimeout
		}
	}
	defer rows.Close()

	for rows.Next() {
		var incident Incident
		if err := rows.Scan(
			&incident.ID,
			&incident.TargetID,
			&incident.Status,
			&incident.StartedAt,
			&incident.ResolvedAt,
			&incident.LastError,
			&incident.ChecksFailed,
		); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 10*time.Second))
				return nil, ErrTimeout
			}

			r.logger.Error("error reading data", zap.Error(err))
			return nil, ErrInternalServer
		}

		incidents = append(incidents, incident)

		if err := rows.Err(); err != nil {
			r.logger.Error("iteration error", zap.Error(err))
			return nil, ErrInternalServer
		}
	}

	return incidents, nil
}

func (r *Repo) GetAllOpenByTargetID(ctx context.Context, targetID int) ([]Incident, error) {
	query := `
		SELECT id, target_id, status, started_at, resolved_at, last_error, checks_failed
		FROM incidents
		WHERE target_id = $1 AND status = 'open';
	`

	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var incidents []Incident
	rows, err := r.db.QueryContext(
		ctxTimeout,
		query,
		targetID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if errors.Is(err, context.DeadlineExceeded) {
			r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 10*time.Second))
			return nil, ErrTimeout
		}

		r.logger.Error("failed to execute query", zap.Error(err))
		return nil, ErrInternalServer
	}

	for rows.Next() {
		var incident Incident
		if err := rows.Scan(
			&incident.ID,
			&incident.TargetID,
			&incident.Status,
			&incident.StartedAt,
			&incident.ResolvedAt,
			&incident.LastError,
			&incident.ChecksFailed,
		); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				r.logger.Warn("database query timed out", zap.Error(err), zap.Duration("timeout_limit", 10*time.Second))
				return nil, ErrTimeout
			}

			r.logger.Error("error reading data", zap.Error(err))
			return nil, ErrInternalServer
		}

		incidents = append(incidents, incident)

		if err := rows.Err(); err != nil {
			r.logger.Error("iteration error", zap.Error(err))
			return nil, ErrInternalServer
		}
	}

	return incidents, nil
}

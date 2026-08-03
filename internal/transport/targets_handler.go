package transport

import (
	"StatusGuard/internal/monitor"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"
)

/*
	POST   /targets - добавление сервиса
	GET    /targets	- получить список сервисов
	GET    /targets/{id}	- получить конкретный сервис
	PATCH  /targets/{id}	- изменить параметры сервиса
	DELETE /targets/{id}	- удалить сервис

	GET /targets/{id}/incidents

	GET /incidents
	GET /incidents/open

	GET    /health - проверить жив ли сервис
*/

type MonitorHandler struct {
	logger         *zap.Logger
	monitorService MonitorService
}

func NewMonitorHandler(logger *zap.Logger, monitorService MonitorService) *MonitorHandler {
	return &MonitorHandler{
		logger:         logger,
		monitorService: monitorService,
	}
}

type MonitorService interface {
	CreateTarget(
		ctx context.Context,
		name string,
		urlTarget string,
		method string,
		expectedStatus int,
		intervalSeconds int,
		timeoutSeconds int,
	) (*monitor.Target, error)
	DeleteTarget(ctx context.Context, id int) error
	GetTarget(ctx context.Context, id int) (*monitor.Target, error)
	GetAllTargets(ctx context.Context) ([]monitor.Target, error)
	UpdateTarget(ctx context.Context, input monitor.UpdateTargetInput) (*monitor.Target, error)
}

func (h *MonitorHandler) CreateTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var data CreateTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "invalid data input", http.StatusBadRequest)
		return
	}

	target, err := h.monitorService.CreateTarget(
		ctx,
		data.Name,
		data.URL,
		data.Method,
		data.ExpectedStatus,
		data.IntervalSeconds,
		data.TimeoutSeconds,
	)
	if err != nil {
		switch {
		case errors.Is(err, monitor.ErrEmptyName):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, monitor.ErrInvalidInterval):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, monitor.ErrInvalidTimeout):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, monitor.ErrInvalidURL):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, monitor.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		default:
			http.Error(w, monitor.ErrInternalServer.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(target); err != nil {
		h.logger.Error("failed to encode targets", zap.Error(err))
		return
	}
}

func (h *MonitorHandler) DeleteTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid target id", http.StatusBadRequest)
		return
	}

	if err := h.monitorService.DeleteTarget(ctx, id); err != nil {
		switch {
		case errors.Is(err, monitor.ErrInvalidID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, monitor.ErrTargetNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, monitor.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		default:
			http.Error(w, monitor.ErrInternalServer.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *MonitorHandler) GetTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid target id", http.StatusBadRequest)
		return
	}

	target, err := h.monitorService.GetTarget(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, monitor.ErrInvalidID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, monitor.ErrTargetNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, monitor.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		default:
			http.Error(w, monitor.ErrInternalServer.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(target); err != nil {
		h.logger.Error("failed to encode targets", zap.Error(err))
		return
	}
}

func (h *MonitorHandler) GetAllTargets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	targets, err := h.monitorService.GetAllTargets(ctx)
	if err != nil {
		switch {
		case errors.Is(err, monitor.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		default:
			http.Error(w, monitor.ErrInternalServer.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(targets); err != nil {
		h.logger.Error("failed to encode targets", zap.Error(err))
		return
	}
}

func (h *MonitorHandler) UpdateTarget(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid target id", http.StatusBadRequest)
		return
	}

	var data UpdateTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "invalid data input", http.StatusBadRequest)
		return
	}

	input := monitor.UpdateTargetInput{
		ID:              id,
		Name:            data.Name,
		URL:             data.URL,
		Method:          data.Method,
		ExpectedStatus:  data.ExpectedStatus,
		IntervalSeconds: data.IntervalSeconds,
		TimeoutSeconds:  data.TimeoutSeconds,
		Enabled:         data.Enabled,
	}

	target, err := h.monitorService.UpdateTarget(ctx, input)
	if err != nil {
		switch {
		case errors.Is(err, monitor.ErrEmptyName),
			errors.Is(err, monitor.ErrInvalidURL),
			errors.Is(err, monitor.ErrInvalidMethod),
			errors.Is(err, monitor.ErrInvalidExpectedStatus),
			errors.Is(err, monitor.ErrInvalidInterval),
			errors.Is(err, monitor.ErrInvalidTimeout):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, monitor.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		case errors.Is(err, monitor.ErrTargetNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, monitor.ErrInternalServer.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(target); err != nil {
		h.logger.Error("failed to encode target", zap.Error(err))
		return
	}
}

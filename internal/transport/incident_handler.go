package transport

import (
	"StatusGuard/internal/incident"
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"
)

type IncidentHandler struct {
	service *incident.Service
	logger  *zap.Logger
}

func NewIncidentHandler(service *incident.Service, logger *zap.Logger) *IncidentHandler {
	return &IncidentHandler{
		service: service,
		logger:  logger,
	}
}

/*
	GET /incidents/open
	GET /targets/{id}/incidents
*/

func (h *IncidentHandler) GetOpen(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	incidents, err := h.service.GetOpen(ctx)
	if err != nil {
		switch {
		case errors.Is(err, incident.ErrNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, incident.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		default:
			http.Error(w, incident.ErrInternalServer.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(incidents); err != nil {
		h.logger.Error("failed to encode data", zap.Error(err))
		return
	}
}

func (h *IncidentHandler) GetAllOpenByTargetID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid target id", http.StatusBadRequest)
		return
	}

	incidents, err := h.service.GetAllOpenByTargetID(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, incident.ErrInvalidTargetID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, incident.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		case errors.Is(err, incident.ErrNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		default:
			http.Error(w, incident.ErrInternalServer.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(incidents); err != nil {
		h.logger.Error("failed to encode data", zap.Error(err))
		return
	}
}

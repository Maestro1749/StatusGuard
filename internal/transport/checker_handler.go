package transport

import (
	"StatusGuard/internal/checker"
	"StatusGuard/internal/monitor"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"go.uber.org/zap"
)

type CheckerHandler struct {
	service *checker.CheckerService
	logger  *zap.Logger
}

func NewCheckerHandler(service *checker.CheckerService, logger *zap.Logger) *CheckerHandler {
	return &CheckerHandler{
		service: service,
		logger:  logger,
	}
}

/*
	POST /targets/{id}/check
	GET /targets/{id}/checks?limit=20
*/

func (h *CheckerHandler) CheckTarget(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid target id", http.StatusBadRequest)
		return
	}

	result, retryAfter, err := h.service.CheckManually(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, monitor.ErrTargetNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, monitor.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		case errors.Is(err, checker.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		case errors.Is(err, checker.ErrTooManyRequests):
			http.Error(w, fmt.Sprintf("%s. Try again after %v seconds", err.Error(), retryAfter), http.StatusTooManyRequests)
		default:
			http.Error(w, checker.ErrInternalServer.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("failed to encode result", zap.Error(err))
		return
	}
}

func (h *CheckerHandler) GetCheckHistory(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid target id", http.StatusBadRequest)
		return
	}

	query := r.URL.Query()
	var limit_query int
	if query.Has("limit") {
		limit_query, err = strconv.Atoi(query.Get("limit"))
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
	} else {
		limit_query = 20
	}

	results, err := h.service.GetCheckHistory(r.Context(), id, limit_query)
	if err != nil {
		switch {
		case errors.Is(err, checker.ErrResultsNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, checker.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		case errors.Is(err, checker.ErrInvalidID):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, checker.ErrInvalidLimit):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, checker.ErrInternalServer.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		h.logger.Error("failed to encode results", zap.Error(err))
		return
	}
}

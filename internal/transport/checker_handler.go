package transport

import (
	"StatusGuard/internal/checker"
	"StatusGuard/internal/monitor"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
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
	GET /targets/{id}/checks
*/

func (h *CheckerHandler) CheckTarget(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "invalid target id", http.StatusBadRequest)
		return
	}

	result, err := h.service.CheckTarget(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, monitor.ErrTargetNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, monitor.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
		case errors.Is(err, checker.ErrTimeout):
			http.Error(w, err.Error(), http.StatusGatewayTimeout)
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

func parseID(r *http.Request) (int, error) {
	vars := mux.Vars(r)

	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return -1, checker.ErrInvalidID
	}

	return id, nil
}

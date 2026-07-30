package transport

import (
	"StatusGuard/internal/checker"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func parseID(r *http.Request) (int, error) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return -1, checker.ErrInvalidID
	}

	return id, nil
}

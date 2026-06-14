package transport

import (
	"StatusGuard/internal/checker"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

func parseID(r *http.Request) (int, error) {
	vars := mux.Vars(r)

	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return -1, checker.ErrInvalidID
	}

	return id, nil
}

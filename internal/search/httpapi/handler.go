package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	querynorm "github.com/venomimonstro/poisk/internal/query"
	searchsvc "github.com/venomimonstro/poisk/internal/search"
)

type Searcher interface {
	Search(ctx context.Context, req searchsvc.Request) (searchsvc.Response, error)
}

type Handler struct {
	SearchService Searcher
}

func (h Handler) Search(w http.ResponseWriter, r *http.Request) {
	if h.SearchService == nil {
		writeError(w, http.StatusServiceUnavailable, "search_unavailable")
		return
	}
	limit := 10
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 20 {
			writeError(w, http.StatusBadRequest, "invalid_limit")
			return
		}
		limit = n
	}
	resp, err := h.SearchService.Search(r.Context(), searchsvc.Request{Query: r.URL.Query().Get("q"), Limit: limit})
	if err != nil {
		switch {
		case errors.Is(err, querynorm.ErrEmptyQuery):
			writeError(w, http.StatusBadRequest, "empty_query")
		case errors.Is(err, querynorm.ErrQueryTooLong), errors.Is(err, querynorm.ErrInvalidQuery):
			writeError(w, http.StatusBadRequest, "invalid_query")
		default:
			writeError(w, http.StatusBadGateway, "search_backend_error")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

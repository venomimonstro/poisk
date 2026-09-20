package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	answersvc "github.com/venomimonstro/poisk/internal/answer"
	querynorm "github.com/venomimonstro/poisk/internal/query"
)

type Answerer interface {
	Answer(ctx context.Context, req answersvc.Request) (answersvc.Response, error)
}

type CitationRecorder interface {
	RecordAnswerCitations(context.Context, []string) error
}

type Handler struct {
	AnswerService Answerer
	Citations     CitationRecorder
}

func (h Handler) Answer(w http.ResponseWriter, r *http.Request) {
	if h.AnswerService == nil {
		writeError(w, http.StatusServiceUnavailable, "answer_unavailable")
		return
	}
	resp, err := h.AnswerService.Answer(r.Context(), answersvc.Request{Query: r.URL.Query().Get("q")})
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
			writeError(w, http.StatusGatewayTimeout, "answer_timeout")
		case errors.Is(err, querynorm.ErrEmptyQuery):
			writeError(w, http.StatusBadRequest, "empty_query")
		case errors.Is(err, querynorm.ErrQueryTooLong), errors.Is(err, querynorm.ErrInvalidQuery):
			writeError(w, http.StatusBadRequest, "invalid_query")
		case errors.Is(err, querynorm.ErrQueryTooComplex):
			writeError(w, http.StatusBadRequest, "query_too_complex")
		default:
			writeError(w, http.StatusBadGateway, "answer_backend_error")
		}
		return
	}
	if h.Citations != nil && resp.Available && len(resp.Sources) > 0 {
		hosts := make([]string, 0, len(resp.Sources))
		for _, source := range resp.Sources { if source.Host != "" { hosts = append(hosts, source.Host) } }
		_ = h.Citations.RecordAnswerCitations(r.Context(), hosts)
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

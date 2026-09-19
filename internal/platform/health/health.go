package health

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Checker struct {
	DB                *pgxpool.Pool
	ManticoreHost     string
	ManticoreSQLPort  int
}

type response struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components,omitempty"`
}

func (c Checker) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, response{Status: "ok"})
}

func (c Checker) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 750*time.Millisecond)
	defer cancel()

	components := map[string]string{}
	ready := true

	if c.DB == nil || c.DB.Ping(ctx) != nil {
		components["postgres"] = "down"
		ready = false
	} else {
		components["postgres"] = "ok"
	}

	address := net.JoinHostPort(c.ManticoreHost, strconv.Itoa(c.ManticoreSQLPort))
	conn, err := (&net.Dialer{Timeout: 300 * time.Millisecond}).DialContext(ctx, "tcp", address)
	if err != nil {
		components["manticore"] = "down"
		ready = false
	} else {
		_ = conn.Close()
		components["manticore"] = "ok"
	}

	if !ready {
		writeJSON(w, http.StatusServiceUnavailable, response{Status: "not_ready", Components: components})
		return
	}

	writeJSON(w, http.StatusOK, response{Status: "ready", Components: components})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

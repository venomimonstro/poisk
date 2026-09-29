package health

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/venomimonstro/poisk/internal/buildinfo"
)

type Checker struct {
	DB                *pgxpool.Pool
	ManticoreHost     string
	ManticoreSQLPort  int
}

type buildIdentity struct {
	GitCommit string `json:"git_commit"`
	ReleaseVersion string `json:"release_version"`
}

type response struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components,omitempty"`
	Build buildIdentity `json:"build"`
}

func currentBuild()buildIdentity{return buildIdentity{GitCommit:buildinfo.GitCommit,ReleaseVersion:buildinfo.ReleaseVersion}}

func (c Checker) Live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, response{Status: "ok",Build:currentBuild()})
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
		writeJSON(w, http.StatusServiceUnavailable, response{Status: "not_ready", Components: components,Build:currentBuild()})
		return
	}

	writeJSON(w, http.StatusOK, response{Status: "ready", Components: components,Build:currentBuild()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control","no-store")
	w.Header().Set("X-Content-Type-Options","nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

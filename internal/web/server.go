package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"micgain-manager/internal/config"
	"micgain-manager/internal/core"
)

//go:embed static/*
var rawStatic embed.FS
var staticContent fs.FS

func init() {
	var err error
	staticContent, err = fs.Sub(rawStatic, "static")
	if err != nil {
		panic(err)
	}
}

// Server exposes HTTP API + mini UI for runtime control.
type Server struct {
	manager *core.Manager
	server  *http.Server
}

// New creates the HTTP server bound to addr.
func New(manager *core.Manager, addr string) *Server {
	mux := http.NewServeMux()
	srv := &Server{manager: manager}
	mux.HandleFunc("/api/config", srv.handleConfig)
	mux.HandleFunc("/api/apply", srv.handleApply)
	mux.Handle("/", http.FileServer(http.FS(staticContent)))

	srv.server = &http.Server{
		Addr:    addr,
		Handler: loggingMiddleware(mux),
	}
	return srv
}

// Start blocks and serves HTTP traffic.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		snap := s.manager.GetSnapshot()
		respondJSON(w, http.StatusOK, snapshotToView(snap))
	case http.MethodPut:
		var req updatePayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		cfg := s.manager.CurrentConfig()
		if req.TargetVolume != nil {
			cfg.TargetVolume = *req.TargetVolume
		}
		if req.IntervalSeconds != nil {
			cfg.Interval = time.Duration(*req.IntervalSeconds * float64(time.Second))
		}
		if req.Enabled != nil {
			cfg.Enabled = *req.Enabled
		}
		if err := s.manager.UpdateConfig(cfg, req.ApplyNow); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respondJSON(w, http.StatusOK, snapshotToView(s.manager.GetSnapshot()))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.manager.ApplyOnce(-1); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, snapshotToView(s.manager.GetSnapshot()))
}

func snapshotToView(snap config.StatusSnapshot) map[string]any {
	var nextRun *time.Time
	if !snap.NextRun.IsZero() {
		nr := snap.NextRun
		nextRun = &nr
	}
	var runningSince *time.Time
	if snap.RunningSince != nil {
		rs := *snap.RunningSince
		runningSince = &rs
	}
	cfg := map[string]any{
		"targetVolume":    snap.Config.TargetVolume,
		"intervalSeconds": snap.Config.Interval.Seconds(),
		"enabled":         snap.Config.Enabled,
		"lastApplyStatus": snap.Config.LastApplyStatus,
	}
	if snap.Config.LastError != "" {
		cfg["lastError"] = snap.Config.LastError
	}
	if !snap.Config.LastApplied.IsZero() {
		cfg["lastApplied"] = snap.Config.LastApplied
	}
	return map[string]any{
		"config":       cfg,
		"nextRun":      nextRun,
		"idle":         snap.SchedulerIdle,
		"runningSince": runningSince,
	}
}

type updatePayload struct {
	TargetVolume    *int     `json:"targetVolume"`
	IntervalSeconds *float64 `json:"intervalSeconds"`
	Enabled         *bool    `json:"enabled"`
	ApplyNow        bool     `json:"applyNow"`
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode JSON: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

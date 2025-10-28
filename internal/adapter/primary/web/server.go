package web

import (
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"micgain-manager/internal/domain"
	"micgain-manager/internal/usecase"
)

//go:embed static/*
var staticFiles embed.FS

// Server is a primary adapter that exposes HTTP API + UI.
// It depends on the use case (primary port).
type Server struct {
	usecase usecase.SchedulerUseCase
	server  *http.Server
}

// NewServer creates the HTTP server bound to addr.
func NewServer(uc usecase.SchedulerUseCase, addr string) *Server {
	mux := http.NewServeMux()
	srv := &Server{usecase: uc}

	// API endpoints
	mux.HandleFunc("/api/config", srv.handleConfig)
	mux.HandleFunc("/api/apply", srv.handleApply)

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

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
		snap := s.usecase.GetSnapshot()
		respondJSON(w, http.StatusOK, snapshotToView(snap))
	case http.MethodPut:
		var req updatePayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		snap := s.usecase.GetSnapshot()
		config := snap.Config

		if req.TargetVolume != nil {
			config.TargetVolume = *req.TargetVolume
		}
		if req.IntervalSeconds != nil {
			config.Interval = time.Duration(*req.IntervalSeconds) * time.Second
		}
		if req.Enabled != nil {
			config.Enabled = *req.Enabled
		}

		if err := s.usecase.UpdateConfig(config, req.ApplyNow); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		respondJSON(w, http.StatusOK, snapshotToView(s.usecase.GetSnapshot()))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if err := s.usecase.ApplyNow(-1); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, snapshotToView(s.usecase.GetSnapshot()))
}

func snapshotToView(snap domain.Snapshot) map[string]any {
	var nextRun *time.Time
	if !snap.ScheduleState.NextRun.IsZero() {
		nr := snap.ScheduleState.NextRun
		nextRun = &nr
	}

	cfg := map[string]any{
		"targetVolume":    snap.Config.TargetVolume,
		"intervalSeconds": snap.Config.Interval.Seconds(),
		"enabled":         snap.Config.Enabled,
		"lastApplyStatus": snap.ScheduleState.LastApplyStatus.String(),
	}

	if snap.ScheduleState.LastError != nil {
		cfg["lastError"] = snap.ScheduleState.LastError.Error()
	}
	if !snap.ScheduleState.LastApplied.IsZero() {
		cfg["lastApplied"] = snap.ScheduleState.LastApplied
	}

	return map[string]any{
		"config":  cfg,
		"nextRun": nextRun,
		"idle":    !snap.ScheduleState.IsRunning,
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

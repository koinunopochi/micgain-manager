package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"micgain-manager/internal/domain"
	"micgain-manager/internal/usecase"
)

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
	mux.HandleFunc("/api/config", srv.handleConfig)
	mux.HandleFunc("/api/apply", srv.handleApply)
	mux.HandleFunc("/", srv.handleRoot)

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

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Mic Gain Manager</title>
    <style>
        body { font-family: sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        h1 { color: #333; }
        .info { background: #f0f0f0; padding: 15px; border-radius: 5px; margin: 20px 0; }
        button { background: #007bff; color: white; border: none; padding: 10px 20px; border-radius: 5px; cursor: pointer; }
        button:hover { background: #0056b3; }
        input { padding: 8px; margin: 5px; }
        label { display: inline-block; width: 150px; }
    </style>
</head>
<body>
    <h1>Mic Gain Manager</h1>
    <div class="info" id="status">Loading...</div>
    <div>
        <label>Volume (0-100):</label>
        <input type="number" id="volume" min="0" max="100">
    </div>
    <div>
        <label>Interval (seconds):</label>
        <input type="number" id="interval" min="1">
    </div>
    <div>
        <label>Enabled:</label>
        <input type="checkbox" id="enabled">
    </div>
    <div style="margin-top: 20px;">
        <button onclick="updateConfig(false)">Save</button>
        <button onclick="updateConfig(true)">Save & Apply Now</button>
        <button onclick="applyNow()">Apply Now</button>
    </div>
    <script>
        async function loadStatus() {
            const res = await fetch('/api/config');
            const data = await res.json();
            document.getElementById('volume').value = data.config.targetVolume;
            document.getElementById('interval').value = data.config.intervalSeconds;
            document.getElementById('enabled').checked = data.config.enabled;

            let status = 'Status: ' + data.config.lastApplyStatus;
            if (data.config.lastApplied) {
                status += ' (Last: ' + new Date(data.config.lastApplied).toLocaleString() + ')';
            }
            if (data.config.lastError) {
                status += '<br>Error: ' + data.config.lastError;
            }
            document.getElementById('status').innerHTML = status;
        }

        async function updateConfig(applyNow) {
            const payload = {
                targetVolume: parseInt(document.getElementById('volume').value),
                intervalSeconds: parseInt(document.getElementById('interval').value),
                enabled: document.getElementById('enabled').checked,
                applyNow: applyNow
            };
            await fetch('/api/config', {
                method: 'PUT',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify(payload)
            });
            await loadStatus();
        }

        async function applyNow() {
            await fetch('/api/apply', {method: 'POST'});
            await loadStatus();
        }

        loadStatus();
        setInterval(loadStatus, 3000);
    </script>
</body>
</html>`))
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

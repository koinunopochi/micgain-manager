package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"micgain-manager/internal/domain"
)

// FileRepository implements domain.ConfigRepository using JSON files.
// This is a secondary adapter.
type FileRepository struct {
	path string
	mu   sync.Mutex
}

// NewFileRepository creates a new file-based config repository.
func NewFileRepository(path string) (domain.ConfigRepository, error) {
	if path == "" {
		return nil, errors.New("path is required")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	return &FileRepository{path: path}, nil
}

// persistedData represents the JSON structure on disk.
type persistedData struct {
	TargetVolume    int    `json:"targetVolume"`
	IntervalSeconds int    `json:"intervalSeconds"`
	Enabled         bool   `json:"enabled"`
	LastApplied     string `json:"lastApplied,omitempty"`
	LastApplyStatus string `json:"lastApplyStatus"`
	LastError       string `json:"lastError,omitempty"`
}

// Load reads the configuration and state from disk.
func (f *FileRepository) Load() (domain.Config, domain.ScheduleState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := os.ReadFile(f.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Return defaults
			config := domain.DefaultConfig()
			state := domain.ScheduleState{
				LastApplyStatus: domain.StatusNever,
			}
			return config, state, nil
		}
		return domain.Config{}, domain.ScheduleState{}, fmt.Errorf("read config: %w", err)
	}

	var persisted persistedData
	if err := json.Unmarshal(data, &persisted); err != nil {
		return domain.Config{}, domain.ScheduleState{}, fmt.Errorf("unmarshal config: %w", err)
	}

	// Convert to domain models
	config := domain.Config{
		TargetVolume: persisted.TargetVolume,
		Interval:     time.Duration(persisted.IntervalSeconds) * time.Second,
		Enabled:      persisted.Enabled,
	}

	// Apply defaults if necessary
	if config.TargetVolume <= 0 {
		config.TargetVolume = 50
	}
	if config.Interval <= 0 {
		config.Interval = 90 * time.Second
	}

	state := domain.ScheduleState{
		LastApplyStatus: parseStatus(persisted.LastApplyStatus),
	}

	if persisted.LastApplied != "" {
		if t, err := time.Parse(time.RFC3339, persisted.LastApplied); err == nil {
			state.LastApplied = t
		}
	}

	if persisted.LastError != "" {
		state.LastError = errors.New(persisted.LastError)
	}

	return config, state, nil
}

// Save persists the configuration and state to disk.
func (f *FileRepository) Save(config domain.Config, state domain.ScheduleState) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	persisted := persistedData{
		TargetVolume:    config.TargetVolume,
		IntervalSeconds: int(config.Interval.Seconds()),
		Enabled:         config.Enabled,
		LastApplyStatus: state.LastApplyStatus.String(),
	}

	if !state.LastApplied.IsZero() {
		persisted.LastApplied = state.LastApplied.Format(time.RFC3339)
	}

	if state.LastError != nil {
		persisted.LastError = state.LastError.Error()
	}

	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Atomic write
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		return fmt.Errorf("rename tmp: %w", err)
	}

	return nil
}

func parseStatus(s string) domain.ApplyStatus {
	switch s {
	case "ok":
		return domain.StatusSuccess
	case "error":
		return domain.StatusError
	default:
		return domain.StatusNever
	}
}

// DefaultPath returns the default configuration file path.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "micgain-manager", "config.json")
}

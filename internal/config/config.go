package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config represents the persisted user preferences for mic gain management.
type Config struct {
	TargetVolume    int           `json:"targetVolume"`
	Interval        time.Duration `json:"interval"`
	Enabled         bool          `json:"enabled"`
	LastApplied     time.Time     `json:"lastApplied"`
	LastApplyStatus string        `json:"lastApplyStatus"`
	LastError       string        `json:"lastError"`
}

// StatusSnapshot is returned to clients (CLI/Web) describing runtime state.
type StatusSnapshot struct {
	Config        Config     `json:"config"`
	NextRun       time.Time  `json:"nextRun"`
	SchedulerIdle bool       `json:"schedulerIdle"`
	RunningSince  *time.Time `json:"runningSince"`
}

var (
	// DefaultInterval defines how often the volume will be enforced if not configured.
	DefaultInterval = 90 * time.Second
	// DefaultVolume is the fallback input volume percentage.
	DefaultVolume = 50
)

// DefaultConfig returns the initial configuration.
func DefaultConfig() Config {
	return Config{
		TargetVolume:    DefaultVolume,
		Interval:        DefaultInterval,
		Enabled:         true,
		LastApplyStatus: "never",
	}
}

// Store persists configuration to disk so CLIとWebインターフェイスで共有できる。
type Store interface {
	Load() (Config, error)
	Save(Config) error
}

// FileStore implements Store using a JSON file with optimistic locking.
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore creates a store under the supplied path. Parent directories are created automatically.
func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, errors.New("path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	return &FileStore{path: path}, nil
}

// Load reads the configuration file or returns defaults if it does not exist.
func (s *FileStore) Load() (Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	if cfg.Interval <= 0 {
		cfg.Interval = DefaultInterval
	}
	if cfg.TargetVolume <= 0 {
		cfg.TargetVolume = DefaultVolume
	}

	return cfg, nil
}

// Save writes the configuration to disk atomically.
func (s *FileStore) Save(cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmp := s.path + ".tmp"
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename tmp: %w", err)
	}
	return nil
}

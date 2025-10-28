package domain

import "time"

// Config represents the configuration entity in the domain.
// This is a pure domain model with no dependencies on external concerns.
type Config struct {
	TargetVolume int
	Interval     time.Duration
	Enabled      bool
}

// ScheduleState represents the current state of the scheduler.
type ScheduleState struct {
	LastApplied     time.Time
	LastApplyStatus ApplyStatus
	LastError       error
	NextRun         time.Time
	IsRunning       bool
}

// ApplyStatus represents the status of a volume application attempt.
type ApplyStatus int

const (
	StatusNever ApplyStatus = iota
	StatusSuccess
	StatusError
)

func (s ApplyStatus) String() string {
	switch s {
	case StatusNever:
		return "never"
	case StatusSuccess:
		return "ok"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// Snapshot represents a complete view of the system state.
type Snapshot struct {
	Config        Config
	ScheduleState ScheduleState
}

// Validate checks if the configuration values are valid.
func (c Config) Validate() error {
	if c.TargetVolume < 0 || c.TargetVolume > 100 {
		return ErrInvalidVolume
	}
	if c.Interval < time.Second {
		return ErrInvalidInterval
	}
	return nil
}

// DefaultConfig returns the default configuration values.
func DefaultConfig() Config {
	return Config{
		TargetVolume: 50,
		Interval:     90 * time.Second,
		Enabled:      true,
	}
}

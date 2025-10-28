package core

import (
	"time"

	"micgain-manager/internal/config"
)

// EffectType represents the type of side effect to be performed.
type EffectType string

const (
	EffectApplyVolume EffectType = "ApplyVolume"
	EffectSaveConfig  EffectType = "SaveConfig"
)

// Effect represents a side effect that should be performed by the adapter layer.
// The domain layer produces Effects without executing them, maintaining purity.
type Effect struct {
	Type   EffectType
	Volume int
	Config config.Config
}

// Event represents an input event to the domain.
type Event struct {
	Type EventType
	Data interface{}
}

// EventType represents the type of event.
type EventType string

const (
	EventTick         EventType = "Tick"
	EventUpdateConfig EventType = "UpdateConfig"
	EventApplyOnce    EventType = "ApplyOnce"
)

// UpdateConfigData contains data for config update events.
type UpdateConfigData struct {
	Config   config.Config
	ApplyNow bool
}

// ApplyOnceData contains data for one-time apply events.
type ApplyOnceData struct {
	Volume int // negative value means use current config
}

// State represents the current state of the domain.
type State struct {
	Config       config.Config
	NextRun      time.Time
	RunningSince *time.Time
}

// StateSnapshot returns a snapshot suitable for external consumption.
func (s State) StateSnapshot() config.StatusSnapshot {
	return config.StatusSnapshot{
		Config:        s.Config,
		NextRun:       s.NextRun,
		SchedulerIdle: s.RunningSince == nil,
		RunningSince:  s.RunningSince,
	}
}

package core

import (
	"context"
	"errors"
	"sync"
	"time"

	"micgain-manager/internal/config"
)

// VolumeApplier abstracts the OS specific way to set the mic gain.
type VolumeApplier interface {
	Apply(volume int) error
}

// Manager coordinates the execution of domain logic and effects.
// It acts as an adapter layer between pure domain logic and side effects.
type Manager struct {
	store   config.Store
	applier VolumeApplier

	mu    sync.RWMutex
	state State

	eventCh chan eventRequest
}

type eventRequest struct {
	event    Event
	resultCh chan error
}

// NewManager loads configuration and prepares the manager.
func NewManager(store config.Store, applier VolumeApplier) (*Manager, error) {
	if store == nil || applier == nil {
		return nil, errors.New("store and applier are required")
	}
	cfg, err := store.Load()
	if err != nil {
		return nil, err
	}
	if cfg, err = config.Normalize(cfg); err != nil {
		return nil, err
	}

	return &Manager{
		store:   store,
		applier: applier,
		state: State{
			Config:  cfg,
			NextRun: time.Now().Add(cfg.Interval),
		},
		eventCh: make(chan eventRequest),
	}, nil
}

// Start launches the event processing loop until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	go m.loop(ctx)
}

func (m *Manager) loop(ctx context.Context) {
	m.mu.RLock()
	interval := m.state.Config.Interval
	m.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case req := <-m.eventCh:
			m.mu.Lock()
			newState, effects, err := HandleEvent(m.state, req.event, time.Now())
			if err != nil {
				m.mu.Unlock()
				if req.resultCh != nil {
					req.resultCh <- err
				}
				continue
			}
			m.state = newState

			// Reset ticker if interval changed
			if newState.Config.Interval != interval {
				interval = newState.Config.Interval
				ticker.Reset(interval)
			}
			m.mu.Unlock()

			// Execute effects
			effectErr := m.executeEffects(effects)
			if req.resultCh != nil {
				req.resultCh <- effectErr
			}

		case <-ticker.C:
			m.mu.Lock()
			newState, effects, err := HandleEvent(m.state, Event{Type: EventTick}, time.Now())
			if err != nil {
				m.mu.Unlock()
				continue
			}
			m.state = newState
			m.mu.Unlock()

			m.executeEffects(effects)
		}
	}
}

// executeEffects executes the given effects and updates state based on results.
func (m *Manager) executeEffects(effects []Effect) error {
	var lastErr error
	for _, eff := range effects {
		var err error
		switch eff.Type {
		case EffectApplyVolume:
			err = m.applier.Apply(eff.Volume)
		case EffectSaveConfig:
			err = m.store.Save(eff.Config)
		}

		if err != nil {
			lastErr = err
			m.mu.Lock()
			m.state = HandleEffectResult(m.state, eff, err)
			m.mu.Unlock()
		}
	}
	return lastErr
}

// GetSnapshot returns a copy of the current config and scheduling info.
func (m *Manager) GetSnapshot() config.StatusSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.StateSnapshot()
}

// UpdateConfig replaces the configuration and optionally applies immediately.
func (m *Manager) UpdateConfig(cfg config.Config, applyNow bool) error {
	ch := make(chan error, 1)
	m.eventCh <- eventRequest{
		event: Event{
			Type: EventUpdateConfig,
			Data: UpdateConfigData{
				Config:   cfg,
				ApplyNow: applyNow,
			},
		},
		resultCh: ch,
	}
	return <-ch
}

// ApplyOnce enqueues an immediate apply with specified volume (or <0 for default).
func (m *Manager) ApplyOnce(volume int) error {
	ch := make(chan error, 1)
	m.eventCh <- eventRequest{
		event: Event{
			Type: EventApplyOnce,
			Data: ApplyOnceData{
				Volume: volume,
			},
		},
		resultCh: ch,
	}
	return <-ch
}

// CurrentConfig returns a copy of the stored config.
func (m *Manager) CurrentConfig() config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.Config
}

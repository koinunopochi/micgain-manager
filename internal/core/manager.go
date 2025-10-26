package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"micgain-manager/internal/config"
)

// VolumeApplier abstracts the OS specific way to set the mic gain.
type VolumeApplier interface {
	Apply(volume int) error
}

// Manager coordinates configuration persistence, scheduling, and execution.
type Manager struct {
	store   config.Store
	applier VolumeApplier

	mu           sync.RWMutex
	cfg          config.Config
	nextRun      time.Time
	runningSince time.Time

	updateCh chan updateRequest
	applyCh  chan applyRequest
}

type updateRequest struct {
	cfg      config.Config
	applyNow bool
	resultCh chan error
}

type applyRequest struct {
	volume   int
	resultCh chan error
}

// NewManager loads configuration and prepares the scheduler goroutine.
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
		store:    store,
		applier:  applier,
		cfg:      cfg,
		updateCh: make(chan updateRequest),
		applyCh:  make(chan applyRequest),
	}, nil
}

// Start launches the scheduler loop until ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	go m.loop(ctx)
}

func (m *Manager) loop(ctx context.Context) {
	ticker := time.NewTicker(m.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case req := <-m.updateCh:
			if err := m.handleUpdate(req.cfg); err != nil {
				if req.resultCh != nil {
					req.resultCh <- err
				}
				continue
			}
			ticker.Reset(m.cfg.Interval)
			if req.applyNow {
				m.triggerApply(m.cfg.TargetVolume, req.resultCh)
			} else if req.resultCh != nil {
				req.resultCh <- nil
			}
		case req := <-m.applyCh:
			m.triggerApply(req.volume, req.resultCh)
		case <-ticker.C:
			m.mu.RLock()
			enabled := m.cfg.Enabled
			volume := m.cfg.TargetVolume
			m.mu.RUnlock()
			if enabled {
				m.triggerApply(volume, nil)
			} else {
				m.mu.Lock()
				m.nextRun = time.Time{}
				m.mu.Unlock()
			}
		}
	}
}

func (m *Manager) triggerApply(volume int, resultCh chan error) {
	now := time.Now()
	m.mu.Lock()
	m.nextRun = now.Add(m.cfg.Interval)
	m.runningSince = now
	m.mu.Unlock()

	err := m.applier.Apply(volume)

	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.cfg.LastApplyStatus = "error"
		m.cfg.LastError = err.Error()
	} else {
		m.cfg.LastApplyStatus = "ok"
		m.cfg.LastError = ""
		m.cfg.LastApplied = now
	}
	_ = m.store.Save(m.cfg)
	m.runningSince = time.Time{}

	if resultCh != nil {
		resultCh <- err
	}
}

func (m *Manager) handleUpdate(cfg config.Config) error {
	var err error
	if cfg, err = config.Normalize(cfg); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	if err := m.store.Save(m.cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	m.nextRun = time.Now().Add(m.cfg.Interval)
	return nil
}

// GetSnapshot returns a copy of the current config and scheduling info.
func (m *Manager) GetSnapshot() config.StatusSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snap := config.StatusSnapshot{
		Config:        m.cfg,
		NextRun:       m.nextRun,
		SchedulerIdle: m.runningSince.IsZero(),
	}
	if !m.runningSince.IsZero() {
		rs := m.runningSince
		snap.RunningSince = &rs
	}
	return snap
}

// UpdateConfig replaces the configuration and optionally applies immediately.
func (m *Manager) UpdateConfig(cfg config.Config, applyNow bool) error {
	ch := make(chan error, 1)
	m.updateCh <- updateRequest{cfg: cfg, applyNow: applyNow, resultCh: ch}
	return <-ch
}

// ApplyOnce enqueues an immediate apply with specified volume (or <0 for default).
func (m *Manager) ApplyOnce(volume int) error {
	if volume < 0 {
		m.mu.RLock()
		volume = m.cfg.TargetVolume
		m.mu.RUnlock()
	}
	ch := make(chan error, 1)
	m.applyCh <- applyRequest{volume: volume, resultCh: ch}
	return <-ch
}

// CurrentConfig returns a copy of the stored config.
func (m *Manager) CurrentConfig() config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

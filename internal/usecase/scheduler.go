package usecase

import (
	"context"
	"sync"
	"time"

	"micgain-manager/internal/domain"
)

// SchedulerUseCase is the primary port for scheduler operations.
// This represents the application's use cases.
type SchedulerUseCase interface {
	Start(ctx context.Context)
	GetSnapshot() domain.Snapshot
	ApplyNow(volume int) error
	UpdateConfig(config domain.Config, applyNow bool) error
}

// schedulerInteractor implements SchedulerUseCase.
// It depends only on domain layer and secondary ports.
type schedulerInteractor struct {
	repo       domain.ConfigRepository
	controller domain.VolumeController
	service    *domain.SchedulerService

	mu     sync.RWMutex
	config domain.Config
	state  domain.ScheduleState
}

// NewSchedulerUseCase creates a new scheduler use case.
// Dependencies are injected (secondary ports).
func NewSchedulerUseCase(
	repo domain.ConfigRepository,
	controller domain.VolumeController,
) (SchedulerUseCase, error) {
	service := domain.NewSchedulerService()

	// Load initial state
	config, state, err := repo.Load()
	if err != nil {
		return nil, err
	}

	// Validate and normalize
	config, err = service.ValidateAndNormalize(config)
	if err != nil {
		return nil, err
	}

	return &schedulerInteractor{
		repo:       repo,
		controller: controller,
		service:    service,
		config:     config,
		state:      state,
	}, nil
}

// Start begins the scheduler loop.
func (s *schedulerInteractor) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *schedulerInteractor) loop(ctx context.Context) {
	s.mu.RLock()
	interval := s.config.Interval
	s.mu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()

			if s.service.ShouldApply(s.state, s.config, now) {
				// Mark as running
				s.state = s.service.StartRunning(s.state)
				volume := s.config.TargetVolume
				config := s.config
				s.mu.Unlock()

				// Execute side effect through secondary port
				err := s.controller.SetVolume(volume)

				s.mu.Lock()
				if err != nil {
					s.state = s.service.ApplyFailure(s.state, config, err, now)
				} else {
					s.state = s.service.ApplySuccess(s.state, config, now)
				}
				// Persist state
				_ = s.repo.Save(s.config, s.state)

				// Update ticker if interval changed
				if s.config.Interval != interval {
					interval = s.config.Interval
					ticker.Reset(interval)
				}
				s.mu.Unlock()
			} else {
				s.mu.Unlock()
			}
		}
	}
}

// GetSnapshot returns the current system state.
func (s *schedulerInteractor) GetSnapshot() domain.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return domain.Snapshot{
		Config:        s.config,
		ScheduleState: s.state,
	}
}

// ApplyNow immediately applies the specified volume.
func (s *schedulerInteractor) ApplyNow(volume int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use current config volume if negative
	if volume < 0 {
		volume = s.config.TargetVolume
	}

	// Validate volume
	if volume < 0 || volume > 100 {
		return domain.ErrInvalidVolume
	}

	now := time.Now()
	s.state = s.service.StartRunning(s.state)

	// Execute side effect
	err := s.controller.SetVolume(volume)

	if err != nil {
		s.state = s.service.ApplyFailure(s.state, s.config, err, now)
	} else {
		s.state = s.service.ApplySuccess(s.state, s.config, now)
	}

	// Persist state
	_ = s.repo.Save(s.config, s.state)

	return err
}

// UpdateConfig updates the configuration and optionally applies immediately.
func (s *schedulerInteractor) UpdateConfig(config domain.Config, applyNow bool) error {
	// Validate through domain service
	config, err := s.service.ValidateAndNormalize(config)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.config = config
	s.state.NextRun = s.service.CalculateNextRun(time.Now(), config.Interval)
	s.mu.Unlock()

	// Persist
	if err := s.repo.Save(config, s.state); err != nil {
		return err
	}

	if applyNow {
		return s.ApplyNow(config.TargetVolume)
	}

	return nil
}

package domain

import "time"

// SchedulerService provides pure domain logic for the scheduler.
// This service has no side effects and no dependencies on external concerns.
type SchedulerService struct{}

// NewSchedulerService creates a new scheduler service.
func NewSchedulerService() *SchedulerService {
	return &SchedulerService{}
}

// ShouldApply determines if volume should be applied based on current state and time.
// This is a pure function with no side effects.
func (s *SchedulerService) ShouldApply(state ScheduleState, config Config, now time.Time) bool {
	if !config.Enabled {
		return false
	}

	// If never run or next run time has passed
	if state.NextRun.IsZero() || now.After(state.NextRun) {
		return true
	}

	return false
}

// CalculateNextRun determines the next scheduled run time.
func (s *SchedulerService) CalculateNextRun(lastApplied time.Time, interval time.Duration) time.Time {
	if lastApplied.IsZero() {
		return time.Now().Add(interval)
	}
	return lastApplied.Add(interval)
}

// ApplySuccess updates the state after a successful volume application.
func (s *SchedulerService) ApplySuccess(state ScheduleState, config Config, appliedAt time.Time) ScheduleState {
	return ScheduleState{
		LastApplied:     appliedAt,
		LastApplyStatus: StatusSuccess,
		LastError:       nil,
		NextRun:         s.CalculateNextRun(appliedAt, config.Interval),
		IsRunning:       false,
	}
}

// ApplyFailure updates the state after a failed volume application.
func (s *SchedulerService) ApplyFailure(state ScheduleState, config Config, err error, attemptedAt time.Time) ScheduleState {
	return ScheduleState{
		LastApplied:     state.LastApplied, // Keep previous success time
		LastApplyStatus: StatusError,
		LastError:       err,
		NextRun:         s.CalculateNextRun(attemptedAt, config.Interval),
		IsRunning:       false,
	}
}

// StartRunning marks the state as currently applying volume.
func (s *SchedulerService) StartRunning(state ScheduleState) ScheduleState {
	return ScheduleState{
		LastApplied:     state.LastApplied,
		LastApplyStatus: state.LastApplyStatus,
		LastError:       state.LastError,
		NextRun:         state.NextRun,
		IsRunning:       true,
	}
}

// ValidateAndNormalize validates a config and returns a normalized version.
func (s *SchedulerService) ValidateAndNormalize(config Config) (Config, error) {
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

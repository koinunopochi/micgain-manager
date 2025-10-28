package domain

import "errors"

var (
	// ErrInvalidVolume indicates that the volume value is out of range.
	ErrInvalidVolume = errors.New("volume must be between 0 and 100")

	// ErrInvalidInterval indicates that the interval is too short.
	ErrInvalidInterval = errors.New("interval must be at least 1 second")

	// ErrNotEnabled indicates that the scheduler is not enabled.
	ErrNotEnabled = errors.New("scheduler is not enabled")
)

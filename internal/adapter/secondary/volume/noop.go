package volume

import "micgain-manager/internal/domain"

// NoopController implements domain.VolumeController with no-op behavior.
// Useful for testing or non-macOS environments.
type NoopController struct{}

// NewNoopController creates a new no-op volume controller.
func NewNoopController() domain.VolumeController {
	return &NoopController{}
}

// SetVolume does nothing and always succeeds.
func (n *NoopController) SetVolume(volume int) error {
	return nil
}

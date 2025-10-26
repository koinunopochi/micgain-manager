package volume

import "fmt"

// NoopApplier logs the desired volume without touching the OS. Useful for tests/CI.
type NoopApplier struct{}

func (NoopApplier) Apply(volume int) error {
	if volume < 0 || volume > 100 {
		return fmt.Errorf("invalid volume %d", volume)
	}
	return nil
}

package volume

import (
	"fmt"
	"os/exec"
)

// AppleScriptApplier runs the native `osascript` command to lock the input volume.
type AppleScriptApplier struct{}

// Apply sets the macOS input volume (0-100).
func (AppleScriptApplier) Apply(volume int) error {
	if volume < 0 || volume > 100 {
		return fmt.Errorf("volume must be between 0 and 100 (got %d)", volume)
	}
	script := fmt.Sprintf("set volume input volume %d", volume)
	cmd := exec.Command("osascript", "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript failed: %w (%s)", err, string(output))
	}
	return nil
}

package volume

import (
	"fmt"
	"os/exec"

	"micgain-manager/internal/domain"
)

// AppleScriptController implements domain.VolumeController using macOS osascript.
// This is a secondary adapter.
type AppleScriptController struct{}

// NewAppleScriptController creates a new AppleScript volume controller.
func NewAppleScriptController() domain.VolumeController {
	return &AppleScriptController{}
}

// SetVolume sets the microphone input volume using osascript.
func (a *AppleScriptController) SetVolume(volume int) error {
	if volume < 0 || volume > 100 {
		return fmt.Errorf("volume must be between 0 and 100, got %d", volume)
	}

	cmd := exec.Command("osascript", "-e", fmt.Sprintf("set volume input volume %d", volume))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript failed: %w, output: %s", err, string(output))
	}

	return nil
}

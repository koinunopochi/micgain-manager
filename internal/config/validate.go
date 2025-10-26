package config

import (
	"fmt"
	"time"
)

// Normalize clamps invalid values and returns a safe copy.
func Normalize(cfg Config) (Config, error) {
	if cfg.TargetVolume < 0 || cfg.TargetVolume > 100 {
		return cfg, fmt.Errorf("targetVolume must be between 0 and 100")
	}
	if cfg.Interval < 5*time.Second {
		return cfg, fmt.Errorf("interval must be >=5s")
	}
	return cfg, nil
}

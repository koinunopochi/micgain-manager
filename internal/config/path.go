package config

import (
	"os"
	"path/filepath"
)

// DefaultPath returns ~/.config/micgain-manager/config.json (or TMP fallback).
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".config", "micgain-manager", "config.json")
	}
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "micgain-manager-config.json")
}

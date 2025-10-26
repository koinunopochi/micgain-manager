package logging

import (
	"fmt"
	"log"
	"strings"
)

// Level represents logging severity.
type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelDebug
	LevelTrace
)

var (
	currentLevel     = LevelWarn
	currentVerbosity = 0
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
}

// SetVerbosity configures logger output from count of -v flags (0-4).
func SetVerbosity(count int) {
	if count < 0 {
		count = 0
	}
	if count > 4 {
		count = 4
	}
	currentVerbosity = count
	switch count {
	case 0:
		currentLevel = LevelWarn
	case 1:
		currentLevel = LevelInfo
	case 2:
		currentLevel = LevelDebug
	default:
		currentLevel = LevelTrace
	}
}

// Verbosity returns the stored -v count.
func Verbosity() int {
	return currentVerbosity
}

// LevelName returns current level label.
func LevelName() string {
	return LevelToString(currentLevel)
}

// LevelToString converts a Level to human readable text.
func LevelToString(l Level) string {
	switch l {
	case LevelError:
		return "error"
	case LevelWarn:
		return "warn"
	case LevelInfo:
		return "info"
	case LevelDebug:
		return "debug"
	case LevelTrace:
		return "trace"
	default:
		return "unknown"
	}
}

// ParseLevel returns Level + verbosity count from string.
func ParseLevel(s string) (Level, int, error) {
	switch strings.ToLower(s) {
	case "error":
		return LevelError, 0, nil
	case "warn", "warning":
		return LevelWarn, 0, nil
	case "info":
		return LevelInfo, 1, nil
	case "debug":
		return LevelDebug, 2, nil
	case "trace":
		return LevelTrace, 4, nil
	default:
		return LevelWarn, currentVerbosity, fmt.Errorf("unknown level %s", s)
	}
}

func shouldLog(l Level) bool {
	return l <= currentLevel
}

func logf(l Level, prefix, format string, args ...any) {
	if !shouldLog(l) {
		return
	}
	msg := fmt.Sprintf(format, args...)
	log.Printf("[%s] %s", strings.ToUpper(prefix), msg)
}

// Errorf always prints.
func Errorf(format string, args ...any) {
	logf(LevelError, "err", format, args...)
}

func Warnf(format string, args ...any) {
	logf(LevelWarn, "warn", format, args...)
}

func Infof(format string, args ...any) {
	logf(LevelInfo, "info", format, args...)
}

func Debugf(format string, args ...any) {
	logf(LevelDebug, "dbg", format, args...)
}

func Tracef(format string, args ...any) {
	logf(LevelTrace, "trc", format, args...)
}

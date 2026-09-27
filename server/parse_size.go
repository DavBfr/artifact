package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// durationEnv reads a duration from the environment, falling back when unset. An
// unparsable value stops the boot rather than being silently ignored: it is
// usually a typo ("5min" instead of "5m") that would otherwise disable whatever
// the setting controls without a word.
func durationEnv(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	value, err := parseDuration(raw)
	if err != nil {
		log.Fatalf("invalid %s: %v", name, err)
	}
	return value
}

func parseBool(s string, defaultValue bool) bool {
	if s == "" {
		return defaultValue
	}
	if value, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
		return value
	}
	return defaultValue
}

func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	if value, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return value
	}
	return defaultValue
}

func parseSize(s string, defaultSize int64) int64 {
	if s == "" {
		return defaultSize
	}

	s = strings.TrimSpace(strings.ToUpper(s))
	units := map[string]int64{
		"K":  1024,
		"KB": 1024,
		"M":  1024 * 1024,
		"MB": 1024 * 1024,
		"G":  1024 * 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
	}

	for unit, multiplier := range units {
		if strings.HasSuffix(s, unit) {
			valueStr := s[:len(s)-len(unit)]
			if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
				return int64(value * float64(multiplier))
			}
			return defaultSize
		}
	}

	if value, err := strconv.ParseInt(s, 10, 64); err == nil {
		return value
	}

	return defaultSize
}

// parseDuration parses a Go duration ("12h", "90m") and additionally accepts a
// "d" suffix for days ("30d"), which time.ParseDuration does not support. A
// plain "0" is valid and means "no expiry" for the callers that allow it.
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	if days, ok := strings.CutSuffix(s, "d"); ok {
		value, err := strconv.ParseFloat(days, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q", s)
		}
		if value < 0 {
			return 0, fmt.Errorf("negative duration %q", s)
		}
		return time.Duration(value * float64(24*time.Hour)), nil
	}

	value, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q", s)
	}
	if value < 0 {
		return 0, fmt.Errorf("negative duration %q", s)
	}

	return value, nil
}

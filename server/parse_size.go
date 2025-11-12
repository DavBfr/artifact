package main

import (
	"strconv"
	"strings"
)

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

package main

import (
	"fmt"
	"time"
)

// parseTime parses a UTC RFC3339 timestamp and returns the moment in
// UTC. An empty string returns the zero time.
func parseTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", raw, err)
	}
	return parsed.UTC(), nil
}

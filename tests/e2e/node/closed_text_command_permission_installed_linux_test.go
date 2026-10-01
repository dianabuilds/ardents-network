//go:build linux && text_worker_installed

package state_test

import (
	"testing"
	"time"
)

func TestInstalledCommandPositivePermissionWindow(t *testing.T) {
	end := time.Date(2026, time.September, 29, 16, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name  string
		now   time.Time
		valid bool
	}{
		{"ordinary hour", end.Add(-30 * time.Minute), true},
		{"last admissible second", end.Add(-installedCommandEndpointLifetime - time.Second), true},
		{"exact bound", end.Add(-installedCommandEndpointLifetime), false},
		{"near UTC hour boundary", end.Add(-5 * time.Minute), false},
		{"already expired", end, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := installedCommandPositivePermissionWindow(test.now, end); (err == nil) != test.valid {
				t.Fatalf("Permission window at %s: %v", test.now, err)
			}
		})
	}
}

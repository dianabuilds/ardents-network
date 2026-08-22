//go:build !linux

package resource_test

import (
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
)

func TestDefaultAdapterRefusesUnsupportedPlatform(t *testing.T) {
	guard, err := resource.New(resource.Config{Profile: "h3-np1-v1", Interval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := guard.Check(); err == nil || err.Error() != "resource guard is unsupported on this platform" {
		t.Fatalf("Check() error = %v, want unsupported-platform refusal", err)
	}
	observation, err := guard.Observe(0, 0, 0)
	if err == nil || err.Error() != "resource guard is unsupported on this platform" || !observation.Protect || !observation.Drain {
		t.Fatalf("Observe() = %+v, %v, want protected drain refusal", observation, err)
	}
}

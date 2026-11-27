//go:build !linux && !darwin

package counters

import (
	"errors"
	"testing"
)

func TestUnsupportedHostRefusesNativeCounterRead(t *testing.T) {
	reading, err := Read([]string{"unavailable-interface"})
	if Supported() || !errors.Is(err, ErrUnsupportedPlatform) || reading.Boot != "" || len(reading.Interfaces) != 0 {
		t.Fatal("unsupported platform supplied a counter observation", reading, err)
	}
}

package systemd

import (
	"errors"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/systemdproperty"
)

func TestDetachedPropertyRefusalRetainsManagerErrorIdentity(t *testing.T) {
	var value Value
	err := Decode([]byte(`{"type":"b","type":"b","data":true}`), &value)
	if !errors.Is(err, ErrObservation) || !errors.Is(err, systemdproperty.ErrObservation) {
		t.Fatalf("observation refusal identity changed: %v", err)
	}
}

//go:build !linux

package inspection

import (
	"context"
	"reflect"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
)

// The Linux Source preparation mechanism is absent on this platform, so the
// authenticated Source contact must return the same explicit typed unavailable
// outcome (ADR-0129): no input or credential is retained and no contact is
// performed. This is the refusal boundary, never a weaker accepting path.
func TestUnsupportedInspectionPlatformRefusesBeforeAnyContact(t *testing.T) {
	response, err := InspectLatest(context.Background(), Request{MaterialIndex: 0})
	if err != errInspectionPlatformUnavailable {
		t.Fatal("unsupported InspectLatest accepted or lost the typed refusal", err)
	}
	if !reflect.DeepEqual(response, source.Message{}) {
		t.Fatal("unsupported inspection returned a contact response", response)
	}
}

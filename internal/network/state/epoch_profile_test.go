package state

import "testing"

func TestClosedRouteProfileRequiresGenerationThreeCarrier(t *testing.T) {
	for _, test := range []struct {
		profile, carrier string
		want             bool
	}{
		{closedRouteProfile, closedTCPCarrierProfile, true},
		{closedRouteProfile, closedQUICCarrierProfile, true},
		{closedRouteProfile, legacyTCPCarrierProfile, false},
		{closedRouteProfile, quicCarrierProfile, false},
		{interactiveRouteProfile, legacyTCPCarrierProfile, true},
		{interactiveRouteProfile, quicCarrierProfile, true},
	} {
		if got := validCarrierForEpoch(test.profile, test.carrier); got != test.want {
			t.Fatalf("validCarrierForEpoch(%q, %q) = %t, want %t", test.profile, test.carrier, got, test.want)
		}
	}
}

package receiver

import "testing"

func TestValidateCarrierRequiresExactClosedTCPProfile(t *testing.T) {
	for _, tc := range []struct {
		name    string
		profile string
		wantOK  bool
	}{
		{name: "closed TCP", profile: "ardents-carrier-tcp-tls-v2", wantOK: true},
		{name: "deferred QUIC", profile: "ardents-carrier-quic-v2"},
		{name: "previous version", profile: "ardents-carrier-tcp-tls-v1"},
		{name: "trailing whitespace", profile: "ardents-carrier-tcp-tls-v2 "},
		{name: "missing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCarrier(tc.profile)
			if (err == nil) != tc.wantOK {
				t.Fatalf("ValidateCarrier(%q) = %v, want acceptance %t", tc.profile, err, tc.wantOK)
			}
		})
	}
}

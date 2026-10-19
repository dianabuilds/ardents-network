package route

import "testing"

func TestClosedReleaseCarrierAdmission(t *testing.T) {
	if !CarrierPermitted("ardents-carrier-tcp-tls-v2") {
		t.Fatal("selected TCP/TLS profile refused")
	}
	for _, profile := range []string{"", "ardents-carrier-quic-v2", "ardents-carrier-quic-v1", "ardents-carrier-tcp-tls-v1", "other"} {
		if CarrierPermitted(profile) {
			t.Fatalf("unselected profile %q admitted", profile)
		}
	}
}

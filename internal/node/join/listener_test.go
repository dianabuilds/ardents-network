package join

import (
	"crypto/tls"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/node/authority"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestValidateRefusesUnavailableCurrentReceiver(t *testing.T) {
	profile := Profile{HostingRoot: t.TempDir(), AdmissionRoot: t.TempDir(), Certificate: tls.Certificate{PrivateKey: new(int)},
		ConnectionLimit: 1, DrainTimeout: time.Second}
	snapshot := state.NodeDuty{ProbeEndpoint: "127.0.0.1:41000", CarrierProfile: string(carrier.ClosedCarrierTCP)}
	err := Validate(profile, authority.Source{}, snapshot, time.Now(), true)
	if err == nil || !strings.Contains(err.Error(), "State projection is unavailable") {
		t.Fatalf("unavailable current JOIN receiver: %v", err)
	}
}

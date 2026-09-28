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

func TestStartRejectsMissingBorrowedDependencies(t *testing.T) {
	for _, test := range []struct {
		name string
		omit func(*Config)
	}{
		{"current route", func(config *Config) { config.Authority.CurrentRoute = nil }},
		{"current duty", func(config *Config) { config.CurrentDuty = nil }},
		{"open host", func(config *Config) { config.OpenHost = nil }},
		{"clock", func(config *Config) { config.Now = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := Config{Authority: authority.Source{CurrentRoute: func() (state.ClosedRouteView, error) {
				return state.ClosedRouteView{}, nil
			}}, CurrentDuty: func() (state.NodeDuty, error) { return state.NodeDuty{}, nil },
				OpenHost: func(string) (Host, error) { return nil, nil }, Now: time.Now}
			test.omit(&config)
			running, err := Start(config)
			if running != nil || err == nil || !strings.Contains(err.Error(), "dependencies are incomplete") {
				t.Fatalf("Start = %v, %v", running, err)
			}
		})
	}
}

func TestValidateRefusesUnavailableCurrentReceiver(t *testing.T) {
	profile := Profile{HostingRoot: t.TempDir(), AdmissionRoot: t.TempDir(), Certificate: tls.Certificate{PrivateKey: new(int)},
		ConnectionLimit: 1, DrainTimeout: time.Second}
	snapshot := state.NodeDuty{ProbeEndpoint: "127.0.0.1:41000", CarrierProfile: string(carrier.ClosedCarrierTCP)}
	err := Validate(profile, authority.Source{}, snapshot, time.Now(), true)
	if err == nil || !strings.Contains(err.Error(), "State projection is unavailable") {
		t.Fatalf("unavailable current JOIN receiver: %v", err)
	}
}

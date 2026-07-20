//go:build integration

package networkfoundation_test

import (
	"strings"
	"testing"

	transport "ardents/internal/network/api"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestTransportConstrainedModeExposesTCPOnlyEndpoints(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "network-foundation",
		ScenarioID:  "NFI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "network", "transport-variant"},
		Speed:       "default",
		Environment: "local",
	})

	var svc transport.Service
	var endpoints []string

	scenario.Precondition("start constrained transport runtime", func(t *testing.T) {
		svc = testkit.StartTransport(t)
	})

	scenario.Step("capture published transport endpoints", func(t *testing.T) {
		endpoints = svc.Endpoints()
		require.NotEmpty(t, endpoints)
		require.Equal(t, "ready", svc.State())
	})

	scenario.Assert("constrained mode exposes only tcp-backed multiaddrs", func(t *testing.T) {
		for _, endpoint := range endpoints {
			require.Truef(t, strings.Contains(endpoint, "/tcp/"), "endpoint %q must remain tcp-backed", endpoint)
			require.Falsef(t, strings.Contains(endpoint, "/udp/"), "endpoint %q must not expose udp transport", endpoint)
			require.Falsef(t, strings.Contains(endpoint, "/quic-v1"), "endpoint %q must not expose quic", endpoint)
			require.Falsef(t, strings.Contains(endpoint, "/webtransport"), "endpoint %q must not expose webtransport", endpoint)
			require.Falsef(t, strings.Contains(endpoint, "/webrtc"), "endpoint %q must not expose webrtc", endpoint)
			require.Falsef(t, strings.Contains(endpoint, "/webrtc-direct"), "endpoint %q must not expose webrtc-direct", endpoint)
		}
	})
}

func TestTransportTCPWSSExposesTCPAndWSSOnly(t *testing.T) {
	scenario := testkit.BeginScenario(t, testkit.Spec{
		Layer:       testkit.LayerIntegration,
		Domain:      "network-foundation",
		ScenarioID:  "NFI-002",
		Suite:       "integration",
		Tags:        []string{"integration", "network", "transport-variant"},
		Speed:       "default",
		Environment: "local",
	})

	const advertisedHost = "wss.example.test"
	certPath, keyPath := testkit.WriteWSSCertForHost(t, advertisedHost)
	svc := testkit.StartTransportWithConfig(t, transport.Config{
		Profile:             transport.ProfileTCPWSS,
		WSSPort:             testkit.ReserveLoopbackTCPPort(t),
		WSSCertPath:         certPath,
		WSSKeyPath:          keyPath,
		WSSCAPath:           testkit.WSSCAPath(certPath),
		WSSAdvertiseAddress: advertisedHost,
	})

	var endpoints []string
	scenario.Step("capture published transport endpoints for tcp_wss profile", func(t *testing.T) {
		endpoints = svc.Endpoints()
		require.NotEmpty(t, endpoints)
		require.Equal(t, "ready", svc.State())
	})

	scenario.Assert("tcp_wss exposes tcp and secure websocket endpoints without widening to quic families", func(t *testing.T) {
		var hasTCP bool
		var hasWSS bool
		for _, endpoint := range endpoints {
			if strings.Contains(endpoint, "/tcp/") && !strings.Contains(endpoint, "/ws") {
				hasTCP = true
			}
			if strings.Contains(endpoint, "/wss") || strings.Contains(endpoint, "/tls/ws") {
				hasWSS = true
			}
			require.Falsef(t, strings.Contains(endpoint, "/udp/"), "endpoint %q must not expose udp transport", endpoint)
			require.Falsef(t, strings.Contains(endpoint, "/quic-v1"), "endpoint %q must not expose quic", endpoint)
			require.Falsef(t, strings.Contains(endpoint, "/webtransport"), "endpoint %q must not expose webtransport", endpoint)
			require.Falsef(t, strings.Contains(endpoint, "/webrtc"), "endpoint %q must not expose webrtc", endpoint)
			require.Falsef(t, strings.Contains(endpoint, "/webrtc-direct"), "endpoint %q must not expose webrtc-direct", endpoint)
		}
		require.True(t, hasTCP, "expected tcp endpoint in %v", endpoints)
		require.True(t, hasWSS, "expected secure websocket endpoint in %v", endpoints)
		require.Contains(t, strings.Join(endpoints, ","), "/dns4/"+advertisedHost+"/")
	})

	scenario.Step("rotate certificate material and restart transport", func(t *testing.T) {
		require.NoError(t, svc.Stop(t.Context()))
		testkit.RotateWSSCert(t, certPath, keyPath, advertisedHost)
		require.NoError(t, svc.Start(t.Context()))
	})

	scenario.Assert("restart revalidates rotated material and republishes the advertised WSS endpoint", func(t *testing.T) {
		require.Equal(t, "ready", svc.State())
		require.Contains(t, strings.Join(svc.Endpoints(), ","), "/dns4/"+advertisedHost+"/")
	})
}

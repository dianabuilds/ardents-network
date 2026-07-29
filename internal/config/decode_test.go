package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeAppliesVersionedDefaults(t *testing.T) {
	doc, err := Decode(strings.NewReader(`{
        "api_version":"ardents.config/v1",
        "node":{"name":"node-a"}
    }`))
	require.NoError(t, err)
	require.Equal(t, "node-a", doc.Node.Name)
	require.Equal(t, "var/node-a", filepath.ToSlash(doc.Node.DataDir))
	require.Equal(t, "var/node-a/waku-store.db", filepath.ToSlash(doc.Network.StorePath))
	require.Equal(t, "service_node", doc.Node.Profile)
	require.Equal(t, "tcp_only", doc.Network.TransportProfile)
	require.Equal(t, "outbound_only", doc.Network.ReachabilityMode)
	require.Equal(t, 100000, doc.Network.Limits.StoreMaxMessages)
	require.Equal(t, 7*24*60*60, doc.Network.Limits.StoreMaxAgeSeconds)
	require.Equal(t, int64(2<<30), doc.Network.Limits.StoreMaxBytes)
	require.Equal(t, "/run/ardents/control.sock", doc.API.SocketPath)
	require.Equal(t, 3, doc.Data.DesiredReplicas)
	require.Equal(t, 2, doc.Data.MinimumReplicas)
}

func TestDecodeAcceptsPurposeScopedTrustedPrincipals(t *testing.T) {
	entry := trustedPrincipalConfig(t, "channel.issue", "discovery.publish")
	raw := `{"api_version":"ardents.config/v1","node":{"name":"node-a"},"trust":{"principals":[{"principal":"` + entry.Principal + `","public_key":"` + entry.PublicKey + `","purposes":["channel.issue","discovery.publish"]}]}}`
	doc, err := Decode(strings.NewReader(raw))
	require.NoError(t, err)
	require.Equal(t, []TrustedPrincipalConfig{entry}, doc.Trust.Principals)
}

func TestDecodeAcceptsTypedWorkloadRequirements(t *testing.T) {
	raw := `{
		"api_version":"ardents.config/v1",
		"workloads":{
			"executor":"disabled",
			"initial":[]
		},
		"policy":{"denied_workload_requirements":["gpu","network.read"]}
	}`
	doc, err := Decode(strings.NewReader(raw))
	require.NoError(t, err)
	require.Equal(t, []string{"gpu", "network.read"}, []string{
		doc.Policy.DeniedWorkloadRequirements[0].String(),
		doc.Policy.DeniedWorkloadRequirements[1].String(),
	})
}

func TestDecodeRejectsUnknownDuplicateAndDeprecatedFields(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"unknown", `{"api_version":"ardents.config/v1","mystery":true}`, "unknown field"},
		{"duplicate", `{"api_version":"ardents.config/v1","api_version":"ardents.config/v1"}`, "duplicate field"},
		{"obsolete transport mode", `{"api_version":"ardents.config/v1","network":{"transport_mode":"tcp"}}`, "unknown field"},
		{"obsolete channel grant store", `{"api_version":"ardents.config/v1","privacy":{"capability_store":"store"}}`, "unknown field"},
		{"obsolete channel grant store key", `{"api_version":"ardents.config/v1","privacy":{"capability_store_key_file":"key"}}`, "unknown field"},
		{"obsolete private channel grant switch", `{"api_version":"ardents.config/v1","policy":{"disable_private_capability_use":true}}`, "unknown field"},
		{"obsolete denied channel grant scopes", `{"api_version":"ardents.config/v1","policy":{"denied_capability_scopes":["realm.discovery"]}}`, "unknown field"},
		{"operator bearer field", `{"api_version":"ardents.config/v1","api":{"token_file":"token"}}`, "unknown field"},
		{"operator plaintext listener", `{"api_version":"ardents.config/v1","api":{"listen_address":"127.0.0.1:8080"}}`, "unknown field"},
		{"application bearer field", `{"api_version":"ardents.config/v1","application_interface":{"token_file":"token"}}`, "unknown field"},
		{"application plaintext listener", `{"api_version":"ardents.config/v1","application_interface":{"listen_address":"127.0.0.1:8081"}}`, "unknown field"},
		{"legacy discovery trust anchors", `{"api_version":"ardents.config/v1","network":{"trust_anchors":["public"]}}`, "unknown field"},
		{"legacy privacy trusted issuers", `{"api_version":"ardents.config/v1","privacy":{"trusted_issuers":{"p_issuer":"public"}}}`, "unknown field"},
		{"legacy workload capabilities", `{"api_version":"ardents.config/v1","workloads":{"initial":[{"id":"work.echo","kind":"service","owner":"node","desired":"present","capabilities":["gpu"]}]}}`, "unknown field"},
		{"legacy denied capabilities", `{"api_version":"ardents.config/v1","policy":{"denied_capabilities":["gpu"]}}`, "unknown field"},
		{"malformed workload requirement", `{"api_version":"ardents.config/v1","workloads":{"initial":[{"id":"work.echo","kind":"service","owner":"node","desired":"present","requirements":[" GPU "]}]}}`, "invalid workload requirement"},
		{"malformed denied workload requirement", `{"api_version":"ardents.config/v1","policy":{"denied_workload_requirements":["gpu/admin"]}}`, "invalid workload requirement"},
		{"version", `{"api_version":"ardents.config/v2"}`, "unsupported api_version"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(tc.raw))
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestObsoleteCredentialEnvironmentIsRejectedEvenWhenEmpty(t *testing.T) {
	for _, name := range []string{
		"ARDENTS_API_TOKEN",
		"ARDENTS_API_TOKEN_FILE",
		"ARDENTS_APPLICATION_TOKEN",
		"ARDENTS_APPLICATION_TOKEN_FILE",
		"ARDENTS_LEGACY_API_TOKEN",
		"ARDENTS_LEGACY_TOKEN_FILE",
		"ARDENTS_TOKEN",
		"ARDENTS_TOKEN_FILE",
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "")
			err := RejectObsoleteCredentialEnvironment()
			require.Error(t, err)
			require.Contains(t, err.Error(), name)
		})
	}
}

func TestDecodeRejectsOversizedDocument(t *testing.T) {
	raw := `{"api_version":"ardents.config/v1","node":{"name":"` + strings.Repeat("x", MaxDocumentBytes) + `"}}`
	_, err := Decode(strings.NewReader(raw))
	require.ErrorContains(t, err, "exceeds")
}

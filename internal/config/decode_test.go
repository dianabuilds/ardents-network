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
	require.Equal(t, "/run/ardents/control.sock", doc.API.SocketPath)
	require.Equal(t, 3, doc.Data.DesiredReplicas)
	require.Equal(t, 2, doc.Data.MinimumReplicas)
}

func TestDecodeRejectsUnknownDuplicateAndDeprecatedFields(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"unknown", `{"api_version":"ardents.config/v1","mystery":true}`, "unknown field"},
		{"duplicate", `{"api_version":"ardents.config/v1","api_version":"ardents.config/v1"}`, "duplicate field"},
		{"deprecated", `{"api_version":"ardents.config/v1","network":{"transport_mode":"tcp"}}`, "network.transport_profile"},
		{"operator bearer field", `{"api_version":"ardents.config/v1","api":{"token_file":"token"}}`, "unknown field"},
		{"operator plaintext listener", `{"api_version":"ardents.config/v1","api":{"listen_address":"127.0.0.1:8080"}}`, "unknown field"},
		{"application bearer field", `{"api_version":"ardents.config/v1","application_interface":{"token_file":"token"}}`, "unknown field"},
		{"application plaintext listener", `{"api_version":"ardents.config/v1","application_interface":{"listen_address":"127.0.0.1:8081"}}`, "unknown field"},
		{"version", `{"api_version":"ardents.config/v2"}`, "unsupported api_version"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(tc.raw))
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestDecodeRejectsOversizedDocument(t *testing.T) {
	raw := `{"api_version":"ardents.config/v1","node":{"name":"` + strings.Repeat("x", MaxDocumentBytes) + `"}}`
	_, err := Decode(strings.NewReader(raw))
	require.ErrorContains(t, err, "exceeds")
}

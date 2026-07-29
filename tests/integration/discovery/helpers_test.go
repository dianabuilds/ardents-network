//go:build integration

package discovery_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"ardents/internal/discovery"
	discoveryrecords "ardents/internal/discovery/records"
	identityprincipal "ardents/internal/identity/principal"

	"github.com/stretchr/testify/require"
)

func TestDiscoveryIntegrationReadyHelper(t *testing.T) {
	if os.Getenv("ARDENTS_DISCOVERY_INTEGRATION_HELPER") != "1" {
		return
	}
	generation := os.Getenv("ARDENTS_WORKLOAD_GENERATION")
	server := &http.Server{Addr: os.Getenv("ARDENTS_DISCOVERY_INTEGRATION_ADDRESS"), Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Ardents-Generation", generation)
		w.WriteHeader(http.StatusNoContent)
	})}
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		os.Exit(2)
	}
}

//goland:noinspection ALL
func readyServiceFixture(t *testing.T) (string, string, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	executable, err := os.Executable()
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{"command": executable, "args": []string{"-test.run=TestDiscoveryIntegrationReadyHelper"},
		"env": map[string]string{"ARDENTS_DISCOVERY_INTEGRATION_HELPER": "1", "ARDENTS_DISCOVERY_INTEGRATION_ADDRESS": fmt.Sprintf("127.0.0.1:%d", port)}})
	require.NoError(t, err)
	for _, address := range mustInterfaceAddresses(t) {
		if address.To4() != nil && address.IsPrivate() && !address.IsLoopback() {
			return string(raw), fmt.Sprintf("http://%s:%d/ready", address.String(), port), fmt.Sprintf("http://127.0.0.1:%d/ready", port)
		}
	}
	t.Fatal("Linux test container has no private IPv4 address")
	return "", "", ""
}

func mustInterfaceAddresses(t *testing.T) []net.IP {
	t.Helper()
	addresses, err := net.InterfaceAddrs()
	require.NoError(t, err)
	result := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		ip, _, parseErr := net.ParseCIDR(address.String())
		if parseErr == nil {
			result = append(result, ip)
		}
	}
	return result
}

func signedNodeRecord(t *testing.T, endpoints []string) (discovery.CatalogRecordSnapshot, ed25519.PrivateKey) {
	t.Helper()

	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoErrorf(t, err, "generate key: %v", err)

	publicKey := base64.StdEncoding.EncodeToString(public)
	principal, err := identityprincipal.FromPublicKey(publicKey)
	require.NoErrorf(t, err, "principal from public key: %v", err)

	record := discovery.CatalogRecordSnapshot{
		Version: discoveryrecords.Version,
		Node: &discovery.CatalogNodeFactsSnapshot{
			Principal: principal,
			PublicKey: publicKey,
			Endpoints: append([]string(nil), endpoints...),
		},
		IssuedAt:  time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	signDiscoveryRecord(t, &record, private)
	return record, private
}

func signDiscoveryRecord(t *testing.T, record *discovery.CatalogRecordSnapshot, private ed25519.PrivateKey) {
	t.Helper()

	payload, err := discovery.Canonical(discovery.RecordFromSnapshot(*record))
	require.NoErrorf(t, err, "canonical record: %v", err)

	record.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, payload))
}

func signedExpiredNodeRecord(t *testing.T, now time.Time, endpoints []string) discovery.Record {
	t.Helper()
	record, private := signedNodeRecord(t, endpoints)
	record.IssuedAt = now.Add(-2 * time.Hour)
	record.ExpiresAt = now.Add(-time.Minute)
	signDiscoveryRecord(t, &record, private)
	return discovery.RecordFromSnapshot(record)
}

//go:build linux && (endpoint_launch_native || (endpoint_pair_native && document_worker_installed))

package endpoint

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	sourcefixture "github.com/dianabuilds/ardents-network/tests/e2e/source"
)

// launchSources identifies the two authenticated Sources and the Endpoint's client credentials.
type launchSources struct {
	Certificate, Key string
	Addresses        [2]string
	Pins             [2][32]byte
}

// StartLaunchSources seeds and starts two independent Sources for the installed Endpoint.
func StartLaunchSources(t testing.TB, base, etcDir string, config state.Config, epoch []byte, inputs, materials [][]byte) launchSources {
	t.Helper()
	// Source credential pair below /etc/ardents: cert root-owned public, key
	// service-owned private (chowned after provision creates the account). Both
	// the F-25 pre-start check and the confined participant read them.
	_, clientPEM, clientKey, clientPin := sourcefixture.SourceCertificate(t, "client", false)
	certificatePath := filepath.Join(etcDir, "client.crt")
	keyPath := filepath.Join(etcDir, "client.key")
	WriteLaunchFile(t, certificatePath, clientPEM, 0644)
	WriteLaunchFile(t, keyPath, clientKey, 0600)
	clear(clientKey)

	// Two genuine live Source servers, each its own accepted State root.
	credentials := filepath.Join(base, "credentials")
	if err := os.Mkdir(credentials, 0755); err != nil {
		t.Fatal(err)
	}
	var addresses [2]string
	var sourcePins [2][32]byte
	for i := range addresses {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addresses[i] = listener.Addr().String()
		if err := listener.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for i, address := range addresses {
		name := []string{"source-a.example", "source-b.example"}[i]
		certificate, rootPEM, keyPEM, pin := sourcefixture.SourceCertificate(t, name, true)
		clear(keyPEM)
		sourcePins[i] = pin
		WriteLaunchFile(t, filepath.Join(credentials, []string{"a.pem", "b.pem"}[i]), rootPEM, 0644)
		cfg := config
		cfg.Root = filepath.Join(base, fmt.Sprintf("source-state-%d", i))
		cfg.LocalRoleStateRoot = filepath.Join(base, fmt.Sprintf("source-roles-%d", i))
		seed, err := state.Open(cfg)
		if err != nil {
			t.Fatal(err)
		}
		_, acceptErr := seed.AcceptEpoch(t.Context(), epoch, inputs, materials)
		closeErr := seed.Close()
		if acceptErr != nil || closeErr != nil {
			t.Fatal(acceptErr, closeErr)
		}
		cfg.Source.ServeAddress = address
		cfg.Source.ServeCertificate = certificate
		cfg.Source.ServeClientRootPEM = clientPEM
		cfg.Source.ServeClientKeyDigests = [][32]byte{clientPin}
		server, err := state.Open(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := server.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	return launchSources{Certificate: certificatePath, Key: keyPath, Addresses: addresses, Pins: sourcePins}

}

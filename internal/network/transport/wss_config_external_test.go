package transport_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	transport "ardents/internal/network/api"
	"ardents/tests/testkit"

	"github.com/stretchr/testify/require"
)

func TestWSSConfigAcceptsCAIssuedMatchingCertificate(t *testing.T) {
	certPath, keyPath := testkit.WriteWSSCert(t)
	err := transport.ValidateTransportConfig(validWSSConfig(t, certPath, keyPath), time.Now())
	require.NoError(t, err)
}

func TestWSSConfigRejectsSelfSignedCertificate(t *testing.T) {
	certPath, keyPath := testkit.WriteSelfSignedWSSCert(t)
	err := transport.ValidateTransportConfig(validWSSConfig(t, certPath, keyPath), time.Now())
	require.ErrorContains(t, err, "must not be self-signed")
}

func TestWSSConfigRejectsMismatchedPrivateKey(t *testing.T) {
	certPath, _ := testkit.WriteWSSCert(t)
	_, otherKeyPath := testkit.WriteWSSCert(t)
	err := transport.ValidateTransportConfig(validWSSConfig(t, certPath, otherKeyPath), time.Now())
	require.ErrorContains(t, err, "invalid or mismatched")
}

func TestWSSConfigRejectsExpiredCertificate(t *testing.T) {
	certPath, keyPath := testkit.WriteExpiredWSSCert(t)
	err := transport.ValidateTransportConfig(validWSSConfig(t, certPath, keyPath), time.Now())
	require.ErrorContains(t, err, "expired")
}

func TestWSSConfigRejectsHostnameMismatch(t *testing.T) {
	certPath, keyPath := testkit.WriteWSSCertForHost(t, "wss.example.test")
	err := transport.ValidateTransportConfig(validWSSConfig(t, certPath, keyPath), time.Now())
	require.ErrorContains(t, err, "does not cover advertised address")
}

func TestWSSConfigRejectsChainOutsideConfiguredTrust(t *testing.T) {
	certPath, keyPath := testkit.WriteWSSCert(t)
	cfg := validWSSConfig(t, certPath, keyPath)
	cfg.WSSCAPath = ""

	err := transport.ValidateTransportConfig(cfg, time.Now())
	require.ErrorContains(t, err, "chain is not trusted")
}

func TestTCPOnlyRejectsDormantWSSSettings(t *testing.T) {
	err := transport.ValidateTransportConfig(transport.Config{
		Profile: transport.ProfileTCPOnly,
		WSSPort: testkit.ReserveLoopbackTCPPort(t),
	}, time.Now())
	require.ErrorContains(t, err, "does not accept secure websocket settings")
}

func TestWSSConfigErrorsDoNotExposeMaterialPaths(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "operator-secret-certificate.pem")
	cfg := validWSSConfig(t, secretPath, secretPath+".key")

	err := transport.ValidateTransportConfig(cfg, time.Now())
	require.Error(t, err)
	require.NotContains(t, err.Error(), secretPath)
}

func TestInvalidWSSConfigFailsBeforeTransportPersistence(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "waku-store.db")
	keyPath := filepath.Join(dir, "waku-node-key.json")
	svc := transport.New(transport.Config{
		Profile:             transport.ProfileTCPWSS,
		StorePath:           storePath,
		PrivateKeyPath:      keyPath,
		WSSPort:             testkit.ReserveLoopbackTCPPort(t),
		WSSCertPath:         filepath.Join(dir, "missing-cert.pem"),
		WSSKeyPath:          filepath.Join(dir, "missing-key.pem"),
		WSSAdvertiseAddress: "127.0.0.1",
	})

	err := svc.Start(context.Background())
	require.Error(t, err)
	_, storeErr := os.Stat(storePath)
	require.ErrorIs(t, storeErr, os.ErrNotExist)
	_, keyErr := os.Stat(keyPath)
	require.ErrorIs(t, keyErr, os.ErrNotExist)
}

func validWSSConfig(t *testing.T, certPath, keyPath string) transport.Config {
	t.Helper()
	return transport.Config{
		Profile:             transport.ProfileTCPWSS,
		WSSPort:             testkit.ReserveLoopbackTCPPort(t),
		WSSCertPath:         certPath,
		WSSKeyPath:          keyPath,
		WSSCAPath:           testkit.WSSCAPath(certPath),
		WSSAdvertiseAddress: "127.0.0.1",
	}
}
